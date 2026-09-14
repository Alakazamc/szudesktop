package crypto

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
)

// HMACMD5Hex 对应深澜登录请求里的 password 字段。
//
// 注意这里不是"先 MD5 再 HMAC"，也不是"拿密码当密钥"：
// 是把明文密码当数据、把服务端下发的 challenge 当密钥，做一次 HMAC-MD5。
// 这是最容易写反的一步。
func HMACMD5Hex(data, key string) string {
	mac := hmac.New(md5.New, []byte(key))
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}

// SHA1Hex 对应深澜登录请求里的 chksum 字段（小写十六进制）。
func SHA1Hex(content string) string {
	h := sha1.New()
	h.Write([]byte(content))
	return hex.EncodeToString(h.Sum(nil))
}
