"""打一个 Windows 首版发布包：dist/szudesktop-<版本>-windows-amd64.zip

包里放什么：
  szudesktop.exe        主程序（单文件，双击就跑）
  README-快速开始.txt   给不写代码的人看的，三句话说清怎么用
  LICENSE               MIT

为什么不打成一个安装器（.msi / .exe installer）：
  这个程序的卖点是"一个文件、双击就用、不需要安装"。
  默认发行版只提供官方 WebVPN 入口，不修改系统代理。

用法: python make_release.py
"""
import hashlib
import os
import re
import sys
import zipfile
from pathlib import Path

# GitHub Windows Runner 可能使用 CP1252；中文发布日志统一输出为 UTF-8。
for _stream in (sys.stdout, sys.stderr):
    if hasattr(_stream, "reconfigure"):
        _stream.reconfigure(encoding="utf-8", errors="replace")

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))  # 项目根，从脚本位置推
DESKTOP = os.path.join(ROOT, "desktop")
DIST = os.path.join(ROOT, "dist")
EXE = os.path.join(DIST, "szudesktop-windows-amd64.exe")
MAIN_GO = os.path.join(DESKTOP, "cmd", "szudesktop", "main.go")

README = """szuDesktop beta0.5 · 荔枝庭院（Windows x64）

1. 解压后双击 szudesktop.exe，不需要安装，不会弹出命令行窗口。
2. 校园网账号与密码默认留空；填写后点击登录。勾选记住，仅在认证成功后保存。
3. 荔枝庭院可照料伙伴、种植、浇水、收获、出售作物并解锁装饰。
4. 学习工具提供待办、专注计时、绩点计算与自设教学周。
5. 设置中可以导出 / 恢复存档，或完整退出应用。只关闭浏览器不会停止服务。

校园卡号默认隐藏。Windows 密码由 DPAPI 加密；导出的存档不含账号密码。
庭院存档默认位于用户目录 .szunet/workspace-v1.json，更新程序不会清空它。
旧版浏览器存档不能自动跨端口迁移，更新前请保留旧数据。

官方学校服务需要网络，部分需要校园网或官方 WebVPN。本版不包含实验 VPN 协议，
不修改系统代理。学校业务数据、远程存档同步和公告后端尚未接入。

这是学生自制的非官方测试版。庭院币没有真实货币价值，无充值、交易和提现。
本版尚无数字签名，校园认证仍需在实际教学区 / 宿舍网络验证。
源码与反馈：https://github.com/Alakazamc/szudesktop
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

    Path(out+".sha256").write_text(hashlib.sha256(Path(out).read_bytes()).hexdigest()+"  "+os.path.basename(out)+"\n", encoding="ascii")
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
