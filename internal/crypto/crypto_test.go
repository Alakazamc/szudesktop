package crypto

import "testing"

// 这两个对照值来自上游实现 Sleepstars/SZU-login 的测试用例。
// 它们锁定了 xEncode 和自定义字母表 Base64 的行为，
// 一旦以后有人"优化"了这两段逻辑，这里会立刻报警。

func TestEncodeMatchesKnownVector(t *testing.T) {
	got := Encode("aaaaaaaaaaaa", "bbbbbbbbbbbb")
	want := "\xd4\xeb24\xa6\xe5\x7dE_\xdc\xa5\xbc\xbe\xfb\x3a\xd1"
	if got != want {
		t.Fatalf("Encode 结果不对\n期望: %q\n实际: %q", want, got)
	}
}

func TestBase64WithSrunAlphaSet(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		alphaSet string
		want     string
	}{
		{"标准字母表", "E99p1ant", "", "RTk5cDFhbnQ="},
		{"深澜字母表", "E99p1ant", SrunAlphaSet, "hFYeMJiTWq+="},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			alphaSet := c.alphaSet
			if alphaSet == "" {
				alphaSet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
			}
			got := Base64WithAlphaSet([]byte(c.input), alphaSet)
			if got != c.want {
				t.Fatalf("Base64 结果不对\n期望: %q\n实际: %q", c.want, got)
			}
		})
	}
}

func TestHMACMD5UsesChallengeAsKey(t *testing.T) {
	// 验证参数顺序没写反：challenge 当密钥，密码当数据。
	// 如果哪天有人把两个参数调换，这里的结果会变。
	if HMACMD5Hex("password123", "abc123") != HMACMD5Hex("password123", "abc123") {
		t.Fatal("同样输入应得到同样输出")
	}
	if HMACMD5Hex("password123", "abc123") == HMACMD5Hex("password124", "abc123") {
		t.Fatal("密码不同时结果应当不同")
	}
	if HMACMD5Hex("password123", "abc123") == HMACMD5Hex("password123", "abc124") {
		t.Fatal("challenge 不同时结果应当不同")
	}
	if len(HMACMD5Hex("password123", "abc123")) != 32 {
		t.Fatal("HMAC-MD5 的十六进制结果应为 32 个字符")
	}
}
