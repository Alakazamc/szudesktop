"""截真服务页面的某一竖条（默认从顶部往下 N 像素），用来检查长页面下半截。

和 shot_live.py 的区别：那个只截首屏。这个会先把页面滚到指定位置再截。
无头模式下窗口不会真的"滚动可见"，所以做法是把 body 整体上移，
再配合固定的视口高度，等效于"往下看了一屏"。

用法: python shot_scroll.py <out.png> <scrollY> [width] [height] [scale]
"""
import os
import socket
import subprocess
import sys
import time
import urllib.request

EXE = r"D:\szuNet\dist\szudesktop-windows-amd64.exe"
TMPDIR = r"D:\szuNet\desktop"


def find_edge():
    for p in (r"C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe",
              r"C:\Program Files\Microsoft\Edge\Application\msedge.exe"):
        if os.path.exists(p):
            return p
    raise SystemExit("找不到 Edge")


def free_port():
    s = socket.socket()
    s.bind(("127.0.0.1", 0))
    p = s.getsockname()[1]
    s.close()
    return p


CSS = """
<style>
  /* 把整页往上挪，等效于滚动。用 !important 压过 body 原本的样式。 */
  html{overflow:hidden !important;}
  body{position:relative !important; top:-%dpx !important; overflow:visible !important;}
</style>
"""


def main(out, scroll, w="1920", h="1100", scale="1"):
    out = os.path.abspath(out)
    port = str(free_port())
    base = "http://127.0.0.1:" + port
    proc = subprocess.Popen([EXE, "--no-open", "--no-auto-login",
                             "--addr", "127.0.0.1:" + port],
                            stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
    tmp = os.path.join(TMPDIR, "_scroll.html")
    try:
        for _ in range(24):
            time.sleep(0.5)
            try:
                urllib.request.urlopen(base + "/api/status", timeout=3)
                break
            except Exception:
                pass
        else:
            raise SystemExit("服务没起来")

        page = urllib.request.urlopen(base + "/").read().decode("utf-8")
        page = page.replace("</head>", CSS % int(scroll) + "</head>", 1)
        open(tmp, "w", encoding="utf-8").write(page)

        cmd = [find_edge(), "--headless=new", "--disable-gpu", "--hide-scrollbars",
               "--force-device-scale-factor=" + scale,
               "--window-size=%s,%s" % (w, h),
               "--user-data-dir=" + os.path.join(os.environ["TEMP"], "edge-szu-scroll"),
               "--virtual-time-budget=9000",
               "--screenshot=" + out,
               "file:///" + tmp.replace("\\", "/")]
        r = subprocess.run(cmd, capture_output=True, timeout=180)
        print("滚动 %spx  exit:%s" % (scroll, r.returncode))
        print("->", out, os.path.getsize(out) if os.path.exists(out) else "(缺)")
    finally:
        try:
            os.remove(tmp)
        except OSError:
            pass
        proc.terminate()
        try:
            proc.wait(timeout=5)
        except Exception:
            proc.kill()
        subprocess.run(["taskkill", "/F", "/IM", os.path.basename(EXE)],
                       capture_output=True)


if __name__ == "__main__":
    if len(sys.argv) < 3:
        print(__doc__)
        sys.exit(1)
    main(*sys.argv[1:])
