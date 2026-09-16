"""Windows 版冒烟测试：起服务 → 打接口 → 关掉。

用法: python smoke_windows.py [exe路径] [端口]

做的事：
  1. 用 --no-auto-login --no-open 在指定端口起服务（不弹浏览器、不乱登录）
  2. 逐个打 /api/* 接口和静态资源，检查状态码、JSON 能否解析
  3. 校验返回的页面确实是当前版本（靠标记字符串比对本地文件）
  4. 收尾杀进程，不管成败都不留后台
"""
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
import time
import urllib.request

# GitHub Windows Runner 可能使用 CP1252；中文测试日志统一输出为 UTF-8。
for _stream in (sys.stdout, sys.stderr):
    if hasattr(_stream, "reconfigure"):
        _stream.reconfigure(encoding="utf-8", errors="replace")

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))  # 项目根，从脚本位置推
EXE = sys.argv[1] if len(sys.argv) > 1 else os.path.join(ROOT, "dist", "szudesktop-windows-amd64.exe")
PORT = sys.argv[2] if len(sys.argv) > 2 else "18899"
BASE = "http://127.0.0.1:" + PORT

# 这些字符串只在新版页面里出现，用来确认包进去的是当前代码。
#
# ⚠️ 加标记时注意：光有 HTML 元素不代表功能能用。
# 登录页翻车过一次——HTML 元素全在、按钮也画出来了，但 refreshLogin()
# 和三个按钮的事件处理全都没写，点了完全没反应。当时这个列表里有
# refreshStatus 却没有 refreshLogin，一字之差就漏过去了。
# 所以这里除了元素 id，还要盯住「驱动它的那个函数存不存在」。
MARKERS = [
    "sys-mode", "btn-logout", "stat-zone", "refreshStatus",
    # 登录页：元素 + 驱动它的函数，两样都得在
    "lg-login", "refreshLogin", "doLoginPage", "doLogoutPage", "doForgetPage",
    # 校外 VPN 页面：入口、操作函数和未来校内服务状态
    "view-vpn", "vpnConnect", "vpnAuth", "vpnDisconnect", "vpnProxyToggle", "campus-badge",
]

ok_all = True


def line(label, ok, detail=""):
    global ok_all
    if not ok:
        ok_all = False
    print("  %-4s %-30s %s" % ("[OK]" if ok else "[!!]", label, detail))


def http(path, timeout=30):
    req = urllib.request.Request(BASE + path)
    with urllib.request.urlopen(req, timeout=timeout) as r:
        return r.status, r.headers.get("Content-Type", ""), r.read()


print("=" * 62)
print("Windows 冒烟测试:", EXE)
print("=" * 62)

if not os.path.exists(EXE):
    print("!! 找不到可执行文件")
    sys.exit(2)

SMOKE_CONFIG = tempfile.mkdtemp(prefix="szudesktop-smoke-", dir=os.path.join(ROOT, "dist"))
proc_env = dict(os.environ, SZUNET_CONFIG_DIR=SMOKE_CONFIG)
proc = subprocess.Popen([EXE, "--no-open", "--no-auto-login", "--addr", "127.0.0.1:" + PORT],
                        stdout=subprocess.PIPE, stderr=subprocess.STDOUT, env=proc_env)
