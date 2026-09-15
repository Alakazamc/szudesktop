"""生成「柯西」的像素头像（内联 SVG）。

为什么不用星露谷原版立绘：
  那几张 face-*.png 是 130x132 的**手绘插画**（不是像素画），缩到 52px
  糊成一团紫，跟页面里的像素字和点阵素材完全不是一个语言。
  而且 Abigail / Robin 都是游戏里的女性 NPC，拿来当"柯西"的头像也不对。
所以照荔宝（.who-face 里那段内联 SVG）的路子，一个 rect 一个 rect 画。

画法说明：
  - 画布 26x26 "像素"，每个像素 = 1 个 SVG 单位，实际显示时靠 CSS 放大。
    比直接写 52x52 的好处：点位全是对齐的小整数，改起来不用算半天。
  - 只画头肩（半身胸像），这是头像位的常见构图，52px 下也认得出来。
  - 配色取自页面 :root，保证跟主题一套。

用法:
  python gen_avatar.py            # 渲染一张预览图
  python gen_avatar.py --write    # 把 SVG 写回 desktop/index.html
"""
import os
import re
import sys

# ---------------------------------------------------------------- 调色板
# 命名规则：一个部位 3~4 个层次（底色/亮部/暗部/描边），像素画靠这个出体积感
PAL = {
    ".": None,              # 透明
    "o": "#3A2410",         # 描边（比 --ink 更深一档，压得住）
    "h": "#2A1A0C",         # 头发暗部
    "H": "#4A3A22",         # 头发中间调
    "g": "#6B5233",         # 头发亮部（顶光）
    "s": "#F0C098",         # 皮肤亮部
    "S": "#E0A87A",         # 皮肤底色
    "d": "#C08858",         # 皮肤暗部（下巴/脖子阴影）
    "e": "#3A2410",         # 眼睛
    "w": "#FFFFFF",         # 眼白/高光
    "m": "#B05A48",         # 嘴
    "c": "#5E9C3A",         # 衣服主色（页面 --ok 的绿，像校服/运动衫）
    "C": "#4C7F2E",         # 衣服暗部
    "l": "#D9B98A",         # 衣领（--cream-line）
    "k": "#C97C0A",         # 胸前校徽（--coin）
}

# ---------------------------------------------------------------- 点阵
# 26 宽 x 26 高。第 0~15 行是头，16 行往下是肩。
AVATAR = [
    "..........................",
    ".........ooooooo..........",
    ".......ooogggggoo.........",
    "......oggggggggggo........",
    ".....ogHHHHHHHHHHgo.......",
    "....ogHHHHHHHHHHHHgo......",
    "....oHHHgHHHHHHHHHHo......",
    "...ohHHgghhhhhhhhhhHo.....",
    "...ohHhhsssssssshhHHo.....",
    "...ohHhsssssssssshHHo.....",
    "...ohHhsseesseeshHHHo.....",
    "...ohHhsseesseeshHHHo.....",
    "...ohHhsssssssssshHHo.....",
    "...ohsssSssssssSssHo......",
    "...ohsssSsmmmSssssho......",
    "...ohsssssSSSsssssho......",
    "....ohssssssssssho........",
    ".....ohsssssssho..........",
    "......oohsssshoo..........",
    "......olllslllo...........",
    ".....occccssccccco........",
    "....occccccscccccco.......",
    "...occcccccscccccccco.....",
    "..occccccclklcccccccco....",
    "..oCCCCCCClklCCCCCCCCo....",
    "...oooooooooooooooooo.....",
]

W = len(AVATAR[0])
H = len(AVATAR)


def build():
    """点阵 -> SVG。把横向连续同色的像素合成一个 rect，输出小一半。"""
    for i, row in enumerate(AVATAR):
        assert len(row) == W, "第 %d 行宽度 %d，应该是 %d: %r" % (i, len(row), W, row)

    rects = []
    for y, row in enumerate(AVATAR):
        x = 0
        while x < W:
            ch = row[x]
            if PAL.get(ch) is None:
                x += 1
                continue
            n = 1
            while x + n < W and row[x + n] == ch:
                n += 1
            rects.append('<rect x="%d" y="%d" width="%d" height="1" fill="%s"/>'
                         % (x, y, n, PAL[ch]))
            x += n

    return ('<svg width="52" height="52" viewBox="0 0 %d %d" '
            'shape-rendering="crispEdges" role="img" '
            'aria-label="柯西：像素头像">%s</svg>' % (W, H, "".join(rects)))


def preview(path):
    """渲染一张放大 8 倍的 PNG，方便肉眼看。"""
    from PIL import Image
    im = Image.new("RGBA", (W, H), (0, 0, 0, 0))
    px = im.load()
    for y, row in enumerate(AVATAR):
        for x, ch in enumerate(row):
            col = PAL.get(ch)
            if col is None:
                continue
            r, g, b = (int(col[1:3], 16), int(col[3:5], 16), int(col[5:7], 16))
            px[x, y] = (r, g, b, 255)
    big = im.resize((W * 8, H * 8), Image.NEAREST)
    # 放到深色底上看透明区有没有问题
    bg = Image.new("RGB", big.size, (44, 62, 107))
    bg.paste(big, (0, 0), big)
    bg.save(path)
    print("->", path, bg.size)


def write_back(page_path):
    svg = build()
    p = open(page_path, encoding="utf-8").read()
    # 把 <img class="avatar" ...> 整行换成内联 SVG
    pat = re.compile(r'<img class="avatar"[^>]*>')
    if not pat.search(p):
        raise SystemExit("页面上没找到 <img class=\"avatar\" ...>，不写了")
    new = pat.sub(svg.replace("\\", "\\\\"), p, count=1)
    open(page_path, "w", encoding="utf-8", newline="").write(new)
    print("已写回", page_path, "SVG 长度", len(svg))


if __name__ == "__main__":
    here = os.path.dirname(os.path.abspath(__file__))
    if "--write" in sys.argv:
        write_back(os.path.join(here, "..", "index.html"))
    else:
        preview(os.path.join(here, "_avatar.png"))
        print("SVG 长度", len(build()))
