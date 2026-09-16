# -*- coding: utf-8 -*-
"""荔宝立绘 v2 —— 按柯西给的官方设定图重画。

官方特征（2026-09-15 截图）：
  玫红圆滚滚身子、深紫红勾边、头顶偏左小呆毛、左手举起打招呼、
  V 形笑眼（开心闭眼）、大白嘴+粉舌、双侧腮红、深蓝小腿（鞋头分色）。

点阵 → 合并矩形 SVG，viewBox 0 0 52 56（26x28 格，每格 2 单位）。
替换 index.html 里两处旧「荔枝果实」SVG：
  1) .talk   处  aria-label="荔宝：像素荔枝精灵"
  2) .modal  处  aria-label="荔宝"

用法:
  python gen_libao.py            # 校验点阵 + 打印 SVG
  python gen_libao.py --preview  # 另存 _libao_preview.html 供截图目检
  python gen_libao.py --write    # 直接替换 index.html 两处（改前先备份判断）
"""
import re
import sys

PALETTE = {
    "O": "#6E1F35",  # 勾边：深紫红
    "R": "#ED4C67",  # 身体：玫红
    "r": "#F585A0",  # 高光浅粉
    "d": "#C93A55",  # 阴影暗玫红
    "K": "#4A1A28",  # 眼嘴线：近黑紫
    "W": "#FFFFFF",  # 嘴巴白
    "t": "#F08080",  # 舌头粉
    "B": "#31407A",  # 腿：藏青
    "b": "#232E5C",  # 鞋：深藏青
    "c": "#F78DA7",  # 腮红
}

# 26 列 x 28 行，每行必须严格 26 字符
DOTS = """
..........OO..............
.........ORRO.............
........OOOOOO............
.OOO....OOOOOOOOOO........
ORRO..OrrRRRRRRRRRRRO.....
ORRO.OrrRRRRRRRRRRRRO.....
.OOOOORRRRRRRRRRRRRRO.....
.ORROORRRKRRRRRRKRRRO.....
.OOOORRRKRKRRRRKRKRRRO....
....ORRRRRRRRRRRRRRRRO....
....ORRRRRKKKKKRRRRRRO....
....OccRRRKWWWKRRRRccO....
....OccRRRKWttKRRRRccO....
....ORRRRRKKKKKRRRRRRO....
....ORRRRRRRRRRRRRRRRO....
....ORRRRRRRRRRRRRRRRO....
....ORRRRRRRRRRRRRRRdO....
....ORRRRRRRRRRRRRddO.....
.....ORRRRRRRRRRRRddO.....
......ORRRRRRRRRRddO......
.......OddddddddddO.......
........OOOOOOOOOO........
........OBBBBOBBBBO.......
........OBBBBOBBBBO.......
........OBBBBOBBBBO.......
........ObbbbObbbbO.......
........ObbbbObbbbO.......
........OOOOOOOOOOO.......
""".strip("\n").split("\n")

W = 26


def check():
    bad = [(i, len(row), row) for i, row in enumerate(DOTS) if len(row) != W]
    for i, n, row in bad:
        print("第 %d 行宽 %d (应 %d): %r" % (i, n, W, row))
    return not bad


def render():
    """点阵 → 合并横向同色段的 <rect> 列表（格宽 2）。"""
    rects = []
    for y, row in enumerate(DOTS):
        x = 0
        while x < W:
            ch = row[x]
            if ch == ".":
                x += 1
                continue
            x0 = x
            while x < W and row[x] == ch:
                x += 1
            rects.append((x0 * 2, y * 2, (x - x0) * 2, 2, PALETTE[ch]))
    return rects


def svg_block(width, height, label):
    parts = ['<svg width="%d" height="%d" viewBox="0 0 52 56" role="img" aria-label="%s">' % (width, height, label)]
    for x, y, w, h, c in render():
        parts.append('<rect x="%d" y="%d" width="%d" height="%d" fill="%s"/>' % (x, y, w, h, c))
    parts.append("</svg>")
    return "".join(parts)


def symbol_block():
    """给页面精灵区用的可复用 symbol（登录页左边迎客那只）。"""
    parts = ['<symbol id="libao" viewBox="0 0 52 56">']
    for x, y, w, h, c in render():
        parts.append('<rect x="%d" y="%d" width="%d" height="%d" fill="%s"/>' % (x, y, w, h, c))
    parts.append("</symbol>")
    return "".join(parts)


SYMBOL_OLD = re.compile(r'<symbol id="libao".*?</symbol>', re.S)


TALK_OLD = re.compile(
    r'<svg width="54" height="62" viewBox="0 0 52 60" role="img" aria-label="荔宝：像素荔枝精灵">.*?</svg>',
    re.S)
MODAL_OLD = re.compile(
    r'<svg width="46" height="52" viewBox="0 0 52 60" role="img" aria-label="荔宝">.*?</svg>',
    re.S)

INDEX = r"D:\szudesktop\desktop\index.html"


def write_index():
    page = open(INDEX, encoding="utf-8").read()
    n1 = len(TALK_OLD.findall(page))
    n2 = len(MODAL_OLD.findall(page))
    print("找到旧立绘: talk=%d modal=%d" % (n1, n2))
    if n1 != 1 or n2 != 1:
        raise SystemExit("旧立绘数量不对，先人工看看再动手")
    page = TALK_OLD.sub(svg_block(54, 58, "荔宝"), page)
    page = MODAL_OLD.sub(svg_block(46, 50, "荔宝"), page)
    # 精灵区那份 symbol 一起换，登录页左边迎客的荔宝才不会落后
    n3 = len(SYMBOL_OLD.findall(page))
    print("找到旧 symbol: %d" % n3)
    if n3 == 1:
        page = SYMBOL_OLD.sub(symbol_block(), page)
    elif n3 > 1:
        raise SystemExit("symbol 有 %d 份，先人工看看" % n3)
    open(INDEX, "w", encoding="utf-8", newline="").write(page)
    print("已写回", INDEX)


def preview():
    html = """<!doctype html><meta charset="utf-8">
<style>html,body{margin:0;height:100%;}body{display:flex;gap:40px;align-items:center;justify-content:center;}
.a{background:#ffdfb0;padding:20px;}.b{background:#1b284b;padding:20px;}img,svg{image-rendering:pixelated;}</style>
<div class="a">__TALK__</div><div class="a">__MODAL__</div><div class="b">__TALK__</div>"""
    html = html.replace("__TALK__", svg_block(54, 58, "preview")).replace("__MODAL__", svg_block(92, 99, "preview2"))
    out = r"D:\szudesktop\desktop\design\_libao_preview.html"
    open(out, "w", encoding="utf-8", newline="").write(html)
    print("预览页 ->", out)


def main():
    if not check():
        raise SystemExit("点阵有坏行，先修")
    print("点阵 %dx%d 校验通过，rect 数 %d" % (W, len(DOTS), len(render())))
    if "--write" in sys.argv:
        write_index()
    elif "--preview" in sys.argv:
        preview()
    else:
        print(svg_block(54, 58, "荔宝")[:400], "……")


if __name__ == "__main__":
    main()
