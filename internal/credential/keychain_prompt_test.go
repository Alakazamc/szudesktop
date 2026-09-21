package credential

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// fakeKeychain 代替真实的 security 命令：记录每条命令的 argv 与标准输入，
// 用一个 map 扮演钥匙串。这样在没有 macOS 的机器上也能验证 F21 的核心性质——
// 密码只从标准输入进去，绝不出现在命令行参数里。
type fakeKeychain struct {
	items map[string]string
	calls []fakeCall
	// refuseAdd 模拟「这台机器上提示输入这条路不可用」。
	refuseAdd bool
	// storeHalf 模拟写进去的内容被截断，用来测读回校验。
	storeHalf bool
}

type fakeCall struct {
	args  []string
	stdin []byte
}

func itemKey(service, account string) string { return service + "/" + account }

// twoLines 把喂给 security 的标准输入拆成「密码」与「确认」两行。
func twoLines(stdin []byte) (string, string) {
	lines := strings.Split(strings.ReplaceAll(string(stdin), "\r\n", "\n"), "\n")
	second := ""
	if len(lines) > 1 {
		second = lines[1]
	}
	return lines[0], second
}

func newFakeKeychain(t *testing.T) *fakeKeychain {
	t.Helper()
	f := &fakeKeychain{items: map[string]string{}}
	saved := runSecurity
	runSecurity = f.run
	t.Cleanup(func() {
		runSecurity = saved
		promptMu.Lock()
		promptVerified = false
		promptMu.Unlock()
	})
	promptMu.Lock()
	promptVerified = false
	promptMu.Unlock()
	return f
}

func (f *fakeKeychain) run(args []string, stdin []byte) ([]byte, error) {
	f.calls = append(f.calls, fakeCall{args: append([]string(nil), args...), stdin: append([]byte(nil), stdin...)})
	valueOf := func(flag string) string {
		for i, a := range args {
			if a == flag && i+1 < len(args) {
				return args[i+1]
			}
		}
		return ""
	}
	id := itemKey(valueOf("-s"), valueOf("-a"))

	switch args[0] {
	case "add-generic-password":
		if f.refuseAdd {
			return nil, errors.New("security add-generic-password 失败: exit status 1（password not supplied）")
		}
		// -w 放最后、不带值，才表示「去提示里读」，也就是从标准输入读。
		if args[len(args)-1] != "-w" {
			return nil, errors.New("测试替身只接受 -w 放最后的写法，收到: " + strings.Join(args, " "))
		}
		// 真实的 security 会问两遍（password data for new item / retype password
		// for new item），只喂一行就得到 "passwords don't match"、条目根本建不起来。
		// 这是 CI 的 macOS 探针在真机上实测到的行为，替身照它建模。
		first, second := twoLines(stdin)
		if first == "" || first != second {
			return nil, errors.New("security add-generic-password 失败: exit status 44（passwords don't match）")
		}
		stored := first
		if f.storeHalf && len(stored) > 4 {
			stored = stored[:len(stored)/2]
		}
		f.items[id] = stored
		return nil, nil
	case "find-generic-password":
		v, ok := f.items[id]
		if !ok {
			return nil, errors.New("security find-generic-password 失败: exit status 44（SecKeychainSearchCopyNext: could not be found (-25300)）")
		}
		return []byte(v + "\n"), nil
	case "delete-generic-password":
		if _, ok := f.items[id]; !ok {
			return nil, errors.New("security delete-generic-password 失败: exit status 44（could not be found (-25300)）")
		}
		delete(f.items, id)
		return nil, nil
	}
	return nil, fmt.Errorf("测试替身不认识的子命令 %s", args[0])
}

// argvContains 检查某段机密是否出现在任何一次调用的命令行参数里。
func (f *fakeKeychain) argvContains(needle string) bool {
	for _, c := range f.calls {
		for _, a := range c.args {
			if strings.Contains(a, needle) {
				return true
			}
		}
	}
	return false
}

// countService 数钥匙串里还剩几个指定前缀的条目。
func (f *fakeKeychain) countService(prefix string) int {
	n := 0
	for id := range f.items {
		if strings.HasPrefix(id, prefix) {
			n++
		}
	}
	return n
}

// probeCalls 数一共做了几次自检（用 -s szunet-selftest-* 的命令行判断）。
func (f *fakeKeychain) probeCalls() int {
	n := 0
	for _, c := range f.calls {
		for i, a := range c.args {
			if a == "-s" && i+1 < len(c.args) && strings.HasPrefix(c.args[i+1], "szunet-selftest-") {
				n++
				break
			}
		}
	}
	return n
}

const realService = "szunet-test"
const realAccount = "szunet-test"

func TestKeychainSaveKeepsSecretOutOfArgv(t *testing.T) {
	f := newFakeKeychain(t)
	secret := []byte(`{"username":"000000","password":"test-only-secret"}`)

	if err := keychainSave(realService, realAccount, secret); err != nil {
		t.Fatalf("保存失败: %v", err)
	}
	if f.argvContains("test-only-secret") {
		t.Fatal("密码出现在了命令行参数里——同机其他进程 ps 就能看到，F21 又回来了")
	}
	if got := f.items[itemKey(realService, realAccount)]; got != string(secret) {
		t.Fatalf("钥匙串里存的不是我们给的值: %q", got)
	}
	if left := f.countService("szunet-selftest-"); left != 0 {
		t.Fatalf("自检条目用完了没清掉，会一直留在用户钥匙串里（剩 %d 个）", left)
	}
}

