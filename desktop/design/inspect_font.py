import struct, os

D = r"D:\szuNet\desktop\assets\fonts"
for n in ["svbold.ttf", "svthin.ttf"]:
    p = os.path.join(D, n)
    d = open(p, "rb").read()
    ver = d[:4]
    num = struct.unpack(">H", d[4:6])[0]
    print(n, ver, "tables=", num)
    off = 12
    tables = {}
    for i in range(num):
        tag = d[off:off + 4].decode("latin1")
        o, ln = struct.unpack(">II", d[off + 8:off + 16])
        tables[tag] = (o, ln)
        off += 16
    print("  tables:", sorted(tables))
    if "maxp" in tables:
        o, _ = tables["maxp"]
        print("  numGlyphs=", struct.unpack(">H", d[o + 4:o + 6])[0])
    if "name" in tables:
        o, ln = tables["name"]
        cnt = struct.unpack(">H", d[o + 2:o + 4])[0]
        so = struct.unpack(">H", d[o + 4:o + 6])[0]
        for i in range(cnt):
            r = o + 6 + i * 12
            pid, eid, lid, nid, length, roff = struct.unpack(">HHHHHH", d[r:r + 12])
            if nid not in (1, 4, 6):
                continue
            raw = d[o + so + roff: o + so + roff + length]
            try:
                s = raw.decode("utf-16-be") if pid == 3 else raw.decode("latin1")
            except Exception:
                continue
            print("   name[%d]" % nid, repr(s[:70]))
    if "cmap" in tables:
        o, _ = tables["cmap"]
        ntab = struct.unpack(">H", d[o + 2:o + 4])[0]
        codes = set()
        for i in range(ntab):
            r = o + 4 + i * 8
            pid, eid, so2 = struct.unpack(">HHI", d[r:r + 8])
            sub = o + so2
            fmt = struct.unpack(">H", d[sub:sub + 2])[0]
            if fmt == 4:
                segx2 = struct.unpack(">H", d[sub + 6:sub + 8])[0]
                seg = segx2 // 2
                endo = sub + 14
                endc = [struct.unpack(">H", d[endo + i * 2:endo + i * 2 + 2])[0] for i in range(seg)]
                starto = endo + segx2 + 2
                startc = [struct.unpack(">H", d[starto + i * 2:starto + i * 2 + 2])[0] for i in range(seg)]
                for a, b in zip(startc, endc):
                    if a == 0xFFFF:
                        continue
                    codes.update(range(a, min(b, 0xFFFD) + 1))
            if len(codes) > 4000:
                break
        cs = sorted(codes)
        print("  mapped chars:", len(cs), "range", hex(cs[0]) if cs else "-", hex(cs[-1]) if cs else "-")
        printable = "".join(chr(c) for c in cs if 32 <= c < 127)
        print("  ASCII:", printable)
