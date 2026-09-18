"""把 desktop/assets 同步到 desktop/internal/ui/assets。

为什么要有这一步：Go 的 go:embed 只能嵌入「本包目录及其子目录」里的文件，
待嵌入的资源不允许通过 .. 往上跳。

页面唯一源文件是 desktop/index.html；字体和图片源文件放在 desktop/assets/。
本脚本先生成 desktop/assets/index.html，再把整套资源复制到
 desktop/internal/ui/assets/，让 embed 能看见。

两个 index.html 副本都是构建产物，已经忽略，别手改。直接运行本脚本或
 desktop/build-windows.py 都会从唯一源文件重新生成，不会把旧页面编进去。
"""
import os, shutil

ROOT = os.path.dirname(os.path.abspath(__file__))  # desktop/ 自身，别写死盘符
MASTER = os.path.join(ROOT, "index.html")
SRC = os.path.join(ROOT, "assets")
DST = os.path.join(ROOT, "internal", "ui", "assets")

# index.html 的唯一源文件在 desktop/；assets/ 下那份是生成物。
shutil.copy2(MASTER, os.path.join(SRC, "index.html"))

assert os.path.realpath(DST) == os.path.join(os.path.realpath(ROOT), "internal", "ui", "assets")
if os.path.isdir(DST):
    shutil.rmtree(DST)
os.makedirs(DST)
for name in ("index.html", "szudesktop.ico"):
    shutil.copy2(os.path.join(SRC,name), os.path.join(DST,name))
shutil.copytree(os.path.join(SRC,"garden"), os.path.join(DST,"garden"))

files = sum(len(f) for _, _, f in os.walk(DST))
size = sum(os.path.getsize(os.path.join(r, f))
           for r, _, fs in os.walk(DST) for f in fs)
print("synced %d files (%.1f KB) -> %s" % (files, size / 1024, DST))

# 顺手校验：CSS 里引用的 woff2 都在
missing = []
for r, _, fs in os.walk(DST):
    for f in fs:
        if not f.endswith(".css"):
            continue
        css = open(os.path.join(r, f), encoding="utf-8").read()
        import re
        for u in re.findall(r"url\(([^)]+\.woff2)\)", css):
            if not os.path.exists(os.path.join(r, u)):
                missing.append(u)
if missing:
    print("!! 缺 %d 个 woff2，页面字体加载不全" % len(missing))
    for m in missing[:5]:
        print("   ", m)
else:
    print("css 里引用的 woff2 全部就位")