// 真实 macOS 的 `security -w` 会要求输入两遍（第二遍是确认）。只喂一遍会得到
// "passwords don't match"，条目根本建不起来——这是 CI 的 macOS 探针在真机上
// 实测到的行为，beta0.7.1 与 beta0.7.2 都因此存不了凭据。
func TestKeychainWriteFeedsSecretTwiceForConfirmation(t *testing.T) {
	f := newFakeKeychain(t)
	secret := []byte(`{"password":"test-only-secret"}`)
	if err := keychainSave(realService, realAccount, secret); err != nil {
		t.Fatalf("保存失败: %v", err)
	}

	var stdin []byte
	for _, c := range f.calls {
		if c.args[0] == "add-generic-password" && strings.Contains(strings.Join(c.args, " "), realService) {
			stdin = c.stdin
			break
		}
	}
	if stdin == nil {
		t.Fatal("没有对真实条目调用 add-generic-password")
	}
	first, second := twoLines(stdin)
	if first != string(secret) || second != string(secret) {
		t.Fatalf("标准输入必须是「密码 + 确认」两行且一致，实际是 %q 与 %q", first, second)
	}
	if f.argvContains("test-only-secret") {
		t.Fatal("密码出现在了命令行参数里，F21 又回来了")
	}
}

func TestKeychainSaveRefusesWhenPromptPathUnavailable(t *testing.T) {
	f := newFakeKeychain(t)
	f.refuseAdd = true

	err := keychainSave(realService, realAccount, []byte(`{"password":"test-only-secret"}`))
	if err == nil {
		t.Fatal("提示输入这条路不可用时必须报错，不能假装保存成功")
	}
	if !strings.Contains(err.Error(), "不可用") {
		t.Fatalf("错误信息没有说清原因: %v", err)
	}
	if _, ok := f.items[itemKey(realService, realAccount)]; ok {
		t.Fatal("自检都没过就把真实凭据写进去了")
	}
	if f.argvContains("test-only-secret") {
		t.Fatal("失败路径退回了命令行传密码")
	}
}

func TestKeychainSaveDetectsMismatchedReadback(t *testing.T) {
	f := newFakeKeychain(t)
	f.storeHalf = true

	err := keychainSave(realService, realAccount, []byte(`{"password":"test-only-secret"}`))
	if err == nil {
		t.Fatal("写进去的值和读回的不一致时必须报错，不能让用户以为存好了")
	}
	if !strings.Contains(err.Error(), "不一致") {
		t.Fatalf("错误信息没有说清原因: %v", err)
	}
}

func TestKeychainPromptSelfCheckIsCachedOnSuccessOnly(t *testing.T) {
	f := newFakeKeychain(t)
	if err := keychainSave(realService, realAccount, []byte("first")); err != nil {
		t.Fatalf("第一次保存失败: %v", err)
	}
	probesAfterFirst := f.probeCalls()
	if probesAfterFirst == 0 {
		t.Fatal("第一次保存应当先做一次自检")
	}
	if err := keychainSave(realService, realAccount, []byte("second")); err != nil {
		t.Fatalf("第二次保存失败: %v", err)
	}
	if extra := f.probeCalls(); extra != probesAfterFirst {
		t.Fatalf("自检成功过就应该复用结果，不该每条都探一次（%d → %d）", probesAfterFirst, extra)
	}

	// 失败不能缓存：用户解锁钥匙串后重试应当还能成功。
	f2 := newFakeKeychain(t)
	f2.refuseAdd = true
	if err := keychainSave(realService, realAccount, []byte("third")); err == nil {
		t.Fatal("这条本该失败")
	}
	f2.refuseAdd = false
	if err := keychainSave(realService, realAccount, []byte("third")); err != nil {
		t.Fatalf("上一次失败被缓存了，重试也失败: %v", err)
	}
}

func TestKeychainLoadAndDeleteDoNotTreatErrorsAsMissing(t *testing.T) {
	newFakeKeychain(t) // 换成假命令，本用例只验证「不存在」与「读不到」的区分
	if _, err := promptRead(realService, realAccount); !securityItemNotFound(err) {
		t.Fatalf("没存过时应认出「条目不存在」，得到: %v", err)
	}
	if err := keychainSave(realService, realAccount, []byte("value")); err != nil {
		t.Fatal(err)
	}
	if _, err := promptRead(realService, realAccount); err != nil {
		t.Fatalf("已保存的条目读不出来: %v", err)
	}
	if _, err := runSecurity([]string{"delete-generic-password", "-a", realAccount, "-s", realService}, nil); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	if _, err := promptRead(realService, realAccount); !securityItemNotFound(err) {
		t.Fatalf("删除后应报告条目不存在，得到: %v", err)
	}
}
