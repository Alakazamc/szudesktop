import os, re, subprocess, tempfile

EDGE = r"C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe"
HTML = r"D:\szudesktop\desktop\index.html"
OUT = r"D:\szudesktop\desktop\design\page-check.png"
ud = os.path.join(tempfile.gettempdir(), "edge_shot_szunet")

# 1) 量真实高度：小视口 + dump-dom 读 title
measure = HTML.replace(".html", ".measure.html")
with open(HTML, encoding="utf-8") as f:
    src = f.read()
src2 = src.replace("</head>",
    "<script>window.addEventListener('load',function(){document.title='H'+document.documentElement.scrollHeight+'H';});</script></head>")
with open(measure, "w", encoding="utf-8") as f:
    f.write(src2)

cmd = [EDGE, "--headless=new", "--disable-gpu", "--user-data-dir=" + ud,
       "--window-size=1280,300", "--no-first-run", "--no-default-browser-check",
       "--virtual-time-budget=4000", "--dump-dom", "file:///" + measure.replace("\\", "/")]
dom = subprocess.run(cmd, capture_output=True, timeout=180).stdout.decode("utf-8", "ignore")
m = re.search(r"<title>H(\d+)H</title>", dom)
h = int(m.group(1)) if m else 0
print("measured height:", h)

# 2) 按量出的高度截图（2x）
if os.path.exists(OUT):
    os.remove(OUT)
cmd = [EDGE, "--headless=new", "--disable-gpu", "--hide-scrollbars",
       "--force-device-scale-factor=2",
       "--window-size=1280,%d" % max(h + 20, 600),
       "--user-data-dir=" + ud,
       "--no-first-run", "--no-default-browser-check",
       "--virtual-time-budget=5000",
       "--screenshot=" + OUT,
       "file:///" + HTML.replace("\\", "/")]
r = subprocess.run(cmd, capture_output=True, timeout=180)
print("shot exit:", r.returncode, "size:", os.path.getsize(OUT) if os.path.exists(OUT) else 0)
os.remove(measure)
