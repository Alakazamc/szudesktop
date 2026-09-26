package ui

import (
	"crypto/aes"
	"encoding/base64"
	"strings"
	"testing"
)

// 黄金向量由学校登录页真实的 encrypt.js（CryptoJS）生成，并与 Node 原生
// crypto 的 AES-CBC 交叉核对过——两边逐字节一致，所以这些值钉住的是
// 「和浏览器同格式」，不是我们自己的假设。
func TestCasAesStringGoldenVectors(t *testing.T) {
	const iv = "0123456789abcdef"
	cases := []struct {
		name, key, data, want string
	}{
		// PKCS7：空数据也要补满一整块。
		{"empty", "1RKM2IpP3pRszFGS", "", "iShDnCL/9maankJxnxUP8A=="},
		{"one byte", "1RKM2IpP3pRszFGS", "a", "njiNsADq86BVMnCfRRRhWQ=="},
		// 刚好 15 字节，补 1 个字节到整块。
		{"15 bytes", "1RKM2IpP3pRszFGS", "0123456789abcde", "0RMNeBw7tTVRdLZ4O6B6lA=="},
		// 刚好 16 字节，PKCS7 仍要补一整块。
		{"exact block", "1RKM2IpP3pRszFGS", "0123456789abcdef", "LTZqp5ZY3PFk1Z9q1e+2gqoEdODWjB9OtAyq/A/yE5A="},
		// 多字节 UTF-8：CryptoJS.enc.Utf8.parse 与 []byte(s) 一致。
		{"chinese", "1RKM2IpP3pRszFGS", "中文密码测试", "4ul8iqPKNhNlDEMwESFZFUPrEkLCpga4fGY8D6/5dfQ="},
		// 真实形态：64 个随机字符前缀 + 密码。
		{"real shape", "1RKM2IpP3pRszFGS", strings.Repeat("X", 64) + "mypassword123", "RBYqOYUHUsCYFmtIuxxio8pXQBgDn06r6IroUbLUh5JV0GedgaWNqjdZR/KP5F++RtafjGU0IypF/2hcrKEDVZykGi7lk29tpZd9DwyOqAM="},
		// salt 长度决定 AES-128/192/256。
		{"aes-192", strings.Repeat("K", 24), strings.Repeat("X", 64) + "p", "7riiX9/x29+aq8CORscX61z64Ih5aBi0JELs3/JuN6AnOVOZ6AEHvNt3DGAYPjbEMIT4+4DHX2GG4scIZCJwSKrJjKlOhp4HE0otMs3yoW4="},
		{"aes-256", strings.Repeat("K", 32), strings.Repeat("X", 64) + "p", "AgoFGJidJWgabyYcwC0B/0YVymUXdv1bQlFQ2iRKIRM/YOCL/pGPIX43SIiZVCn4H7S27gp7+H+nTRM/m5Qj1Fg1V/LyAWOjxIVvNG8trwQ="},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := casAesString(c.data, c.key, iv)
			if err != nil {
				t.Fatalf("casAesString: %v", err)
			}
			if got != c.want {
				t.Fatalf("密文与浏览器不一致\n got: %s\nwant: %s", got, c.want)
			}
			// 长度自洽：PKCS7 后必然是块大小的整数倍。
			raw, err := base64.StdEncoding.DecodeString(got)
			if err != nil {
				t.Fatalf("输出不是合法 Base64: %v", err)
			}
			if len(raw)%aes.BlockSize != 0 {
				t.Fatalf("密文长度 %d 不是块的整数倍", len(raw))
			}
		})
	}
}

func TestCasAesStringRejectsBadInput(t *testing.T) {
	// 盐长度不对：服务端换了非标准的盐，必须明确失败而不是悄悄发出错东西。
	if _, err := casAesString("data", "short", "0123456789abcdef"); err == nil {
		t.Fatal("盐长度 5 字节应当报错")
	}
	// 22/26 字节这类「CryptoJS 会照单全收、标准 AES 不接受」的长度尤其要拦住。
	for _, n := range []int{1, 15, 17, 22, 26, 31, 33} {
		if _, err := casAesString("data", strings.Repeat("K", n), "0123456789abcdef"); err == nil {
			t.Fatalf("盐长度 %d 应当报错", n)
		}
	}
	if _, err := casAesString("data", "1RKM2IpP3pRszFGS", "tooshort"); err == nil {
		t.Fatal("iv 长度不对应当报错")
	}
}

func TestCasEncryptPassword(t *testing.T) {
	const salt = "1RKM2IpP3pRszFGS"
	first, err := casEncryptPassword("mypassword123", salt)
	if err != nil {
		t.Fatalf("casEncryptPassword: %v", err)
	}
	if first == "" {
		t.Fatal("加密结果不应为空")
	}
	// 随机前缀与随机 iv 让同一密码每次结果都不同——这是协议要求，不是 bug。
	second, err := casEncryptPassword("mypassword123", salt)
	if err != nil {
		t.Fatalf("casEncryptPassword: %v", err)
	}
	if first == second {
		t.Fatal("同一密码两次加密结果相同，随机前缀或 iv 没生效")
	}
	// 明文绝不应出现在密文里（Base64 之后更不可能）。
	if strings.Contains(first, "mypassword123") {
		t.Fatal("密文里出现了明文密码")
	}
	// 形态自洽：64 字节前缀 + 密码，PKCS7 后按块取整。
	raw, err := base64.StdEncoding.DecodeString(first)
	if err != nil {
		t.Fatalf("输出不是合法 Base64: %v", err)
	}
	if want := (64 + len("mypassword123") + aes.BlockSize) / aes.BlockSize * aes.BlockSize; len(raw) != want {
		t.Fatalf("密文长度 %d，期望 %d", len(raw), want)
	}
	// 空密码也要能产出合法报文（学校会自己报「密码不能为空」，不该在我们这里就崩）。
	empty, err := casEncryptPassword("", salt)
	if err != nil {
		t.Fatalf("空密码也应产出合法报文: %v", err)
	}
	if _, err := base64.StdEncoding.DecodeString(empty); err != nil {
		t.Fatalf("空密码的输出不是合法 Base64: %v", err)
	}
	// 盐不受支持时要把错误传出去，不能退回明文。
	if _, err := casEncryptPassword("p", "bad"); err == nil {
		t.Fatal("盐不受支持时必须报错，不能悄悄退回明文")
	}
}

func TestCasRandomStringAlphabet(t *testing.T) {
	// 字母表必须和 encrypt.js 完全一致：形近字符被刻意去掉了。
	for i := 0; i < 200; i++ {
		s, err := casRandomString(64)
		if err != nil {
			t.Fatalf("casRandomString: %v", err)
		}
		if len(s) != 64 {
			t.Fatalf("长度 %d，期望 64", len(s))
		}
		for _, r := range s {
			if !strings.ContainsRune(casAlphabet, r) {
				t.Fatalf("出现了字母表外的字符 %q", r)
			}
		}
	}
}
