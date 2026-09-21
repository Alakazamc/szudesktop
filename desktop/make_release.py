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
VERSION_FILE = os.path.join(ROOT, "internal", "version", "VERSION")

README = """szuDesktop __VERSION__ · 荔枝庭院（Windows x64）

1. 解压后双击 szudesktop.exe，不需要安装，不会弹出命令行窗口。
2. 校园网账号与密码默认留空；填写后点击登录。勾选记住，仅在认证成功后保存。
3. 荔枝庭院可照料伙伴、种植、浇水、收获、出售作物并解锁装饰。
4. 学习工具支持本科/研究生成绩表批量导入、绩点统计、专注计时、官方自动校历与手动教学周。
5. 设置中可以导出 / 恢复存档，或立即完整退出应用。关闭所有窗口约 10 秒后自动退出。

校园卡号默认隐藏。Windows 密码由 DPAPI 加密；导出的存档不含账号密码。
庭院存档默认位于用户目录 .szunet/workspace-v1.json，更新程序不会清空它。
重复启动会复用同一份本机服务；换电脑前请导出存档。

官方学校服务需要网络，部分需要校园网或官方 WebVPN。本版不包含实验 VPN 协议，
不修改系统代理。校园服务可按学院/部门查看公开公告，列出 28 个学院与学部，其中 17 个支持直接读取；并提供社区场地实时空位和自习日历提醒。
社区公开空位查询需要校园网，仅供预览。点击「登录并预约」打开学校原页面，
在学校页面完成登录、选择时段、提交和查看结果。预约不再要求复制 Cookie，尚不支持应用内提交。
研究生课表提供应用内登录与手动读取（接入测试中，仍待真实账号完整验收）；
账号密码仅本次使用，登录状态随应用退出清除。学校验证码需要本人填写。
本科个人课表使用学校「我的课表」接口，保留时间地点原文；仍待有本科权限的真实账号验收。
本科需要先在官方页面登录，在应用成绩区域保存该业务的 ehall Cookie，再手动读取。
成绩表支持 CSV / TSV 或复制粘贴；不直接读取 PDF、图片和 XLSX。研究生不套用本科绩点规则。

学习工具另提供实验性在线成绩读取：仅在本机应用中输入对应学校业务的 Cookie，
安全存储失败时拒绝保存。当前只查询第一页，总数未知或尚未取全会明确提示，尚待真实成绩验收。
请勿把 Cookie 发到聊天或公开反馈中。清除本机会话不会注销学校浏览器登录。

这是学生自制的非官方测试版。庭院币没有真实货币价值，无充值、交易和提现。
本版尚无数字签名，校园认证仍需在实际教学区 / 宿舍网络验证。
源码与反馈：https://github.com/Alakazamc/szudesktop
"""


def read_version():
    """版本号只有一个来源：internal/version/VERSION（Go 二进制也嵌入同一个文件）。"""
    ver = open(VERSION_FILE, encoding="utf-8").read().strip()
    if not ver:
        print("!! %s 是空的，包名和快速开始都会写错版本" % VERSION_FILE)
        sys.exit(1)
    return ver


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
        z.writestr("README-快速开始.txt", README.replace("__VERSION__", ver))
        z.writestr("FONT-LICENSE-OFL.txt", Path(DESKTOP, "assets", "fonts", "LICENSE-OFL.txt").read_bytes())
        lic = os.path.join(ROOT, "LICENSE")
        if os.path.exists(lic):
            z.writestr("LICENSE", open(lic, "rb").read())

    Path(out+".sha256").write_text(hashlib.sha256(Path(out).read_bytes()).hexdigest()+"  "+os.path.basename(out)+"\n", encoding="ascii")
    # 打印清单 + 校验和，方便发布时贴出去
    print("->", out)
    print("   压缩前 %.1f MB / 压缩后 %.1f MB"
          % (len(exe_bytes) / 1024 / 1024, os.path.getsize(out) / 1024 / 1024))
    print("   版本 %s" % ver)
    print("   EXE sha256 %s" % hashlib.sha256(exe_bytes).hexdigest())
    print("   ZIP sha256 %s" % hashlib.sha256(Path(out).read_bytes()).hexdigest())
    with zipfile.ZipFile(out) as z:
        if z.read("szudesktop.exe") != Path(EXE).read_bytes():
            raise RuntimeError("包内程序与当前构建不一致，请重新打包")
        print("   包内文件:")
        for i in z.infolist():
            print("     %-28s %8d 字节" % (i.filename, i.file_size))


if __name__ == "__main__":
    main()
