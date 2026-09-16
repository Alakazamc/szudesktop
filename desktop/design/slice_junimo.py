"""从 loading-junimo.gif 里切出单只祝尼魔。

那是游戏加载画面用的一张 2300x400 长图，横着排了 5 只不同颜色的祝尼魔，
每只大概 460x400，脚下还有一小片阴影。整张图缩到页面上用会糊成一条，
所以按列切成单只，各自存成 PNG（保留透明）。

只处理第一帧：GIF 是逐帧动画，但这里只是拿它当"图"来切，
真正要动的地方我们用 CSS 的 steps() 来做，风格才统一。
"""
import os
import struct
import subprocess
import sys
import zlib

HERE = os.path.dirname(os.path.abspath(__file__))
ART = r"D:\szudesktop\desktop\assets\art"
SRC = r"D:\stardewOS-main\stardewOS-main\img\loading-junimo.gif"

# 5 只，从左到右。名字按颜色起，方便按需取用。
NAMES = ["junimo-cyan", "junimo-green", "junimo-pink", "junimo-yellow", "junimo-purple"]


def gif_to_png(gif, png):
    """GIF 解出来是带索引色的，用 Pillow 最省事。没有 Pillow 就让调用方装。"""
    from PIL import Image
    im = Image.open(gif)
    im.seek(0)
    im.convert("RGBA").save(png)
    return Image.open(png).size


def slice_sheet(png, names):
    """把一张横排的图按"透明缝隙"切开。

    不能按宽度平均切：每只祝尼魔宽度不一样（有的还把手臂伸出来了），
    平均切会把隔壁的手臂切进来、又把自己切掉一半。
    正确做法是按列扫，找整列全透明的位置当分界。
    """
    from PIL import Image
    im = Image.open(png).convert("RGBA")
    w, h = im.size
    px = im.load()

    # 每一列有没有内容
    col_has = []
    for x in range(w):
        has = any(px[x, y][3] > 8 for y in range(h))
        col_has.append(has)

    # 找连续的内容段
    spans = []
    start = None
    for x, has in enumerate(col_has):
        if has and start is None:
            start = x
        elif not has and start is not None:
            spans.append((start, x))
            start = None
    if start is not None:
        spans.append((start, w))

    # 太窄的段是噪点（阴影、碎像素），丢掉
    spans = [s for s in spans if s[1] - s[0] > 6]
    print("检测到 %d 段，宽度：" % len(spans), [e - s for s, e in spans])

    out = []
    for i, (x0, x1) in enumerate(spans):
        piece = im.crop((x0, 0, x1, h))
        bbox = piece.getbbox()          # 再去掉上下的空白
        if bbox:
            piece = piece.crop(bbox)
        name = names[i] if i < len(names) else "junimo-%d" % i
        dst = os.path.join(ART, name + ".png")
        piece.save(dst)
        out.append((name, piece.size))
    return out


if __name__ == "__main__":
    try:
        from PIL import Image  # noqa: F401
    except ImportError:
        print("需要 Pillow。装：pip install pillow")
        sys.exit(1)

    tmp = os.path.join(HERE, "_junimo.png")
    size = gif_to_png(SRC, tmp)
    print("GIF 尺寸 %dx%d" % size)
    res = slice_sheet(tmp, NAMES)
    os.remove(tmp)
    print("切出 %d 只：" % len(res))
    for n, s in res:
        print("  %-18s %dx%d" % (n, s[0], s[1]))
