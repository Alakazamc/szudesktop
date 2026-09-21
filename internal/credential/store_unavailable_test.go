package credential

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 没有系统密钥环时，校园网密码不允许退回明文文件。
//
// 明文文件会被备份、云同步和误提交带走，而用户看到的却是「保存成功」——
// 这比保存失败糟得多。业务会话早就是这个标准（unavailableSessionStore），
// 凭据必须跟上。
func TestUnavailableStoreRefusesToSavePassword(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SZUNET_CONFIG_DIR", dir)

	s := newUnavailableStore("本机没有 Secret Service")

	err := s.Save(Credentials{Username: "2026123456", Password: "hunter2"})
	if err == nil {
		t.Fatal("没有密钥环时 Save 竟然报成功")
	}
	if !errors.Is(err, ErrStorageUnavailable) {
		t.Fatalf("调用方需要能识别这个错误并给出对应提示，得到 %v", err)
	}
	if !strings.Contains(err.Error(), "本机没有 Secret Service") {
		t.Fatalf("错误没有带上具体原因：%v", err)
	}

	entries, readErr := os.ReadDir(dir)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if len(entries) != 0 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("密码被写进了配置目录：%v", names)
	}

	if _, loadErr := s.Load(); !errors.Is(loadErr, ErrStorageUnavailable) {
		t.Fatalf("读不出来的原因必须如实说明，得到 %v", loadErr)
	}
	if !strings.Contains(s.Describe(), "不可用") {
		t.Fatalf("Describe 会显示给用户，必须如实说明存储不可用，得到 %q", s.Describe())
	}
	if strings.Contains(s.Describe(), "明文文件") && !strings.Contains(s.Describe(), "不使用") {
		t.Fatalf("Describe 不能让人误以为密码写成了明文：%q", s.Describe())
	}
}

// 「忘掉账号」必须把上一个版本可能留下的明文密码文件一起清掉。
//
// 现在拒绝写明文了，但不能把老用户盘上那个 credentials.json 留着不管——
// 那等于修好了新路、旧坑还在。删除本身可以如实报成功：没有密钥环时，
// 明文文件就是密码唯一可能存在的地方。
func TestUnavailableStoreDeleteRemovesLegacyPlaintextFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SZUNET_CONFIG_DIR", dir)

	legacy := filepath.Join(dir, "credentials.json")
	if err := os.WriteFile(legacy, []byte(`{"username":"2026123456","password":"hunter2"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	s := newUnavailableStore("本机没有 Secret Service")
	if err := s.Delete(); err != nil {
		t.Fatalf("删除不该失败：%v", err)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatal("上一个版本留下的明文密码文件还在盘上")
	}

	// 没有遗留文件时，删除是空操作，同样不该报错。
	if err := newUnavailableStore("本机没有 Secret Service").Delete(); err != nil {
		t.Fatalf("没有东西可删时不该报错：%v", err)
	}
}
