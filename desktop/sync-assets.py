"""把 desktop/assets 同步到 desktop/internal/ui/assets。

为什么要有这一步：Go 的 go:embed 只能嵌入「本包目录及其子目录」里的文件，
待嵌入的资源不允许通过 .. 往上跳。

所以页面（index.html）和字体放在 desktop/assets/ 供浏览器直接打开，
打包时再复制一份到 desktop/internal/ui/assets/ 让 embed 能看见。

改完页面后跑一次这个脚本即可。两个目录内容完全一致，别手改 ui 下面那份。

⚠️ 更推荐用 desktop/build-windows.py：它会先把 index.html 主副本复制到 assets/，
再调本脚本，最后编译 + 字节校验。只跑这个脚本的话，忘了复制主副本就会把旧页面编进去。
"""
import os, shutil

ROOT = os.path.dirname(os.path.abspath(__file__))  # desktop/ 自身，别写死盘符
SRC = os.path.join(ROOT, "assets")
DST = os.path.join(ROOT, "internal", "ui", "assets")

if os.path.isdir(DST):
    shutil.rmtree(DST)
shutil.copytree(SRC, DST)

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
