"""打一个 Windows 首版发布包：dist/szudesktop-<版本>-windows-amd64.zip

包里放什么：
  szudesktop.exe        主程序（单文件，双击就跑）
  README-快速开始.txt   给不写代码的人看的，三句话说清怎么用
  LICENSE               MIT

为什么不打成一个安装器（.msi / .exe installer）：
  这个程序的卖点是"一个文件、双击就用、不需要安装"。
  校园网登录不会改系统代理；以后使用校外 VPN 的一键代理时，程序会备份并恢复
  当前用户的代理设置。做安装器反而要用户点一路"下一步"、还可能被安全软件拦。

用法: python make_release.py
"""
import hashlib
import os
import re
import sys
import zipfile

# GitHub Windows Runner 可能使用 CP1252；中文发布日志统一输出为 UTF-8。
for _stream in (sys.stdout, sys.stderr):
    if hasattr(_stream, "reconfigure"):
        _stream.reconfigure(encoding="utf-8", errors="replace")

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))  # 项目根，从脚本位置推
DESKTOP = os.path.join(ROOT, "desktop")
DIST = os.path.join(ROOT, "dist")
EXE = os.path.join(DIST, "szudesktop-windows-amd64.exe")
MAIN_GO = os.path.join(DESKTOP, "cmd", "szudesktop", "main.go")

README = """szuDesktop 深大校园服务台（beta0.4 · Windows）
=================================================

这是什么
--------
一个单文件的桌面小程序，用来代替每次上网都要手动打开的校园网登录页。
双击就能运行，不用安装。校园网登录本身不会修改系统代理设置。

怎么用
------
1. 双击 szudesktop.exe。
   它会自己在本机起一个小服务、然后打开浏览器当窗口（这是正常现象，窗口就是浏览器）。
2. 顶上点「校园网」，填 6 位校园卡号和统一身份认证密码。
3. 想下次免输就勾「记住账号密码」，然后点「登录」。
   - 密码由 Windows DPAPI 加密，只有这台机器的当前账户能解开，不会明文保存。
   - 不勾也能登录，本次用完即丢。
4. 程序会自动识别教学区（深澜）或宿舍区（Dr.COM）并走对应认证。
   本程序不会在后台每 30 秒自动重登；需要登录时由你点击按钮。
5. 在校外点顶部「校外 VPN」，填 VPN 账号密码后连接。
   需要短信验证码或动态口令时，在页面输入；连接后可选择打开 Windows 系统代理。
   断开 VPN 会恢复连接前的系统代理设置。VPN 密码不会显示在状态或日志里。

校内后端服务
------------
beta0.4 已预留校内后端服务的本地托管入口，但具体业务接口尚未接入。
后续确定接口协议后，再按白名单接入课表、一卡通等服务，不开放任意网址转发。

这一版改了什么（beta0.4）
------------------------
- 概览页改成二级菜单：总览 / 服务 / 农田 / 统计 / 动态 / 荔宝，点哪块看哪块。
- 登录失败涉及接入点编号（ac_id）时，页面自动展开高级选项让你手动填；平时收起。
- 界面瘦身：删掉了重复的「服务」标签页和一堆没人用的样式。

接了自己的路由器？
------------------
本版修好了「接路由器后登录不上」的问题（beta0.3 起）。

原因：校园网每个接入点有自己的编号（ac_id），你插哪个口就得报哪个号。
之前的版本固定用 1，而路由器那条线路要的是 12，编号报错就会被拒。

现在程序会这样找编号，从上到下、哪个先成用哪个：
  1. 你自己指定的（命令行 --ac-id 12）
  2. 这台机器在这个网口上上次认证成功用过的编号
  3. 没认证时被网络拦下来，从它给的跳转地址里读出来的（最可靠）
  4. 挨个试常见编号（只能证明编号存在，不一定是你这个，界面上会标「猜的」）

编号是跟着墙上那个网口走的，不跟路由器走：
换路由器插回原来那个口，编号不变；换个口才会变。

出现「认证失败：ac_id 用错了」时，用 szunet detect 看当前识别出的接入点，
必要时加 --ac-id 手动指定。

常见问题
--------
Q: 浏览器关掉了，程序还在跑吗？
A: 在跑。浏览器只是它的"窗口"，关掉窗口程序不死。要退出就在任务管理器里结束它，
   或者启动时加 --no-open 让它别开浏览器。

Q: 提示"还没有保存账号密码"？
A: 在应用顶部点「校园网」，填校园卡号和密码后点「登录」。勾上「记住账号密码」，
   认证成功后才会安全保存；密码错误时不会覆盖原来保存的凭据。

Q: 杀毒软件报警？
A: 因为这是个没有数字签名的单文件程序，属于常见误报。
   代码是开源的，可以自己看、自己编（go build）。

命令行参数（想进阶用的话）
--------------------------
  --no-open           不要自动打开应用窗口
  --no-auto-login     启动时不要使用已保存凭据自动登录一次
  --zone auto         区域选择：auto / teaching / dorm
  -u 卡号 -p 密码     本次临时指定账号密码（不保存）
  --addr 127.0.0.1:0  监听地址，默认随机端口
  --version           查看版本号

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
