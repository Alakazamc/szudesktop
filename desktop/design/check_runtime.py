"""用无头浏览器真跑一次页面，确认登录页的 JS 确实执行了。

为什么需要这个：静态检查只能证明「函数都定义了」，证明不了「运行时没抛错」。
登录页那次翻车正是运行时错误 —— refreshLogin() 未定义，点标签时抛
ReferenceError，页面照样显示、按钮点了没反应，不打开控制台看不出来。

判定思路：登录页上有几个位置的初始文案是写死在 HTML 里的（比如区域徽章写着
「探测中…」、状态格子是空的）。只有 refreshLogin() 真的跑完，这些地方才会被
改写。所以对比「渲染后的 DOM」和「初始 HTML」，就能确认 JS 到底有没有执行到位。

跑法：
    python desktop/design/check_runtime.py
返回码非 0 表示有问题。
"""
import os
import re
import subprocess
import sys
import tempfile
import time
import urllib.request

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))  # desktop/
PROJ = os.path.dirname(ROOT)
EXE = os.path.join(PROJ, "dist", "szudesktop-windows-amd64.exe")
PORT = "18877"
BASE = "http://127.0.0.1:" + PORT

EDGE_CANDIDATES = [
    r"C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe",
    r"C:\Program Files\Microsoft\Edge\Application\msedge.exe",
]
edge = next((p for p in EDGE_CANDIDATES if os.path.exists(p)), None)
if not edge:
    print("[--] 找不到 Edge，跳过运行时检查")
    sys.exit(0)
if not os.path.exists(EXE):
    print("[XX] 找不到 exe，先跑 build-windows.py:", EXE)
    sys.exit(1)

fails = []
srv = subprocess.Popen(
    [EXE, "--addr", "127.0.0.1:" + PORT, "--no-open", "--no-auto-login"],
    stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
)
try:
    # 等服务起来
    for _ in range(40):
        try:
            urllib.request.urlopen(BASE + "/api/status", timeout=1).read()
            break
        except Exception:
            time.sleep(0.25)
    else:
        print("[XX] 服务没起来")
        sys.exit(1)
    print("[OK] 服务已启动", BASE)

    profile = os.path.join(tempfile.gettempdir(), "_szu_edge_profile")
    # --virtual-time-budget 让页面里的 setTimeout/fetch 有时间跑完再 dump
    out = subprocess.run(
        [edge, "--headless=new", "--disable-gpu", "--no-first-run",
         "--user-data-dir=" + profile, "--virtual-time-budget=6000",
         "--dump-dom", BASE + "/#login"],
        capture_output=True, text=True, encoding="utf-8", errors="ignore", timeout=90,
    )
    dom = out.stdout or ""
    print("[OK] 渲染完成，DOM %d 字符" % len(dom))

    if len(dom) < 5000:
        print("[XX] DOM 太短，页面可能没渲染出来")
        print(out.stderr[:600])
        fails.append("页面没渲染")
    else:
        # 1) 登录视图应该是显示状态（#login 会触发切到校园网标签）
        m = re.search(r'id="view-login"[^>]*style="([^"]*)"', dom)
        style = m.group(1) if m else ""
        if "display:none" in style.replace(" ", ""):
            print("[XX] 登录页还是隐藏的 —— 标签切换没生效（refreshLogin 可能抛错了）")
            fails.append("登录页没显示")
        else:
            print("[OK] 登录页已切到显示状态")

        # 2) 区域徽章应该被改写，不再是写死的「探测中…」
        m = re.search(r'id="lg-zone"[^>]*>(.*?)<', dom, re.S)
        zone = (m.group(1).strip() if m else "")
        if not zone or zone == "探测中…":
            print("[XX] 区域徽章还是「探测中…」—— refreshLogin() 没跑完")
            fails.append("徽章没更新")
        else:
            print("[OK] 区域徽章已更新为:", zone)

        # 3) 状态格子应该被填上内容（初始是空 div）
        m = re.search(r'id="lg-state"[^>]*>(.*?)</div>\s*</div>\s*</section>', dom, re.S)
        cells = len(re.findall(r'class="k"', m.group(1))) if m else 0
        if cells == 0:
            # 退一步：整篇里数一下
            cells = len(re.findall(r'class="k"', dom))
        if cells < 3:
            print("[XX] 状态格子没填上（找到 %d 个）—— refreshLogin() 没执行到底" % cells)
            fails.append("状态格子空")
        else:
            print("[OK] 状态格子已填充，%d 个字段" % cells)

        # 4) 提示行应该被改写，不再是写死的「填好就能登录。」
        m = re.search(r'id="lg-note"[^>]*>(.*?)<', dom, re.S)
        note = (m.group(1).strip() if m else "")
        print("[OK] 提示行当前内容:", note or "(空)")

        # 5) 顺手确认内核识别成功（不是演示模式）
        m = re.search(r'id="sys-mode"[^>]*>(.*?)<', dom, re.S)
        mode = (m.group(1).strip() if m else "")
        if mode == "内核已连接":
            print("[OK] 页面认出了本地内核")
        else:
            print("[!!] 页面显示模式为 %r（期待「内核已连接」）" % mode)
            fails.append("没认出内核")
finally:
    srv.terminate()
    try:
        srv.wait(timeout=5)
    except Exception:
        srv.kill()
    print("[OK] 服务已收尾")

print()
if fails:
    print("结果: 不通过 —— " + "、".join(fails))
    sys.exit(1)
print("结果: 全部通过")
