"""把页面按视口切片截图，方便逐块检查（页面太高，一张图看不清字）。"""
import os, re, subprocess, tempfile

EDGE = r"C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe"
HTML = r"D:\szuNet\desktop\index.html"
OUTDIR = r"D:\szuNet\desktop\design"
PAGEDIR = os.path.dirname(HTML)   # 临时副本必须和 index.html 同目录，相对路径（字体）才会生效
ud = os.path.join(tempfile.gettempdir(), "edge_slice_szunet")

VIEW_W, VIEW_H = 1280, 700
SLICES = [0, 560, 1120, 1680]

src = open(HTML, encoding="utf-8").read()

# 先量总高度
tmp = os.path.join(PAGEDIR, "_measure.html")
open(tmp, "w", encoding="utf-8").write(
    src.replace("</head>", "<script>window.addEventListener('load',function(){"
                           "document.title='H'+document.documentElement.scrollHeight+'H';});</script></head>"))
dom = subprocess.run([EDGE, "--headless=new", "--disable-gpu", "--user-data-dir=" + ud,
                      "--window-size=1280,300", "--no-first-run", "--no-default-browser-check",
                      "--virtual-time-budget=4000", "--dump-dom",
                      "file:///" + tmp.replace("\\", "/")],
                     capture_output=True, timeout=180).stdout.decode("utf-8", "ignore")
m = re.search(r"<title>H(\d+)H</title>", dom)
total = int(m.group(1)) if m else 0
os.remove(tmp)
print("total height:", total)

made = []
for y in SLICES:
    if y >= total:
        continue
    tag = os.path.join(PAGEDIR, "_slice%d.html" % y)
    inject = "<style>body{top:-%dpx !important;}</style>" % y if y else ""
    open(tag, "w", encoding="utf-8").write(src.replace("</head>", inject + "</head>"))
    out = os.path.join(OUTDIR, "slice-%d.png" % y)
    if os.path.exists(out):
        os.remove(out)
    subprocess.run([EDGE, "--headless=new", "--disable-gpu", "--hide-scrollbars",
                    "--force-device-scale-factor=2",
                    "--window-size=%d,%d" % (VIEW_W, VIEW_H),
                    "--user-data-dir=" + ud, "--no-first-run", "--no-default-browser-check",
                    "--virtual-time-budget=5000", "--screenshot=" + out,
                    "file:///" + tag.replace("\\", "/")],
                   capture_output=True, timeout=180)
    os.remove(tag)
    if os.path.exists(out):
        print("  ", out, os.path.getsize(out))
        made.append(out)
print("done", len(made))
