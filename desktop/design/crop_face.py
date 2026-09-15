"""从游戏立绘里裁出纯头像部分。

user.png / user0.png 是 182x173，里面其实有两块：
  · 上半部分是木框头像（真正的头像）
  · 下半部分是写着 "Robin's PC" 的木牌

我们的卡片只要头像，所以按实测边界把下面那块裁掉，再做成正方形。
实测：头像块 y 0-131，木牌从 y 144 开始。
"""
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from clean_signs import read_png, write_png

ART = r"D:\szuNet\desktop\assets\art"

# 头像块的边界（含木框）。多留 2px 免得切掉边框。
FACE_TOP = 0
FACE_BOTTOM = 132
FACE_LEFT = 26
FACE_RIGHT = 156


def crop(name, outname):
    w, h, nch, px = read_png(os.path.join(ART, name))

    l, t, r, b = FACE_LEFT, FACE_TOP, FACE_RIGHT, FACE_BOTTOM
    cw, ch = r - l, b - t
    out = bytearray(cw * ch * nch)
    for y in range(ch):
        src = ((t + y) * w + l) * nch
        dst = y * cw * nch
        out[dst:dst + cw * nch] = px[src:src + cw * nch]

    dst = os.path.join(ART, outname)
    write_png(dst, cw, ch, nch, out)
    print("  %-12s %dx%d -> %s %dx%d" % (name, w, h, outname, cw, ch))


if __name__ == "__main__":
    print("裁头像（去掉下面的名牌）：")
    crop("user0.png", "face-abigail.png")
    crop("user.png", "face-robin.png")
    print("done")
