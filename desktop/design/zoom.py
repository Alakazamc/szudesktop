"""放大截取页面上某一块，用来肉眼检查细节。

用法: python zoom.py <top偏移px> <宽> <高> <倍数> <输出名> [额外CSS]
可选环境变量 VIEWW：渲染宽度，默认 1241（和 probe.py 一致，坐标才对得上）

做法：复制一份页面到同目录（相对路径的字体/图片才找得到），
往 </head> 前塞一段样式把 body 往上顶，再让 Edge 无头截一张小窗口。

⚠️ 窗口宽度必须固定成 1241：页面是自适应布局，
换了宽度三栏就会重排，用 probe.py 量出来的坐标全部作废。
"""
import os
import subprocess
import sys

PAGEDIR = r"D:\szuNet\desktop"
PAGE = os.path.join(PAGEDIR, "index.html")
OUTDIR = os.path.join(PAGEDIR, "design")
VIEWW = int(os.environ.get("VIEWW", "1241"))   # 必须和 probe.py 的窗口宽度一致


def find_edge():
    for p in (r"C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe",
              r"C:\Program Files\Microsoft\Edge\Application\msedge.exe"):
        if os.path.exists(p):
            return p
    raise SystemExit("找不到 Edge")


def shot(offset, w, h, scale, name, extra_css=""):
    src = open(PAGE, encoding="utf-8").read()
    # 1) 把视口固定成 VIEWW 宽，避免重排
    # 2) 想截哪块就截哪块：给 body 加个负 top，把目标区域顶到视口顶部
    css = ("html{width:%dpx !important;}\n"
           "body{width:%dpx !important; position:relative; top:-%dpx !important;}\n"
           "%s") % (VIEWW, VIEWW, offset, extra_css)
    src = src.replace("</head>", "<style>%s</style></head>" % css)
    tmp = os.path.join(PAGEDIR, "_zoom.html")
    open(tmp, "w", encoding="utf-8").write(src)

    out = os.path.join(OUTDIR, name)
    edge = find_edge()
    # 窗口宽度给足 VIEWW，让横向裁剪点落在 w 上；高度给 h
    cmd = [edge, "--headless=new", "--disable-gpu",
           "--force-device-scale-factor=%d" % scale,
           "--user-data-dir=" + os.path.join(os.environ["TEMP"], "edge-szu-zoom"),
           "--virtual-time-budget=3500",
           "--window-size=%d,%d" % (VIEWW, h),
           "--screenshot=" + out,
           "file:///" + tmp.replace("\\", "/")]
    r = subprocess.run(cmd, capture_output=True)
    os.remove(tmp)
    print("exit %d  %s  %d 字节" % (r.returncode, out,
                                   os.path.getsize(out) if os.path.exists(out) else 0))


if __name__ == "__main__":
    a = sys.argv[1:]
    shot(int(a[0]), int(a[1]), int(a[2]), int(a[3]), a[4],
         a[5] if len(a) > 5 else "")
