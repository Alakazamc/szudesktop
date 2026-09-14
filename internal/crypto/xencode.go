package crypto

import "math"

// 本文件实现深澜门户页面里那个叫 xEncode 的 JavaScript 函数。
// 它是 XXTEA 的一个改动版：加密轮数固定为 6 + 52/(n+1)，delta 取 0x9E3779B9。
// 改动版实现改写自 Sleepstars/SZU-login（MIT License，原始版权 E99p1ant）。

// ordAt 取字符串第 idx 个字节的值，越界返回 0。
func ordAt(msg string, idx int) uint32 {
	if len(msg) > idx {
		return uint32(msg[idx])
	}
	return 0
}

// sensCode 把字符串按每 4 字节打包成一个 uint32（小端）。
// key 为真时把原始长度追加到末尾，解密时要用它判断是否解对。
func sensCode(content string, key bool) []uint32 {
	l := len(content)
	pwd := make([]uint32, 0, l/4+2)
	for i := 0; i < l; i += 4 {
		pwd = append(pwd,
			ordAt(content, i)|
				ordAt(content, i+1)<<8|
				ordAt(content, i+2)<<16|
				ordAt(content, i+3)<<24,
		)
	}
	if key {
		pwd = append(pwd, uint32(l))
	}
	return pwd
}

// lenCode 把 uint32 切片还原成字节序列（小端）。
// key 为真时按存在末尾的原始长度截断，长度不合理则返回 nil。
func lenCode(msg []uint32, key bool) []byte {
	l := uint32(len(msg))
	ll := (l - 1) << 2
	if key {
		m := msg[l-1]
		if m < ll-3 || m > ll {
			return nil
		}
		ll = m
	}

	t := make([]byte, 0, len(msg)*4)
	for i := range msg {
		t = append(t,
			byte(msg[i]&0xff),
			byte(msg[i]>>8&0xff),
			byte(msg[i]>>16&0xff),
			byte(msg[i]>>24&0xff),
		)
	}
	if key {
		return t[0:ll]
	}
	return t
}

// Encode 对应深澜门户里的 xEncode(content, key)。
// content 是要加密的明文，key 是服务端下发的 challenge。
// 返回值是加密后的原始字节（未做 Base64）。
func Encode(content, key string) string {
	if content == "" {
		return ""
	}

	pwd := sensCode(content, true)
	pwdk := sensCode(key, false)
	for len(pwdk) < 4 {
		pwdk = append(pwdk, 0)
	}

	n := uint32(len(pwd) - 1)
	z := pwd[n]
	y := pwd[0]
	var c uint32 = 0x86014019 | 0x183639A0
	var m, e, p uint32
	q := math.Floor(6 + 52/(float64(n)+1))
	var d uint32

	for q > 0 {
		d = d + c&(0x8CE0D9BF|0x731F2640)
		e = d >> 2 & 3

		p = 0
		for p < n {
			y = pwd[p+1]
			m = z>>5 ^ y<<2
			m = m + ((y>>3 ^ z<<4) ^ (d ^ y))
			m = m + (pwdk[(p&3)^e] ^ z)
			pwd[p] = pwd[p] + m&(0xEFB8D130|0x10472ECF)
			z = pwd[p]
			p = p + 1
		}

		y = pwd[0]
		m = z>>5 ^ y<<2
		m = m + ((y>>3 ^ z<<4) ^ (d ^ y))
		m = m + (pwdk[(p&3)^e] ^ z)
		pwd[n] = pwd[n] + m&(0xBB390742|0x44C6F8BD)
		z = pwd[n]

		q = q - 1
	}

	return string(lenCode(pwd, false))
}
