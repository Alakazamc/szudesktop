"""校园网登录端到端验收：真 exe + 真 Edge 点击 + 本地假深澜门户。

全程使用临时凭据目录，不访问真实校园网、不读写 ~/.szunet。
第一遍登录由假门户返回密码错误，验证错误密码不会保存；第二遍返回成功，
验证页面最终提示保留、密码框清空、成功后才保存凭据。
"""
import http.server
import json
import os
import shutil
import socketserver
import subprocess
import sys
import tempfile
import threading
import time
import urllib.request

DESKTOP = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
ROOT = os.path.dirname(DESKTOP)
EXE = os.path.join(ROOT, "dist", "szudesktop-windows-amd64.exe")
NODE = r"C:\Users\Alakazam\.workbuddy\binaries\node\versions\22.22.2-3\node.exe"
EDGE_CANDIDATES = [
    r"C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe",
    r"C:\Program Files\Microsoft\Edge\Application\msedge.exe",
]
EDGE = next((p for p in EDGE_CANDIDATES if os.path.exists(p)), None)
APP_PORT = 18876
CDP_PORT = 18875

if not os.path.exists(EXE):
    print("[XX] 找不到 exe，先运行 desktop/build-windows.py")
    sys.exit(1)
if not EDGE or not os.path.exists(NODE):
    print("[XX] 缺少 Edge 或托管 Node，无法做点击验收")
    sys.exit(1)

state = {"login_calls": 0}


