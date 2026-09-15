"""一步到位：同步页面 → 编译 Windows 版 → 校验内嵌页面是当前版本。

为什么要有这个脚本：页面要进二进制，中间隔了三步（主副本 → assets/ → internal/ui/assets/ → 编译），
手敲很容易漏掉一步，结果编出来的 exe 里还是旧页面，而且看不出来。已经踩过一次。

用法:
    python build-windows.py            # 同步 + 编译
    python build-windows.py --no-build # 只同步
"""
import hashlib
import os
import shutil
import subprocess
import sys

DESKTOP = r"D:\szuNet\desktop"
ROOT = r"D:\szuNet"
MASTER = os.path.join(DESKTOP, "index.html")           # 页面主副本（唯一应该手改的）
ASSETS = os.path.join(DESKTOP, "assets")               # 浏览器直接打开用的副本
UI_ASSETS = os.path.join(DESKTOP, "internal", "ui", "assets")  # go:embed 能看见的那份
OUT = os.path.join(ROOT, "dist", "szudesktop-windows-amd64.exe")


def md5(path):
    return hashlib.md5(open(path, "rb").read()).hexdigest()


def size(path):
    return os.path.getsize(path)


def step(msg):
    print("\n>> " + msg)


# 1. 主副本 → assets/
step("同步页面主副本到 assets/")
if not os.path.exists(MASTER):
    print("!! 找不到 desktop/index.html")
    sys.exit(1)
shutil.copy2(MASTER, os.path.join(ASSETS, "index.html"))
print("   index.html  %d 字节  %s" % (size(MASTER), md5(MASTER)[:10]))

# 2. assets/ → internal/ui/assets/（go:embed 不能往上一级跳，必须复制一份）
step("同步 assets/ 到 internal/ui/assets/")
if os.path.isdir(UI_ASSETS):
    shutil.rmtree(UI_ASSETS)
shutil.copytree(ASSETS, UI_ASSETS)
files = sum(len(f) for _, _, f in os.walk(UI_ASSETS))
total = sum(os.path.getsize(os.path.join(r, f)) for r, _, fs in os.walk(UI_ASSETS) for f in fs)
print("   %d 个文件，%.1f KB" % (files, total / 1024))

# 3. 顺手校验 CSS 里引用的 woff2 都在（字体缺了页面会悄悄变丑）
missing = []
import re
for r, _, fs in os.walk(UI_ASSETS):
    for f in fs:
        if f.endswith(".css"):
            css = open(os.path.join(r, f), encoding="utf-8").read()
            for u in re.findall(r"url\(([^)]+\.woff2)\)", css):
                if not os.path.exists(os.path.join(r, u)):
                    missing.append(u)
if missing:
    print("   !! 缺 %d 个 woff2，字体加载不全" % len(missing))
    sys.exit(1)
print("   css 引用的 woff2 全部就位")

if "--no-build" in sys.argv:
    print("\n只同步，不编译。")
    sys.exit(0)

# 4. 编译
step("编译 Windows 版")
env = dict(os.environ, CGO_ENABLED="0", GOOS="windows", GOARCH="amd64")
r = subprocess.run(["go", "build", "-trimpath", "-ldflags", "-s -w",
                    "-o", OUT, "./desktop/cmd/szudesktop"],
                   cwd=ROOT, env=env)
if r.returncode != 0:
    print("!! 编译失败")
    sys.exit(r.returncode)
print("   %s  %.1f MB" % (OUT, os.path.getsize(OUT) / 1024 / 1024))

# 5. 校验：主副本和 assets/ 那份必须字节一致（能抓出「改了页面忘了同步」）
step("校验同步结果")
master_h = md5(MASTER)
assets_h = md5(os.path.join(ASSETS, "index.html"))
same = master_h == assets_h
print("   主副本   %d 字节  %s" % (size(MASTER), master_h[:10]))
print("   assets/ %d 字节  %s" % (size(os.path.join(ASSETS, "index.html")), assets_h[:10]))
print("   两边一致: %s" % ("是" if same else "否"))
if not same:
    print("!! 同步没生效，编出来的 exe 里会是旧页面")
    sys.exit(1)

print("\n完成。下一步跑冒烟测试:")
print("   python desktop/smoke_windows.py")
