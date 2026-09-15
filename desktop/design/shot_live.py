"""把真服务的页面截下来，人工看。

不用 dump-dom，直接 --screenshot。如果截图里图片是好的，
说明之前的"坏图"纯粹是 DOM 快照时序问题，页面本身没问题。

用法: python shot_live.py <out.png> [width] [height] [scale]
"""
import os
import socket
import subprocess
import sys
import time
import urllib.request

EXE = r"D:\szuNet\dist\szudesktop-windows-amd64.exe"


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


def main(out, w="1280", h="900", scale="1"):
    out = os.path.abspath(out)
    port = str(free_port())
    base = "http://127.0.0.1:" + port
    proc = subprocess.Popen([EXE, "--no-open", "--no-auto-login",
                             "--addr", "127.0.0.1:" + port],
                            stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
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

        cmd = [find_edge(), "--headless=new", "--disable-gpu", "--hide-scrollbars",
               "--force-device-scale-factor=" + scale,
               "--window-size=%s,%s" % (w, h),
               "--user-data-dir=" + os.path.join(os.environ["TEMP"], "edge-szu-shotlive"),
               "--virtual-time-budget=9000",
               "--screenshot=" + out,
               base + "/"]
        r = subprocess.run(cmd, capture_output=True, timeout=180)
        print("exit:", r.returncode)
        print("->", out, os.path.getsize(out) if os.path.exists(out) else "(缺)")
    finally:
        proc.terminate()
        try:
            proc.wait(timeout=5)
        except Exception:
            proc.kill()
        subprocess.run(["taskkill", "/F", "/IM", os.path.basename(EXE)],
                       capture_output=True)


if __name__ == "__main__":
    if len(sys.argv) < 2:
        print(__doc__)
        sys.exit(1)
    a = sys.argv[1:]
    main(*(a[:4]))
