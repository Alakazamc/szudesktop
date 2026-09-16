"""重新生成页面里的装饰层 HTML（两侧留白 + 卡片缝隙）。

为什么不手写：这是一份几十行的清单，每行都要算 calc() 的偏移量，
手写又长又容易错。用脚本算，改布局只要改参数重跑。

⚠️ 位置一律用 calc(50% ± 590px ± Npx) 锚到版心边缘，不要写死百分比。
   版心 .wrap 是 1180px 居中，左右留白随窗口宽度剧烈变化：
     1241px 视口只剩 30px，1440px 有 130px，1920px 有 370px。
   写死百分比的话窄屏会压到卡片上、宽屏又离得太远。锚边缘就永远贴身。

用法: python gen_ground.py            # 打印 HTML
      python gen_ground.py --write    # 直接替换进 desktop/index.html
"""
import os
import re
import sys

PAGE = r"D:\szudesktop\desktop\index.html"
HALF = 590        # 版心半宽（1180/2）

# 纵向范围：标题牌 + 标签栏占到 y≈325，footer 从 y≈1662 开始
Y_TOP = 340
Y_BOT = 1620
# 页高 1707，卡片列实测 x=47~1195（1241px 视口下）

# (文件名, 宽度, 距版心边缘的偏移, 纵向位置, 动画class)
# 偏移越大离版心越远；留白窄的窗口会被 overflow:hidden 裁掉，不影响
LEFT = [
    ("tree.png",      56,  10,  360, ""),
    ("tallgrass.png", 38,  66,  500, "s2"),
    ("berrybush.png", 42,  14,  640, ""),
    ("fern.png",      36,  70,  790, "s3"),
    ("tuft.png",      32,  18,  930, "s2"),
    ("pumpkin.png",   30,  74, 1060, ""),
    ("chicken.png",   38,  16, 1150, "chick"),
    ("tallgrass.png", 36,  12, 1280, "s4"),
    ("rock.png",      28,  68, 1370, ""),
    ("mushroom.png",  24,  64, 1460, "s2"),
    ("cat.png",       50,  20, 1540, "catnap"),
]
RIGHT = [
    ("tree.png",      56,  10,  380, "s2"),
    ("tallgrass.png", 38,  66,  530, "s3"),
    ("mushroom.png",  26,  16,  670, ""),
    ("fern.png",      36,  72,  810, "s4"),
    ("tuft.png",      32,  18,  950, "s2"),
    ("berrybush.png", 42,  68, 1080, ""),
    ("pumpkin.png",   30,  14, 1200, "s2"),
    ("rock.png",      28,  70, 1330, ""),
    ("chicken.png",   38,  16, 1420, "chick"),
    ("cat.png",       50,  20, 1530, "catnap"),
    ("mushroom.png",  24,  66, 1610, "s3"),
]

# 版心左右边缘的 x 位置（相对容器）：50% ∓ 590px
EDGE_L = "calc(50%% - %dpx)" % HALF
EDGE_R = "calc(50%% + %dpx)" % HALF

# 卡片列结构：296 + 14 + 508 + 14 + 316 = 1148
# 两条缝分别起于 +296 和 +296+14+508 = +818，缝宽 14px
SEAM1 = "calc(50%% - %dpx + 296px)" % HALF
SEAM2 = "calc(50%% - %dpx + 818px)" % HALF
# 缝里只能放 ≤12px 的东西，位置 +2px 居中
SEAM_INSET = 2


def cls(base, extra):
    return (base + " " + extra).strip()


def build():
    out = []
    out.append("<!-- 背景土地上的花草动物。")
    out.append("     位置全部用 calc(50% ± 590px ...) 锚到版心边缘（为什么见上面 CSS 的注释）。")
    out.append("     纵向从 340px 铺到 1620px —— 上面留给标题牌和标签栏，下面避开页脚文字。")
    out.append("     卡片是不透明的，所以只有「左右留白」和「卡片缝隙」这两处能露出东西。 -->")
    out.append('<div class="ground" aria-hidden="true">')

    out.append("  <!-- 左侧留白：右边缘贴版心左边缘，偏移越大越靠外 -->")
    for name, w, off, y, anim in LEFT:
        out.append('  <img class="%s" src="assets/art/flora/%s" '
                   'style="right:calc(50%% + %dpx + %dpx); top:%dpx; width:%dpx;">'
                   % (cls("L", anim), name, HALF, off, y, w))

    out.append("")
    out.append("  <!-- 右侧留白 -->")
    for name, w, off, y, anim in RIGHT:
        out.append('  <img class="%s" src="assets/art/flora/%s" '
                   'style="left:calc(50%% + %dpx + %dpx); top:%dpx; width:%dpx;">'
                   % (cls("R", anim), name, HALF, off, y, w))

    out.append("")
    out.append("  <!-- ⚠️ 卡片之间那两条 14px 的竖缝：试过在里面种花，全被卡片盖住了，")
    out.append("       缝本身装不下 14px 的图（还有边框和投影）。这条路走不通，别再加回来。")
    out.append("       要塞的话塞进卡片内部（.in-deco），见下面那段。 -->")

    out.append("")
    out.append("  <!-- 会飞的：落在两侧留白里，绕着圈飞 -->")
    bflies = [
        ("L", 30, 620, 20, None),
        ("L", 100, 1010, 18, -3.5),
        ("L", 56, 1440, 18, -7),
        ("R", 40, 760, 20, -6),
        ("R", 120, 1250, 18, -1.5),
        ("R", 70, 1520, 18, -4.5),
    ]
    for side, off, y, w, delay in bflies:
        prop = "right" if side == "L" else "left"
        extra = ' animation-delay:%ss;' % delay if delay else ""
        out.append('  <img class="bfly" src="assets/art/flora/butterfly.png" '
                   'style="%s:calc(50%% + %dpx + %dpx); top:%dpx; width:%dpx;%s">'
                   % (prop, HALF, off, y, w, extra))

    out.append("</div>")
    return "\n".join(out)


def main():
    html = build()
    if "--write" not in sys.argv:
        print(html)
        return

    src = open(PAGE, encoding="utf-8").read()
    # 替换从 <!-- 背景土地上 开头到对应 </div> 结束的整块
    start = src.find("<!-- 背景土地上的花草动物")
    if start < 0:
        raise SystemExit("找不到装饰层注释起点")
    # 从起点往后找第一个 </div>（装饰层自己的收尾）
    end = src.find("</div>", start)
    end += len("</div>")
    # 还要把它后面紧邻的 ground-low 块一起删掉（如果存在）
    tail = src[end:]
    m = re.match(r"\s*<!--[^>]*?-->\s*<div class=\"ground-low\"[\s\S]*?</div>", tail)
    if m:
        end += m.end()
    new = src[:start] + html + src[end:]
    open(PAGE, "w", encoding="utf-8").write(new)
    print("已写回 %s（%d -> %d 字节）" % (PAGE, len(src), len(new)))


if __name__ == "__main__":
    main()
