package credential

import (
	"errors"
	"fmt"
	"os"
)

// ErrStorageUnavailable 表示这台机器没有可用的系统密钥环，凭据无法安全保存。
//
// 校园网密码不允许退回明文文件：明文会被备份、云同步和误提交带走，
// 而用户看到的却是「保存成功」——这比保存失败糟得多。业务会话早就是
// 这个标准（见 ErrSessionStorageUnavailable 与 unavailableSessionStore），
// 凭据跟它保持一致。
var ErrStorageUnavailable = errors.New("系统安全存储不可用，拒绝把校园网密码写成明文文件")

// unavailableStore 是没有系统密钥环时的凭据存储：一律拒绝，不落任何文件。
type unavailableStore struct{ reason string }

func newUnavailableStore(reason string) Store { return &unavailableStore{reason: reason} }

func (s *unavailableStore) Save(Credentials) error     { return s.err() }
func (s *unavailableStore) Load() (Credentials, error) { return Credentials{}, s.err() }

// Delete 清掉上一个版本可能留下的明文 credentials.json。
//
// 没有系统密钥环时，明文文件就是密码唯一可能存在的地方，所以这里可以如实
// 报成功；读和写仍然一律拒绝。修好新路的同时不能把旧坑留在用户盘上。
func (s *unavailableStore) Delete() error {
	path, err := dataPath()
	if err != nil {
		return nil // 连用户目录都定不下来，也就没有能定位的遗留文件
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除遗留的明文凭据文件失败: %w", err)
	}
	return nil
}

// Describe 会直接显示给用户（CLI 的「存放方式」、界面的 store_desc）。
func (s *unavailableStore) Describe() string {
	return "系统安全存储不可用（" + s.reason + "），不使用明文文件"
}

func (s *unavailableStore) err() error {
	return fmt.Errorf("%w：%s。请安装系统密钥环后重试，或每次登录时手动输入账号密码（不会保存）", ErrStorageUnavailable, s.reason)
}
