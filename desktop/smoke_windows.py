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
import subprocess
import sys
import time
import urllib.request

EXE = sys.argv[1] if len(sys.argv) > 1 else r"D:\szuNet\dist\szudesktop-windows-amd64.exe"
PORT = sys.argv[2] if len(sys.argv) > 2 else "18899"
BASE = "http://127.0.0.1:" + PORT

# 这些字符串只在新版页面里出现，用来确认包进去的是当前代码
MARKERS = ["sys-mode", "btn-logout", "stat-zone", "refreshStatus"]

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

proc = subprocess.Popen([EXE, "--no-open", "--no-auto-login", "--addr", "127.0.0.1:" + PORT],
                        stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
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
    for path in ["/api/status", "/api/diag", "/api/credential"]:
        try:
            st, ct, body = http(path)
            parsed = json.loads(body.decode("utf-8"))
            line(path, st == 200 and "json" in ct, "%d, %d 字节, %d 个字段" % (st, len(body), len(parsed)))
        except Exception as e:
            line(path, False, "%s: %s" % (type(e).__name__, e))

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

    print("\n[4] 保持在线开关")
    try:
        _, _, body = http("/api/keepalive")
        d = json.loads(body.decode("utf-8"))
        line("默认是开着的", d.get("on") is True, "interval=%s 秒" % d.get("interval"))

        req = urllib.request.Request(BASE + "/api/keepalive",
                                     data=json.dumps({"on": False}).encode(),
                                     headers={"Content-Type": "application/json"}, method="POST")
        with urllib.request.urlopen(req, timeout=15) as r:
            d2 = json.loads(r.read().decode("utf-8"))
        line("可以关掉", d2.get("on") is False, "started=%s" % d2.get("started"))

        # 复现过的问题：关掉再打开时，状态改在 goroutine 里，
        # 响应可能早于状态变更返回，读出来还是 on=false。这里 POST 完再单独 GET 复核。
        req = urllib.request.Request(BASE + "/api/keepalive",
                                     data=json.dumps({"on": True}).encode(),
                                     headers={"Content-Type": "application/json"}, method="POST")
        with urllib.request.urlopen(req, timeout=15) as r:
            d3 = json.loads(r.read().decode("utf-8"))
        line("可以再打开", d3.get("on") is True, "started=%s" % d3.get("started"))

        _, _, body = http("/api/keepalive")
        d4 = json.loads(body.decode("utf-8"))
        line("再打开后 GET 复核", d4.get("on") is True)
    except Exception as e:
        line("保持在线", False, str(e))

    print("\n[4b] 状态接口与开关一致")
    try:
        _, _, body = http("/api/status")
        d = json.loads(body.decode("utf-8"))
        line("status.keep_alive 是布尔", isinstance(d.get("keep_alive"), bool), str(d.get("keep_alive")))
        line("status.relogins 是数字", isinstance(d.get("relogins"), int), str(d.get("relogins")))
    except Exception as e:
        line("状态接口", False, str(e))

    print("\n[5] 静态资源")
    for path, size_min in [("/", 40000), ("/fonts/fusion-pixel.css", 50000), ("/fonts/svbold.ttf", 10000)]:
        try:
            st, ct, body = http(path)
            line(path, st == 200 and len(body) >= size_min,
                 "%d, %.1f KB, %s" % (st, len(body) / 1024, ct.split(";")[0]))
        except Exception as e:
            line(path, False, str(e))

    print("\n[6] 页面是当前版本")
    try:
        _, _, body = http("/")
        page = body.decode("utf-8", "ignore")
        for m in MARKERS:
            line("含标记 " + m, m in page)
        local = open(r"D:\szuNet\desktop\assets\index.html", encoding="utf-8").read()
        line("与本地 index.html 一致", len(page) == len(local),
             "内嵌 %d 字节 / 本地 %d 字节" % (len(page), len(local)))
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

print("\n" + "=" * 62)
print("结果:", "全部通过" if ok_all else "有失败项，见上面 [!!]")
print("=" * 62)
sys.exit(0 if ok_all else 1)
