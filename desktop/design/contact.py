"""把素材源里所有图片拼成一张放大的"图鉴"，方便一眼看清都是什么。

为什么需要：素材源里有一堆名字看不出内容的图（f1/f2/t1/t2/stars...），
与其一个个 Read 打开，不如拼成一张带名字的墙，一次看完。

用 Pillow（已装）。透明背景铺成棋盘格，小图放大到能看清像素。

用法: python contact.py <素材目录> <输出.png> [放大倍数]
"""
import os
import sys

from PIL import Image, ImageDraw

CELL = 150          # 每格边长
PAD = 6
LABEL_H = 18
COLS = 8


def checker(size, s=8):
    """画棋盘格当底，透明区域一眼可见。"""
    img = Image.new("RGBA", size, (60, 60, 70, 255))
    d = ImageDraw.Draw(img)
    for y in range(0, size[1], s):
        for x in range(0, size[0], s):
            if (x // s + y // s) % 2:
                d.rectangle([x, y, x + s - 1, y + s - 1], fill=(82, 82, 94, 255))
    return img


def main(srcdir, out, zoom=None):
    names = sorted(f for f in os.listdir(srcdir)
                   if f.lower().endswith((".png", ".gif", ".jpg", ".jpeg")))
    # avast / bg 之类的大背景单独跳过，不然格子撑爆
    skip = {"bg.png", "bg-2.jpg", "bg-test.png"}
    names = [n for n in names if n not in skip]

    rows = (len(names) + COLS - 1) // COLS
    W = COLS * (CELL + PAD) + PAD
    H = rows * (CELL + LABEL_H + PAD) + PAD
    sheet = Image.new("RGB", (W, H), (28, 28, 34))

    try:
        from PIL import ImageFont
        font = ImageFont.load_default()
    except Exception:
        font = None

    for i, n in enumerate(names):
        col, row = i % COLS, i // COLS
        x = PAD + col * (CELL + PAD)
        y = PAD + row * (CELL + LABEL_H + PAD)
        try:
            im = Image.open(os.path.join(srcdir, n))
            im.load()
            im = im.convert("RGBA")
        except Exception as e:
            print("  跳过 %s (%s)" % (n, e))
            continue

        ow, oh = im.size
        z = zoom or max(1, min(6, CELL // max(ow, oh) or 1))
        big = im.resize((ow * z, oh * z), Image.NEAREST)
        if big.width > CELL or big.height > CELL:
            big = big.resize((min(CELL, big.width), min(CELL, big.height)), Image.NEAREST)

        tile = checker((CELL, CELL))
        tile.alpha_composite(big, ((CELL - big.width) // 2, (CELL - big.height) // 2))
        sheet.paste(tile.convert("RGB"), (x, y))

        d = ImageDraw.Draw(sheet)
        d.text((x + 2, y + CELL + 3), "%s %dx%d" % (n, ow, oh), fill=(200, 200, 180), font=font)

    sheet.save(out)
    print("->  %s  %dx%d  共 %d 张" % (out, W, H, len(names)))


if __name__ == "__main__":
    if len(sys.argv) < 3:
        print(__doc__)
        sys.exit(1)
    main(sys.argv[1], sys.argv[2], sys.argv[3] if len(sys.argv) > 3 else None)
