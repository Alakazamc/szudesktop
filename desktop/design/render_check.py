import os, subprocess, tempfile

EDGE = r"C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe"
HTML = r"D:\szuNet\desktop\design\prototype-v2.html"
OUT = r"D:\szuNet\desktop\design\prototype-v2.png"
ud = os.path.join(tempfile.gettempdir(), "edge_shot_szunet")

if os.path.exists(OUT):
    os.remove(OUT)

cmd = [
    EDGE, "--headless=new", "--disable-gpu", "--hide-scrollbars",
    "--force-device-scale-factor=2",
    "--window-size=680,520",
    "--user-data-dir=" + ud,
    "--no-first-run", "--no-default-browser-check",
    "--screenshot=" + OUT,
    "file:///" + HTML.replace("\\", "/"),
]
r = subprocess.run(cmd, capture_output=True, timeout=180)
print("exit:", r.returncode)
print("exists:", os.path.exists(OUT), "size:", os.path.getsize(OUT) if os.path.exists(OUT) else 0)
if r.stderr:
    err = r.stderr.decode("utf-8", "ignore")
    print("stderr tail:", err[-300:] if len(err) > 300 else err)
