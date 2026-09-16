"""一步到位：同步页面 → 编译 Windows 版 → 塞图标/版本信息 → 校验内嵌页面是当前版本。

为什么要有这个脚本：页面要进二进制，中间隔了三步（主副本 → assets/ → internal/ui/assets/ → 编译），
手敲很容易漏掉一步，结果编出来的 exe 里还是旧页面，而且看不出来。已经踩过一次。

用法:
    python build-windows.py            # 同步 + 编译 + 打资源
    python build-windows.py --no-build # 只同步
    python build-windows.py --no-res   # 不打图标/版本信息
"""
import hashlib
import os
import re
import shutil
import subprocess
import sys

# 路径都从脚本自己的位置推出来，项目目录改名/挪盘都不用改这里
DESKTOP = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.dirname(DESKTOP)
MASTER = os.path.join(DESKTOP, "index.html")           # 页面主副本（唯一应该手改的）
ASSETS = os.path.join(DESKTOP, "assets")               # 浏览器直接打开用的副本
UI_ASSETS = os.path.join(DESKTOP, "internal", "ui", "assets")  # go:embed 能看见的那份
OUT = os.path.join(ROOT, "dist", "szudesktop-windows-amd64.exe")
ICON = os.path.join(ASSETS, "szudesktop.ico")
MAIN_GO = os.path.join(DESKTOP, "cmd", "szudesktop", "main.go")


def md5(path):
    return hashlib.md5(open(path, "rb").read()).hexdigest()


def size(path):
    return os.path.getsize(path)


def step(msg):
    print("\n>> " + msg)


def read_version():
    """从 main.go 里抠出版本号，省得两处各写一份、改一处忘一处。"""
    m = re.search(r'const\s+version\s*=\s*"([^"]+)"',
                  open(MAIN_GO, encoding="utf-8").read())
    if not m:
        print("!! 在 main.go 里找不到 const version")
        sys.exit(1)
    return m.group(1)


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

VER = read_version()

# 4. 编译
step("编译 Windows 版")
env = dict(os.environ, CGO_ENABLED="0", GOOS="windows", GOARCH="amd64")
r = subprocess.run(["go", "build", "-trimpath", "-ldflags", "-s -w",
                    "-o", OUT, "./desktop/cmd/szudesktop"],
                   cwd=ROOT, env=env)
if r.returncode != 0:
    print("!! 编译失败")
    sys.exit(r.returncode)
print("   %s  %.1f MB  版本 %s" % (OUT, os.path.getsize(OUT) / 1024 / 1024, VER))

# 5. 塞图标和版本信息。
#    必须跟在 go build 之后 —— 重编译会把 .rsrc 段冲掉。
if "--no-res" in sys.argv:
    print("\n>> 跳过图标/版本信息")
elif not os.path.exists(ICON):
    print("\n>> !! 找不到 %s，先生成：python design/gen_icon.py" % ICON)
    sys.exit(1)
else:
    step("写入图标与版本信息")
    r = subprocess.run([sys.executable, os.path.join(DESKTOP, "add_resource.py"),
                        OUT, "--ico", ICON, "--version", VER],
                       cwd=ROOT)
    if r.returncode != 0:
        print("!! 资源写入失败")
        sys.exit(r.returncode)

# 6. 校验：主副本和 assets/ 那份必须字节一致（能抓出「改了页面忘了同步」）
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
