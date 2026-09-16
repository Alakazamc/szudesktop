"""把 m1-m4 告示牌上的英文擦掉，留一块干净木牌。

原图是四张 230x181 的木头告示牌，牌面上印着大字（MAP / WIKI / BING / PLAY）
和一个物品小图标。我们只要那张"牌子"，文字自己用中文写。

思路：牌心是一块颜色平缓的浅木色，所以直接从牌心取一小条竖直样本，
横向铺满整个牌心，就能把字和图标一起盖掉，而且不留痕迹。

牌心范围是量出来的（见 CARD）：外侧留边框，内侧留一点纸面边距。
"""
import os
import struct
import zlib

SRCDIR = r"D:\stardewOS-main\stardewOS-main\img"
DSTDIR = r"D:\szudesktop\desktop\assets\art"

# 牌心：左右上下各留出边框，中间这块是"可以随便涂"的纸面
CARD = (34, 30, 196, 150)   # left, top, right, bottom

# 牌心实测是一整块平色 #FED788（254,215,136），所以直接填平色最干净，
# 不留采样带的痕迹。四张牌的牌心颜色实测一致。
FACE = (254, 215, 136)

# 如果某张牌的牌心色不同，在这里单独指定
FACE_OVERRIDE = {}


def read_png(path):
    """读 PNG，返回 (width, height, pixel 列表)。只处理 8 位 RGB/RGBA、非隔行。"""
    d = open(path, "rb").read()
    assert d[:8] == b"\x89PNG\r\n\x1a\n", path
    pos = 8
    w = h = None
    idat = b""
    bitd = colort = None
    while pos < len(d):
        ln = struct.unpack(">I", d[pos:pos + 4])[0]
        typ = d[pos + 4:pos + 8]
        body = d[pos + 8:pos + 8 + ln]
        if typ == b"IHDR":
            w, h, bitd, colort, comp, filt, inter = struct.unpack(">IIBBBBB", body)
            assert bitd == 8 and inter == 0, "只支持 8 位非隔行"
        elif typ == b"IDAT":
            idat += body
        elif typ == b"IEND":
            break
        pos += 12 + ln

    nch = {0: 1, 2: 3, 3: 1, 4: 2, 6: 4}[colort]
    raw = zlib.decompress(idat)
    stride = w * nch
    out = bytearray(w * h * nch)
    prev = bytearray(stride)
    p = 0
    for y in range(h):
        f = raw[p]
        p += 1
        line = bytearray(raw[p:p + stride])
        p += stride
        # 逐字节反滤波
        for i in range(stride):
            a = line[i - nch] if i >= nch else 0
            b = prev[i]
            c = prev[i - nch] if i >= nch else 0
            if f == 1:
                line[i] = (line[i] + a) & 0xFF
            elif f == 2:
                line[i] = (line[i] + b) & 0xFF
            elif f == 3:
                line[i] = (line[i] + (a + b) // 2) & 0xFF
            elif f == 4:
                pp = a + b - c
                pa, pb, pc = abs(pp - a), abs(pp - b), abs(pp - c)
                pr = a if (pa <= pb and pa <= pc) else (b if pb <= pc else c)
                line[i] = (line[i] + pr) & 0xFF
        out[y * stride:(y + 1) * stride] = line
        prev = line
    return w, h, nch, out


def write_png(path, w, h, nch, pixels):
    ct = {1: 0, 2: 4, 3: 2, 4: 6}[nch]
    raw = bytearray()
    stride = w * nch
    for y in range(h):
        raw.append(0)  # filter: none
        raw += pixels[y * stride:(y + 1) * stride]

    def chunk(typ, data):
        c = struct.pack(">I", len(data)) + typ + data
        return c + struct.pack(">I", zlib.crc32(typ + data) & 0xFFFFFFFF)

    png = b"\x89PNG\r\n\x1a\n"
    png += chunk(b"IHDR", struct.pack(">IIBBBBB", w, h, 8, ct, 0, 0, 0))
    png += chunk(b"IDAT", zlib.compress(bytes(raw), 9))
    png += chunk(b"IEND", b"")
    open(path, "wb").write(png)


def clean(name):
    src = os.path.join(SRCDIR, name)
    w, h, nch, px = read_png(src)
    l, t, r, b = CARD
    face = FACE_OVERRIDE.get(name, FACE)

    # 用平色铺满牌心。alpha 保持原值（牌心是不透明的）
    cleared = 0
    for y in range(t, b):
        for x in range(l, r):
            off = (y * w + x) * nch
            px[off] = face[0]
            px[off + 1] = face[1]
            px[off + 2] = face[2]
            cleared += 1

    dst = os.path.join(DSTDIR, name)
    write_png(dst, w, h, nch, px)
    print("  %-8s %dx%d  擦了 %d 像素 -> %s" % (name, w, h, cleared, os.path.basename(dst)))


if __name__ == "__main__":
    print("擦掉告示牌上的原版英文：")
    for n in ("m1.png", "m2.png", "m3.png", "m4.png"):
        clean(n)
    print("done")