package ui

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
)

// 深大统一身份认证（authserver.szu.edu.cn）登录口令的加密规则。
//
// 契约直接取自登录页 /authserver/cusSzuTheme/static/common/encrypt.js：
//
//	getAesString(data, salt, iv) = Base64(AES-CBC-PKCS7(key=salt, iv=iv, data))
//	encryptAES(pwd, salt)       = getAesString(randomString(64)+pwd, salt, randomString(16))
//
// salt 是登录页隐藏域 #pwdEncryptSalt 的值（它没有 name 属性，不随表单回传，
// 由服务端自己记住）；iv 是 16 个随机字符，正好一个 AES 块；明文前面垫 64 个
// 随机字符。输出只有 Base64 密文，不带 OpenSSL 的 "Salted__" 前缀——服务端如何
// 还原那个随机 iv 我们不需要知道，只要产出与浏览器同格式，服务端就会像对待
// 浏览器那样对待我们。
const casAlphabet = "ABCDEFGHJKMNPQRSTWXYZabcdefhijkmnprstwxyz2345678"

const (
	casPrefixLen = 64 // randomString(64)
	casIVLen     = 16 // randomString(16)，一个 AES 块
)

// casRandomString 复刻 encrypt.js 的 randomString：从去混淆字母表里取 n 个字符。
// 字母表刻意去掉了形近字符（I/L/O/U/V 与 0/1/9），必须原样照抄。
func casRandomString(n int) (string, error) {
	const limit = len(casAlphabet)
	// 拒绝采样：256 不是字母表长度的整数倍，直接取模会让排在前面的字符概率偏高。
	ceiling := 256 - 256%limit
	out := make([]byte, 0, n)
	buf := make([]byte, n)
	for len(out) < n {
		if _, err := rand.Read(buf); err != nil {
			return "", fmt.Errorf("读取随机数失败：%w", err)
		}
		for _, v := range buf {
			if int(v) >= ceiling {
				continue
			}
			out = append(out, casAlphabet[int(v)%limit])
			if len(out) == n {
				break
			}
		}
	}
	return string(out), nil
}

// casPkcs7Pad 实现 CryptoJS.pad.Pkcs7 的填充：按块大小补 n 个值为 n 的字节，
// 数据刚好整块时也会补一整块。
func casPkcs7Pad(data []byte, blockSize int) []byte {
	pad := blockSize - len(data)%blockSize
	out := make([]byte, len(data)+pad)
	copy(out, data)
	for i := len(data); i < len(out); i++ {
		out[i] = byte(pad)
	}
	return out
}

// casAesString 对应 getAesString(data, salt, iv)。key 与 iv 都按 UTF-8 字节使用，
// 与 CryptoJS.enc.Utf8.parse 一致；salt 长度决定 AES-128/192/256。
func casAesString(data, salt, iv string) (string, error) {
	key, vect := []byte(salt), []byte(iv)
	switch len(key) {
	case 16, 24, 32:
	default:
		return "", errors.New("学校返回的加密盐长度不受支持")
	}
	if len(vect) != aes.BlockSize {
		return "", errors.New("加密向量长度不正确")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	padded := casPkcs7Pad([]byte(data), block.BlockSize())
	dst := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, vect).CryptBlocks(dst, padded)
	return base64.StdEncoding.EncodeToString(dst), nil
}

// casEncryptPassword 对应 encryptPassword(pwd, salt)，也就是登录表单 password 字段的值。
// 每次调用都会换新的随机前缀与 iv，所以同一密码两次结果不同——这是预期的。
func casEncryptPassword(password, salt string) (string, error) {
	prefix, err := casRandomString(casPrefixLen)
	if err != nil {
		return "", err
	}
	iv, err := casRandomString(casIVLen)
	if err != nil {
		return "", err
	}
	return casAesString(prefix+password, salt, iv)
}
