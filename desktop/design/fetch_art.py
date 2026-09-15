"""直接问服务端：这些图片路径到底能不能取到、取到什么。

不走浏览器，纯 urllib —— 排除浏览器缓存/混合内容/时序等一切干扰。
检查三件事：
  1. HTTP 状态码是不是 200
  2. Content-Type 是不是图片
  3. 返回的字节数是不是和磁盘文件一模一样（md5 对比）

用法: python fetch_art.py
"""
import hashlib
import os
import socket
import subprocess
import sys
import time
import urllib.request

EXE = r"D:\szuNet\dist\szudesktop-windows-amd64.exe"
ART_DIR = r"D:\szuNet\desktop\assets\art"


def free_port():
    s = socket.socket()
    s.bind(("127.0.0.1", 0))
    p = s.getsockname()[1]
    s.close()
    return p


def free_port_unused():
    return free_port()


def main():
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

        names = sorted(os.listdir(ART_DIR))
        bad = 0
        print("服务 %s  共 %d 个素材" % (base, len(names)))
        for n in names:
            if not n.lower().endswith((".png", ".gif", ".jpg", ".jpeg")):
                continue
            url = base + "/art/" + n
            try:
                req = urllib.request.Request(url)
                with urllib.request.urlopen(req, timeout=8) as resp:
                    code = resp.status
                    ctype = resp.headers.get("Content-Type", "?")
                    clen = resp.headers.get("Content-Length", "?")
                    data = resp.read()
            except Exception as e:
                print("  [FAIL] %-24s %s" % (n, e))
                bad += 1
                continue
            disk = open(os.path.join(ART_DIR, n), "rb").read()
            same = hashlib.md5(data).hexdigest() == hashlib.md5(disk).hexdigest()
            flag = "OK " if (code == 200 and same) else "!! "
            if flag != "OK ":
                bad += 1
            print("  [%s] %-24s code=%s type=%-12s len=%-8s 与磁盘一致=%s"
                  % (flag.strip(), n, code, ctype, clen, "是" if same else "否"))
        print("\n异常 %d 项" % bad)
    finally:
        proc.terminate()
        try:
            proc.wait(timeout=5)
        except Exception:
            proc.kill()
        subprocess.run(["taskkill", "/F", "/IM", os.path.basename(EXE)],
                       capture_output=True)


if __name__ == "__main__":
    main()
