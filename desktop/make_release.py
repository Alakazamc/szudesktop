"""打一个 Windows 首版发布包：dist/szudesktop-<版本>-windows-amd64.zip

包里放什么：
  szudesktop.exe        主程序（单文件，双击就跑）
  README-快速开始.txt   给不写代码的人看的，三句话说清怎么用
  LICENSE               MIT

这是与 Electron 安装版并行保留的便携包；两者使用同一份 Go 引擎与庭院资源。
Electron 安装版由 desktop/electron/build.mjs 另行打包。
默认发行版只提供官方 WebVPN 入口，不修改系统代理。

用法: python make_release.py
"""
import hashlib
import os
import sys
import zipfile
from pathlib import Path

import release_notes  # 同目录：发布说明的抽取与校验

# GitHub Windows Runner 可能使用 CP1252；中文发布日志统一输出为 UTF-8。
for _stream in (sys.stdout, sys.stderr):
    if hasattr(_stream, "reconfigure"):
        _stream.reconfigure(encoding="utf-8", errors="replace")

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))  # 项目根，从脚本位置推
DESKTOP = os.path.join(ROOT, "desktop")
DIST = os.path.join(ROOT, "dist")
EXE = os.path.join(DIST, "szudesktop-windows-amd64.exe")
VERSION_FILE = os.path.join(ROOT, "internal", "version", "VERSION")

README = """szuDesktop __VERSION__ · 荔枝庭院（Windows x64 便携版）

1. 解压后双击 szudesktop.exe，不需要安装，使用本机 Edge / Chrome 开窗，不会弹出命令行窗口。
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
由本人在学校页面完成登录、选择时段、提交和查看结果。本便携版使用系统浏览器，
不要求复制预约 Cookie，也没有本机预约写接口；学校连接不稳定时请稍后重试。
研究生课表的备用登录与当前课表已用真实账号读取；非空排课和完整账号业务仍待验收。
账号密码仅本次使用，登录状态随应用退出清除。学校验证码需要本人填写。
本科个人课表使用学校「我的课表」接口，保留时间地点原文；仍待有本科权限的真实账号验收。
本科可在成绩区域的备用登录中尝试统一身份认证，再手动读取；保留手动会话备选。
成绩表支持 CSV / TSV 或复制粘贴；不直接读取 PDF、图片和 XLSX。研究生不套用本科绩点规则。

学习工具另提供实验性在线成绩读取：仅在本机应用中输入对应学校业务的 Cookie，
安全存储失败时拒绝保存。当前只查询第一页，总数未知或尚未取全会明确提示，尚待真实成绩验收。
请勿把 Cookie 发到聊天或公开反馈中。清除本机会话不会注销学校浏览器登录。

这是学生自制的非官方测试版。庭院币没有真实货币价值，无充值、交易和提现。
本版尚无数字签名，校园认证仍需在实际教学区 / 宿舍网络验证。
源码与反馈：https://github.com/SzuDesktopTeam/szudesktop
同一发布页的 szuDesktop-Setup 是 Electron 安装版，自带窗口运行时；本便携版不含独立宠物窗。
安装版支持左键/右键点击宠物打开菜单：摸头、喂食、陪玩、睡觉，以及打开庭院、农田、学习工具。
鼠标停在宠物上滚动，每次调整 10 个百分点，范围 40%–200%；也可用菜单、设置滑杆或托盘预设。
按住宠物可拖动，大小与位置会在重启后恢复，照料与窗口内庭院共用本机存档。
安装版另有托盘和应用内学校原页登录；学校预约全流程、业务会话接回和琴房权限尚未验收通过。
安装版关主窗口后继续常驻，从宠物菜单「打开主窗口」或托盘重开。
完整退出请用宠物菜单「退出应用」、托盘「退出」或「设置 → 退出应用」。
"""


def read_version():
    """版本号只有一个来源：internal/version/VERSION（Go 二进制也嵌入同一个文件）。"""
    ver = open(VERSION_FILE, encoding="utf-8").read().strip()
    if not ver:
        print("!! %s 是空的，包名和快速开始都会写错版本" % VERSION_FILE)
        sys.exit(1)
    return ver


def check_changelog(ver):
    """打包前先确认 CHANGELOG.md 里写好了这个版本的发布说明。

    CI 的 release job 会再拦一次，但拦在打包之前更有用：VERSION 一升、
    CHANGELOG 忘了写，PR 阶段就会红，而不是等打完 tag 才发现说明是空的
    （beta0.7 就是这样发出去的，正文只有一行 compare 链接）。
    """
    try:
        release_notes.extract(release_notes.load(), ver)
    except release_notes.NotesError as e:
        print("!! " + str(e))
        sys.exit(1)
    except OSError as e:
        print("!! 读不到 CHANGELOG.md: %s" % e)
        sys.exit(1)


def main():
    ver = read_version()
    check_changelog(ver)
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

    # newline="\n" 不能省：这个脚本在 Windows 上跑（CI 的 windows 任务也一样），
    # 文本模式默认把 \n 翻成 \r\n，下载者用 Linux/macOS 的 sha256sum -c 会直接失败
    # （coreutils 不会去掉行尾的 \r，会当成文件名的一部分）。
    Path(out+".sha256").write_text(hashlib.sha256(Path(out).read_bytes()).hexdigest()+"  "+os.path.basename(out)+"\n", encoding="ascii", newline="\n")
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
