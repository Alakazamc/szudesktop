package version

import (
	"regexp"
	"testing"
)

// 版本号会直接显示在页面、关于页和发布包里，格式写错会被用户看见。
var versionPattern = regexp.MustCompile(`^[A-Za-z0-9]+(\.[A-Za-z0-9]+)*$`)

func TestCurrentIsUsable(t *testing.T) {
	if Current == "" {
		t.Fatal("版本号为空：VERSION 文件缺失或没有内容")
	}
	if !versionPattern.MatchString(Current) {
		t.Fatalf("版本号 %q 含空格或非法字符，会破坏页面与发布包文案", Current)
	}
}
