// Package crypto 实现深澜（SRun）认证协议所需的加密原语。
//
// 深澜的密码和用户信息都不走明文，需要三道处理：
//  1. 密码用 HMAC-MD5 处理，密钥是服务端下发的 challenge
//  2. 用户信息用 XXTEA 的一个变体加密，再用一副打乱顺序的 Base64 字母表编码
//  3. 上面这些字段拼起来算一个 SHA1 校验和，供服务端确认请求没被改过
//
// xencode.go 里的 XXTEA 实现改写自 Sleepstars/SZU-login（MIT License，
// 原始版权归 E99p1ant），该实现又源自深澜门户页面自带的 JavaScript。
package crypto

import "encoding/base64"

// SrunAlphaSet 是深澜认证使用的自定义 Base64 字母表。
//
// 标准 Base64 按 A-Za-z0-9+/ 的顺序排，深澜把这 64 个字符的顺序打乱了。
// 用标准字母表编码出来的结果，服务端解不开，会直接报解密失败。
const SrunAlphaSet = "LVoJPiCN2R8G90yg+hmFHuacZ1OWMnrsSTXkYpUq/3dlbfKwv6xztjI7DeBE45QA"

// Base64WithAlphaSet 用指定字母表做 Base64 编码。
func Base64WithAlphaSet(data []byte, alphaSet string) string {
	return base64.NewEncoding(alphaSet).EncodeToString(data)
}
