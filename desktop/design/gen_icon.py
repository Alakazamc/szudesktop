"""生成 Windows 应用图标 szudesktop.ico。

画什么：一颗像素荔枝（深大的"荔"），跟页面里的荔宝同一套视觉语言。
为什么要自己画而不是拿现成的：
  星露谷素材包里没有荔枝，而且那批图是插画风、不是像素画，做出来的图标
  跟应用本身不是一个语言。这里跟头像一样用点阵字符串画。

ICO 里塞几个尺寸：16/32/48/64/128/256。
Windows 会按显示场景自己挑（任务栏用 32，桌面大图标用 256）。
**16 和 32 必须单独画**——直接把 256 缩下来，眼睛和纹路会糊成一片，
所以这两个尺寸用简化版点阵（去掉小噪点，加粗轮廓）。

用法: python gen_icon.py            # 生成 desktop/assets/szudesktop.ico
"""
import io
import os

from PIL import Image

# ---------------------------------------------------------------- 调色板
PAL = {
    ".": None,
    "o": "#2A1A0C",     # 描边
    "r": "#D24A3C",     # 果壳亮部
    "R": "#A83029",     # 果壳底色
    "q": "#7A1F1A",     # 果壳暗部
    "g": "#5EA83C",     # 叶子亮部
    "G": "#3E7A26",     # 叶子底色
    "w": "#F0A0A8",     # 高光
}

# 16x16：小尺寸用的简化版。轮廓加粗、去掉细节，远看还是颗荔枝。
SMALL = [
    "......ooo.......",
    ".....oGGo.......",
    "....ogggo.......",
    "..oooGGGo.......",
    ".oRRRooRo.......",
    "oRRRRRRRo.......",
    "oRwRRRRRo.......",
    "oRwRRRRRo.......",
    "oRRRRRRRo.......",
    ".oRRRRRo........",
    "..oRRRo.........",
    "...ooo..........",
    "................",
    "................",
    "................",
    "................",
]

# 32x32：中等尺寸，能放下龟裂纹理
MID = [
    "............oooo................",
    "..........ooGGGGo...............",
    ".........oggggGGo...............",
    "........ogggooGGGo..............",
    ".....oooogGo..oGGGo.............",
    "...ooRRRRooo...oGGo.............",
    "..oRRRRRRRRRooo.oGo.............",
    "..oRRRRRRRRRRRRooo..............",
    ".oRRRwRRRRRRRRRRRo..............",
    ".oRRwRRRRRRRRRRRRo..............",
    ".oRRRqqRRRRRqqRRRRo.............",
    ".oRRRRqRRRRRqRRRRRo.............",
    "..oRRRRRRRRRRRRRRRo.............",
    "..oRRRqqRRRRRqqRRRo.............",
    "...oRRqRRRRRqRRRRo..............",
    "...oRRRRRRRRRRRRRo..............",
    "....oRRRqqRRRqqRRo..............",
    "....oRRqRRRRqRRRo...............",
    ".....oRRRRRRRRRo................",
    ".....oRRqqRRRRo.................",
    "......oRqRRRRo..................",
    "......oRRRRRo...................",
    ".......oRRRo....................",
    ".......oRRo.....................",
    "........oo......................",
    "................................",
    "................................",
    "................................",
    "................................",
    "................................",
    "................................",
    "................................",
]


def render(rows, size):
    """点阵 -> PIL Image（正方形，缩放到 size）

    行宽不齐时**右侧补透明**而不是报错：图标这种东西是拿眼睛调的，
    写的时候顺手多按少按一两个点很正常，不该因此编不出图标。
    （但会在下面打印一次提醒，避免真的写错还蒙在鼓里。）
    """
    h = len(rows)
    w = max(len(r) for r in rows)
    ragged = [i for i, r in enumerate(rows) if len(r) != w]
    if ragged:
        print("   注意: 第 %s 行宽度不是 %d，已按右侧补透明处理"
              % (", ".join(map(str, ragged[:6])), w))
    rows = [r.ljust(w, ".") for r in rows]

    im = Image.new("RGBA", (w, h), (0, 0, 0, 0))
    px = im.load()
    for y, row in enumerate(rows):
        for x, ch in enumerate(row):
            col = PAL.get(ch)
            if col is None:
                continue
            px[x, y] = (int(col[1:3], 16), int(col[3:5], 16), int(col[5:7], 16), 255)
    # 点阵本身留了边距，裁掉再缩放，图标才占得满
    bbox = im.getbbox()
    if bbox:
        im = im.crop(bbox)
    # 保持正方
    side = max(im.size)
    sq = Image.new("RGBA", (side, side), (0, 0, 0, 0))
    sq.paste(im, ((side - im.width) // 2, (side - im.height) // 2), im)
    return sq.resize((size, size), Image.NEAREST)


def main(out):
    # 16/32 用专门画的小图；48 以上用 MID 放大（像素画放大不糊）
    imgs = [
        render(SMALL, 16),
        render(SMALL, 32),
        render(MID, 48),
        render(MID, 64),
        render(MID, 128),
        render(MID, 256),
    ]
    write_ico(out, imgs)
    print("->", out, os.path.getsize(out), "字节",
          "尺寸:", [i.size for i in imgs])


# ---------------------------------------------------------------- ICO 容器
# 为什么不直接用 Pillow 的 save(format="ICO", sizes=..., append_images=...):
#   Pillow 12 起那个多尺寸写法不认 append_images 了，只会存下第一张
#   （实测只剩 16x16）。ICO 格式本身很简单，自己拼更省心，也不用赌版本行为。
#
# ICO = 6 字节头 + N*16 字节目录 + N 张 PNG 数据（Vista 起可以直接内嵌 PNG）
def write_ico(path, imgs):
    import struct
    count = len(imgs)
    header = struct.pack("<HHH", 0, 1, count)   # reserved, type=1(icon), count
    entries, blobs = b"", b""
    offset = 6 + 16 * count
    for im in imgs:
        buf = io.BytesIO()
        im.save(buf, format="PNG")
        data = buf.getvalue()
        # 宽高字段是 1 字节，256 要写成 0（这是 ICO 格式的历史包袱）
        w = 0 if im.width >= 256 else im.width
        h = 0 if im.height >= 256 else im.height
        entries += struct.pack("<BBBBHHII", w, h, 0, 0, 1, 32, len(data), offset)
        blobs += data
        offset += len(data)
    with open(path, "wb") as f:
        f.write(header + entries + blobs)


if __name__ == "__main__":
    here = os.path.dirname(os.path.abspath(__file__))
    main(os.path.join(here, "..", "assets", "szudesktop.ico"))
