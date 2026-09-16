"""打一个 Windows 首版发布包：dist/szudesktop-<版本>-windows-amd64.zip

包里放什么：
  szudesktop.exe        主程序（单文件，双击就跑）
  README-快速开始.txt   给不写代码的人看的，三句话说清怎么用
  LICENSE               MIT

为什么不打成一个安装器（.msi / .exe installer）：
  这个程序的全部卖点就是"一个文件、双击就用、不写注册表、不留残留"。
  做安装器反而要用户点一路"下一步"、还可能被安全软件拦。
  真要装到开始菜单，用户自己建个快捷方式就行。

用法: python make_release.py
"""
import hashlib
import os
import re
import sys
import zipfile

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))  # 项目根，从脚本位置推
DESKTOP = os.path.join(ROOT, "desktop")
DIST = os.path.join(ROOT, "dist")
EXE = os.path.join(DIST, "szudesktop-windows-amd64.exe")
MAIN_GO = os.path.join(DESKTOP, "cmd", "szudesktop", "main.go")

README = """szuDesktop 深大校园服务台（Windows 首版）
========================================

这是什么
--------
一个单文件的桌面小程序，用来代替每次上网都要手动打开的校园网登录页。
双击就能运行，不用安装，也不会往注册表里写东西。

怎么用
------
1. 双击 szudesktop.exe。
   它会自己在本机起一个小服务、然后打开浏览器当窗口（这是正常现象，窗口就是浏览器）。
2. 第一次用，先填校园卡号和密码，点「保存凭据」。
   - 密码存在 Windows 自己的加密设施里（DPAPI，只有这台机器、这个账户解得开），
     不会明文写在文件里。
3. 之后每次开机只要开着这个程序，它每 30 秒看一眼网络，
   掉线了会自动补登，不用管它。

常见问题
--------
Q: 浏览器关掉了，程序还在跑吗？
A: 在跑。浏览器只是它的"窗口"，关掉窗口程序不死。要退出就在任务管理器里结束它，
   或者启动时加 --no-open 让它别开浏览器。

Q: 提示"还没有保存账号密码"？
A: 就是字面意思，还没存过。按上面第 2 步存一次。

Q: 杀毒软件报警？
A: 因为这是个没有数字签名的单文件程序，属于常见误报。
   代码是开源的，可以自己看、自己编（go build）。

命令行参数（想进阶用的话）
--------------------------
  --no-open         不要自动开浏览器
  --no-auto-login   启动时不自动登录
  --no-keep-alive   不要常驻保持在线
  --interval 30     保持在线的检查间隔（秒）
  --addr 127.0.0.1:0  监听地址，默认随机端口
  --version         看版本号

注意
----
本程序只做「帮你登录校园网」这一件事，不含任何绕过计费或共享上网的功能。
请遵守学校网络使用规定。
"""


def read_version():
    m = re.search(r'const\s+version\s*=\s*"([^"]+)"',
                  open(MAIN_GO, encoding="utf-8").read())
    return m.group(1) if m else "0.0.0"


def main():
    ver = read_version()
    if not os.path.exists(EXE):
        print("!! 还没有编好的 exe，先跑 desktop/build-windows.py")
        sys.exit(1)

    out = os.path.join(DIST, "szudesktop-%s-windows-amd64.zip" % ver)
    exe_bytes = open(EXE, "rb").read()

    if os.path.exists(out):
        os.remove(out)

    with zipfile.ZipFile(out, "w", zipfile.ZIP_DEFLATED, compresslevel=9) as z:
        # 包里的文件名用 ascii，避免某些解压工具处理中文名出问题
        z.writestr("szudesktop.exe", exe_bytes)
        z.writestr("README-快速开始.txt", README)
        lic = os.path.join(ROOT, "LICENSE")
        if os.path.exists(lic):
            z.writestr("LICENSE", open(lic, "rb").read())

    # 打印清单 + 校验和，方便发布时贴出去
    print("->", out)
    print("   压缩前 %.1f MB / 压缩后 %.1f MB"
          % (len(exe_bytes) / 1024 / 1024, os.path.getsize(out) / 1024 / 1024))
    print("   版本 %s" % ver)
    print("   sha256 %s" % hashlib.sha256(exe_bytes).hexdigest())
    with zipfile.ZipFile(out) as z:
        print("   包内文件:")
        for i in z.infolist():
            print("     %-28s %8d 字节" % (i.filename, i.file_size))


if __name__ == "__main__":
    main()