try:
    # 等服务起来，最多 10 秒
    up = False
    for _ in range(20):
        time.sleep(0.5)
        try:
            http("/api/status", timeout=3)
            up = True
            break
        except Exception:
            continue

    print("\n[1] 服务启动")
    line("端口 %s 可访问" % PORT, up)
    if not up:
        raise SystemExit(1)

    print("\n[2] 接口")
    for path in ["/api/status", "/api/diag", "/api/credential", "/api/vpn/status", "/api/campus/status"]:
        try:
            st, ct, body = http(path)
            parsed = json.loads(body.decode("utf-8"))
            line(path, st == 200 and "json" in ct, "%d, %d 字节, %d 个字段" % (st, len(body), len(parsed)))
        except Exception as e:
            line(path, False, "%s: %s" % (type(e).__name__, e))

    print("\n[2b] VPN 状态接口")
    try:
        _, _, body = http("/api/vpn/status")
        d = json.loads(body.decode("utf-8"))
        line("默认状态未登录", d.get("state") == "idle", str(d.get("state_label", "")))
        line("SOCKS 只绑本机", d.get("socks_addr") == "127.0.0.1:7891", str(d.get("socks_addr", "")))
        line("系统代理状态可读", isinstance(d.get("proxy"), dict), str(d.get("proxy", {}).get("note", ""))[:60])
        sensitive = {"password", "passwd", "pwd", "token", "twfid"}
        line("VPN 响应不含秘密字段", not sensitive.intersection(k.lower() for k in d.keys()),
             "字段: " + ", ".join(sorted(d.keys())))
    except Exception as e:
        line("VPN 状态接口", False, str(e))

    print("\n[3] 状态内容")
    try:
        _, _, body = http("/api/status")
        d = json.loads(body.decode("utf-8"))
        line("zone 字段", d.get("zone") in ("online", "teaching", "dorm", "outside", "unknown"),
             d.get("zone_label", ""))
        line("internet_ok 是布尔", isinstance(d.get("internet_ok"), bool), str(d.get("internet_ok")))
        line("store_desc 非空", bool(d.get("store_desc")), str(d.get("store_desc"))[:44])
        # 只认字段名，别去 grep 整段文本——提示语里也会出现 password 这个词
        has_pwd_field = any(k in ("password", "passwd", "pwd") for k in d.keys())
        line("响应里没有密码字段", not has_pwd_field, "字段: " + ", ".join(sorted(d.keys())))
    except Exception as e:
        line("状态内容", False, str(e))

    print("\n[3b] 凭据接口不回传密码")
    try:
        _, _, body = http("/api/credential")
        d = json.loads(body.decode("utf-8"))
        line("GET 不含密码字段", not any(k in ("password", "passwd", "pwd") for k in d.keys()),
             "字段: " + ", ".join(sorted(d.keys())))
        # POST 一个假凭据再删掉，验证写入/删除路径（用不会真的登录的假卡号）
        req = urllib.request.Request(BASE + "/api/credential",
                                     data=json.dumps({"username": "000000", "password": "smoketest-not-real"}).encode(),
                                     headers={"Content-Type": "application/json"}, method="POST")
        with urllib.request.urlopen(req, timeout=15) as r:
            d2 = json.loads(r.read().decode("utf-8"))
        line("写入凭据", d2.get("ok") is True, str(d2.get("store_desc", ""))[:40])
        req = urllib.request.Request(BASE + "/api/credential", method="DELETE")
        with urllib.request.urlopen(req, timeout=15) as r:
            d3 = json.loads(r.read().decode("utf-8"))
        line("删除凭据", d3.get("ok") is True, "测试凭据已清理")
    except Exception as e:
        line("凭据接口", False, str(e))

    print("\n[4] 登录接口接受临时账号（不落盘）")
    try:
        req = urllib.request.Request(BASE + "/api/login",
                                     data=json.dumps({"username": "", "password": ""}).encode(),
                                     headers={"Content-Type": "application/json"}, method="POST")
        with urllib.request.urlopen(req, timeout=30) as r:
            d = json.loads(r.read().decode("utf-8"))
        # 保存区为空时走这个请求应得到明确报错文案，而不是 500 或崩溃
        line("空账号返回说明而非崩溃", isinstance(d.get("message"), str) and d.get("message") != "",
             str(d.get("message", ""))[:80])
    except Exception as e:
        line("登录接口", False, str(e))

    print("\n[5] 静态资源")
    for path, size_min in [("/", 40000), ("/fonts/fusion-pixel.css", 50000), ("/fonts/svbold.ttf", 10000)]:
        try:
            st, ct, body = http(path)
            line(path, st == 200 and len(body) >= size_min,
                 "%d, %.1f KB, %s" % (st, len(body) / 1024, ct.split(";")[0]))
        except Exception as e:
            line(path, False, str(e))

    print("\n[5b] 美术素材能不能取到")
    # 这些是从 stardewOS 拿来的图。少一张页面就会有块空白，
    # 但浏览器不会报错，所以必须在这里挑几张关键的打一遍。
    for path in ["/art/m1.png", "/art/calendar.png", "/art/coursor.png",
                 "/art/junimo-green.png", "/art/loading.png", "/art/bird.gif",
                 "/art/dwarf.png", "/art/maximize.png"]:
        try:
            st, ct, body = http(path)
            line(path, st == 200 and len(body) > 100,
                 "%d, %.1f KB, %s" % (st, len(body) / 1024, ct.split(";")[0]))
        except Exception as e:
            line(path, False, str(e))

    print("\n[6] 页面引用的资源路径一条都不能 404")
    # 这条是补的坑：页面用 file:// 直接打开时，相对路径是基于 desktop/ 目录的，
    # 所以写的是 assets/art/xxx.png；但服务端嵌的根是 assets/ 这一层，
    # 收到 /assets/art/xxx.png 会去 assets/assets/art/ 找，直接 404。
    # 表现为：本地浏览器打开一切正常，跑起来满屏破图，而且浏览器不报错。
    # 所以这里把页面里所有 src/href 抽出来，逐个真打一遍。
    try:
        _, _, body = http("/")
        page = body.decode("utf-8", "ignore")
        # 先把 <style>...</style> 整段抠掉再抽路径。
        # 为什么：CSS 注释里为了说明用法会写示例，比如
        #     /* 用法：<img class="in-deco" src="..."> */
        # 这种"路径"是给人看的，不是真资源，正则一抓就误报 404。
        # 前一次冒烟就是这么挂的：报 /... -> 404，看着像图坏了，其实是注释。
        scannable = re.sub(r"<style\b[^>]*>.*?</style>", "", page,
                           flags=re.S | re.I)
        refs = set()
        for m in re.finditer(r'(?:src|href)\s*=\s*"([^"]+)"', scannable):
            u = m.group(1)
            if u.startswith(("http://", "https://", "data:", "#", "mailto:")):
                continue
            # JS 模板串（形如 ${...}）是运行时才拼出真路径的，这里没法验，跳过
            if "${" in u or "+" in u:
                continue
            # 省略号占位（...、…、以及夹在中间的 ...）一律不是真路径
            if "..." in u or "…" in u:
                continue
            if u.startswith("/"):
                refs.add(u)
            else:
                refs.add("/" + u)
        refs = sorted(refs)
        bad = []
        for u in refs:
            try:
                st, _, _ = http(u)
                if st != 200:
                    bad.append("%s -> %d" % (u, st))
            except Exception as e:
                bad.append("%s -> %s" % (u, e))
        line("页面共引用 %d 个资源" % len(refs), len(refs) > 0, "共 " + str(len(refs)) + " 条")
        # 出问题时把完整清单打出来。上次因为只打前 6 条、又截了长度，
        # 只看到一个 "/..." ，白猜了半天空。
        line("全部能取到", not bad, ("; ".join(bad[:6]) if bad else "没有 404"))
        for b in bad:
            print("       × " + b)
    except Exception as e:
        line("资源路径检查", False, str(e))

    print("\n[7] 页面是当前版本")
    try:
        _, _, body = http("/")
        page = body.decode("utf-8", "ignore")
        for m in MARKERS:
            line("含标记 " + m, m in page)
        # 注意这里比的是"构建时同步过去的那份"，不是 desktop/index.html。
        # 因为同步是 build-windows.py 干的：master → assets/index.html → 嵌进二进制。
        # 这一步挂掉通常意味着"改完页面没重新构建"，而不是页面本身有问题。
        #
        # ⚠️ 必须按字节比，不能 open(..., encoding=) 读成字符串再比长度。
        # 为什么：这台机器 git core.autocrlf=true，工作区文件是 CRLF 的；
        # 而 Python 文本模式读取会做换行归一化，把 \r\n 变成 \n，
        # 于是"读出来的字符数"永远比"发出去的字节数"少——正好少一个换行数。
        # 之前就是这个坑：1838 行 → 差 1838，看着像"内嵌的是旧版本"，
        # 白折腾一轮重新构建。（文件其实一直是好的，字节数完全一致。）
        local = open(os.path.join(ROOT, "desktop", "assets", "index.html"), "rb").read()
        line("与同步副本一致", body == local,
             "内嵌 %d 字节 / 副本 %d 字节" % (len(body), len(local)))
        # 顺手比一下 master，省得同步完忘了重新构建、或者反过来
        master = open(os.path.join(ROOT, "desktop", "index.html"), "rb").read()
        line("同步副本与 master 一致", master == local,
             "master %d 字节" % len(master))
    except Exception as e:
        line("页面版本", False, str(e))

finally:
    proc.terminate()
    try:
        proc.wait(timeout=5)
    except subprocess.TimeoutExpired:
        proc.kill()
    # 双保险：按镜像名再杀一次
    subprocess.run(["taskkill", "/F", "/IM", os.path.basename(EXE)], capture_output=True)
    # 测试凭据只存在这个临时目录；无论成功失败都清理，不碰 ~/.szunet 的真实凭据。
    shutil.rmtree(SMOKE_CONFIG, ignore_errors=True)

print("\n" + "=" * 62)
print("结果:", "全部通过" if ok_all else "有失败项，见上面 [!!]")
print("=" * 62)
sys.exit(0 if ok_all else 1)
