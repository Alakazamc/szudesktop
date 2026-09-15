"""把 PNG 拆开逐段看，定位浏览器不认的原因。

重点验三件事：
  1. zlib 解完之后的字节数，是不是正好等于 h * (1 + w * nch)
  2. 每行开头的 filter 类型是不是都在 0-4 范围内
  3. 如果 1 不成立，多出来/少了多少字节 —— 这能区分"行多算了一行"和"彻底乱套"

用法: python pngdeep.py <文件>
"""
import os
import struct
import sys
import zlib


def deep(path):
    d = open(path, "rb").read()
    print("== %s  %d 字节" % (os.path.basename(path), len(d)))
    pos = 8
    idat = b""
    w = h = bitd = colort = inter = None
    while pos < len(d):
        ln, = struct.unpack(">I", d[pos:pos + 4])
        typ = d[pos + 4:pos + 8]
        body = d[pos + 8:pos + 8 + ln]
        if typ == b"IHDR":
            w, h, bitd, colort, comp, filt, inter = struct.unpack(">IIBBBBB", body)
        elif typ == b"IDAT":
            idat += body
        pos += 12 + ln
    nch = {0: 1, 2: 3, 3: 1, 4: 2, 6: 4}[colort]
    print("   %dx%d 位深%d 颜色类型%d -> 每像素 %d 通道" % (w, h, bitd, colort, nch))

    try:
        raw = zlib.decompress(idat)
    except Exception as e:
        print("   !! zlib 解压失败: %s" % e)
        return

    expect = h * (1 + w * nch)
    print("   解压出 %d 字节，按 %d 行算应该是 %d 字节，差 %d"
          % (len(raw), h, expect, len(raw) - expect))

    # 逐行扫 filter 字节
    stride = 1 + w * nch
    bad = []
    for y in range(h):
        off = y * stride
        if off >= len(raw):
            break
        f = raw[off]
        if f > 4:
            bad.append((y, f))
    if bad:
        print("   !! 有 %d 行 filter 字节非法，前几个: %s"
              % (len(bad), ", ".join("行%d=0x%02X" % (y, f) for y, f in bad[:6])))
    else:
        print("   全部 %d 行 filter 字节合法" % min(h, len(raw) // stride))


if __name__ == "__main__":
    for f in sys.argv[1:]:
        deep(f)
        print()