class PortalHandler(http.server.BaseHTTPRequestHandler):
    def log_message(self, *_):
        pass

    def do_GET(self):
        path = self.path.split("?", 1)[0]
        if path == "/cgi-bin/get_challenge":
            body = b'_({"challenge":"0123456789abcdef","client_ip":"10.20.30.40","error":"ok"})'
        elif path == "/srun_portal_pc":
            body = b'window.portal = { acid: "12" };'
        elif path == "/cgi-bin/srun_portal":
            state["login_calls"] += 1
            if state["login_calls"] == 1:
                body = b'_({"error":"login_error","error_msg":"bad password"})'
            else:
                body = b'_({"error":"ok","suc_msg":"login_ok","online_ip":"10.20.30.40"})'
        elif path == "/cgi-bin/rad_user_info":
            body = b'_({"error":"ok","user_name":"123456","online_ip":"10.20.30.40"})'
        else:
            self.send_error(404)
            return
        self.send_response(200)
        self.send_header("Content-Type", "application/javascript; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)


class ReuseServer(socketserver.TCPServer):
    allow_reuse_address = True


portal = ReuseServer(("127.0.0.1", 0), PortalHandler)
portal_port = portal.server_address[1]
threading.Thread(target=portal.serve_forever, daemon=True).start()

work = tempfile.mkdtemp(prefix="szudesktop-login-e2e-")
profile = os.path.join(work, "edge-profile")
config = os.path.join(work, "config")
env = dict(os.environ, SZUNET_CONFIG_DIR=config)
app = edge = None


def wait_url(url, attempts=80):
    for _ in range(attempts):
        try:
            return urllib.request.urlopen(url, timeout=1).read()
        except Exception:
            time.sleep(0.25)
    raise RuntimeError("等待服务超时：" + url)


try:
    app = subprocess.Popen([
        EXE, "--no-open", "--no-auto-login", "--addr", f"127.0.0.1:{APP_PORT}",
        "--zone", "teaching", "--host-teaching", f"http://127.0.0.1:{portal_port}",
    ], env=env, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    wait_url(f"http://127.0.0.1:{APP_PORT}/api/status")
    print("[OK] 测试客户端已启动（独立临时保险箱）")

    edge = subprocess.Popen([
        EDGE, "--headless=new", "--disable-gpu", "--no-first-run",
        f"--remote-debugging-port={CDP_PORT}", "--user-data-dir=" + profile,
        f"http://127.0.0.1:{APP_PORT}/#login",
    ], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    raw = wait_url(f"http://127.0.0.1:{CDP_PORT}/json/list")
    pages = json.loads(raw.decode("utf-8"))
    page = next(p for p in pages if p.get("type") == "page")

    js_path = os.path.join(work, "drive.mjs")
    script = r'''
const ws = new WebSocket(process.argv[2]);
let seq = 0;
const pending = new Map();
ws.onmessage = e => {
  const m = JSON.parse(e.data);
  if (m.id && pending.has(m.id)) { pending.get(m.id)(m); pending.delete(m.id); }
};
const send = (method, params={}) => new Promise(resolve => {
  const id = ++seq; pending.set(id, resolve); ws.send(JSON.stringify({id, method, params}));
});
const sleep = ms => new Promise(r => setTimeout(r, ms));
const evalJS = async expression => {
  const m = await send("Runtime.evaluate", {expression, awaitPromise:true, returnByValue:true});
  if (m.result?.exceptionDetails) throw new Error(m.result.exceptionDetails.text);
  return m.result?.result?.value;
};
ws.onopen = async () => {
  try {
    await send("Runtime.enable");
    for (let i=0; i<80; i++) {
      if (await evalJS('document.readyState === "complete" && !!document.getElementById("lg-login")')) break;
      await sleep(100);
    }
    await evalJS(`(() => {
      document.querySelector('#tabs .tab[data-view="network"]').click();
      document.getElementById('lg-user').value='123456';
      document.getElementById('lg-pass').value='wrong-first';
      document.getElementById('lg-zone-pick').value='teaching';
      document.getElementById('lg-save').checked=true;
      document.getElementById('lg-login').click();
    })()`);
    for (let i=0; i<120 && await evalJS('lgBusy'); i++) await sleep(100);
    const failed = await evalJS(`(async () => ({
      note: document.getElementById('lg-note').textContent,
      pass: document.getElementById('lg-pass').value,
      saved: (await fetch('/api/credential')).json ? await (await fetch('/api/credential')).json() : null
    }))()`);
    if (!failed.note.includes('认证失败') || failed.saved.saved !== false || failed.pass !== 'wrong-first')
      throw new Error('失败路径不符合预期: ' + JSON.stringify(failed));
    console.log('[OK] 密码错误时显示明确原因，错误密码未保存');

    await evalJS(`(() => {
      document.getElementById('lg-pass').value='correct-second';
      document.getElementById('lg-login').click();
    })()`);
    for (let i=0; i<120 && await evalJS('lgBusy'); i++) await sleep(100);
    const success = await evalJS(`(async () => {
      const r = await fetch('/api/credential');
      return {note:document.getElementById('lg-note').textContent,
              kind:document.getElementById('lg-note').className,
              pass:document.getElementById('lg-pass').value,
              saved:await r.json()};
    })()`);
    if (!success.note.includes('认证成功') || !success.note.includes('安全保存') ||
        !success.kind.includes('ok') || success.pass !== '' || success.saved.saved !== true ||
        success.saved.username !== '123456')
      throw new Error('成功路径不符合预期: ' + JSON.stringify(success));
    console.log('[OK] 成功提示保持可见，密码框已清空，凭据在成功后才保存');
    console.log(JSON.stringify({failed, success}));
    ws.close();
    process.exit(0);
  } catch (e) {
    console.error('[XX] ' + e.stack);
    ws.close();
    process.exit(1);
  }
};
'''
    with open(js_path, "w", encoding="utf-8") as f:
        f.write(script)
    result = subprocess.run([NODE, js_path, page["webSocketDebuggerUrl"]],
                            capture_output=True, text=True, encoding="utf-8", errors="replace", timeout=60)
    print(result.stdout.strip())
    if result.returncode != 0:
        print(result.stderr.strip())
        raise RuntimeError("浏览器点击验收失败")
    if state["login_calls"] != 2:
        raise RuntimeError(f"假门户收到 {state['login_calls']} 次登录，期望 2 次")
    print("[OK] 真页面按钮已贯通：页面 → 本地接口 → 深澜协议")
finally:
    portal.shutdown()
    portal.server_close()
    for proc in (edge, app):
        if proc and proc.poll() is None:
            subprocess.run(["taskkill", "/F", "/T", "/PID", str(proc.pid)], capture_output=True)
    shutil.rmtree(work, ignore_errors=True)
    print("[OK] 测试进程和临时凭据已清理")
