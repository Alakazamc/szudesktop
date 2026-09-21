//go:build windows

package credential

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

// windowsStore 用 DPAPI 把凭据加密后再写文件。
//
// DPAPI 的密钥由 Windows 根据当前用户账户派生，换一台机器或换一个用户
// 都解不开。所以就算这个文件被拷走、被同步到网盘，里面的密码也读不出来。
type windowsStore struct {
	path string
}

func platformStore() Store {
	path, err := dataPath()
	if err != nil {
		// 连用户目录都定不下来时，不往当前目录写明文 credentials.json。
		return newUnavailableStore("无法确定用户目录")
	}
	return &windowsStore{path: path}
}

// platformSessionStore 的会话同样走 DPAPI。
// 会话（Cookie）能直接登进学校系统，泄露的危害不比密码小，
// 所以按一样的规格对待：加密落盘、换机器解不开。
func platformSessionStore() SessionStore {
	path, err := sessionPath()
	if err != nil {
		return &unavailableSessionStore{}
	}
	return &windowsSessionStore{path: path}
}

type windowsSessionStore struct {
	path string
}

func (s *windowsSessionStore) Save(v Session) error {
	plain, err := json.Marshal(v)
	if err != nil {
		return err
	}
	encrypted, err := dpapiProtect(plain)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}
	if err := os.WriteFile(s.path, encrypted, 0o600); err != nil {
		return fmt.Errorf("写入登录状态失败: %w", err)
	}
	return nil
}

func (s *windowsSessionStore) Load() (Session, error) {
	encrypted, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return Session{}, ErrSessionNotFound
		}
		return Session{}, err
	}
	plain, err := dpapiUnprotect(encrypted)
	if err != nil {
		return Session{}, err
	}
	var v Session
	if err := json.Unmarshal(plain, &v); err != nil {
		return Session{}, fmt.Errorf("登录状态内容格式不对: %w", err)
	}
	return v, nil
}

func (s *windowsSessionStore) Delete() error {
	err := os.Remove(s.path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (s *windowsSessionStore) Describe() string {
	return "Windows DPAPI（用当前用户账户加密，换机器或换用户都解不开）"
}

// dataBlob 对应 Windows 的 DATA_BLOB 结构。
type dataBlob struct {
	cbData uint32
	pbData *byte
}

var (
	crypt32DLL        = syscall.NewLazyDLL("crypt32.dll")
	kernel32DLL       = syscall.NewLazyDLL("kernel32.dll")
	procProtectData   = crypt32DLL.NewProc("CryptProtectData")
	procUnprotectData = crypt32DLL.NewProc("CryptUnprotectData")
	procLocalFree     = kernel32DLL.NewProc("LocalFree")
)

func blobFromBytes(b []byte) dataBlob {
	if len(b) == 0 {
		return dataBlob{}
	}
	return dataBlob{cbData: uint32(len(b)), pbData: &b[0]}
}

func (b dataBlob) toBytes() []byte {
	if b.pbData == nil || b.cbData == 0 {
		return nil
	}
	src := unsafe.Slice(b.pbData, b.cbData)
	out := make([]byte, len(src))
	copy(out, src)
	return out
}

// dpapiProtect 调 CryptProtectData 加密。
func dpapiProtect(plain []byte) ([]byte, error) {
	in := blobFromBytes(plain)
	var out dataBlob

	ret, _, err := procProtectData.Call(
		uintptr(unsafe.Pointer(&in)),
		0, // 描述文字，不需要
		0, // 附加熵，不需要
		0, // 保留
		0, // 提示结构，不需要
		0, // flags
		uintptr(unsafe.Pointer(&out)),
	)
	if ret == 0 {
		return nil, fmt.Errorf("CryptProtectData 调用失败: %v", err)
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(out.pbData)))

	return out.toBytes(), nil
}

// dpapiUnprotect 调 CryptUnprotectData 解密。
func dpapiUnprotect(encrypted []byte) ([]byte, error) {
	in := blobFromBytes(encrypted)
	var out dataBlob

	ret, _, err := procUnprotectData.Call(
		uintptr(unsafe.Pointer(&in)),
		0, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(&out)),
	)
	if ret == 0 {
		return nil, fmt.Errorf("CryptUnprotectData 调用失败（换过机器或换过用户账户时会这样）: %v", err)
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(out.pbData)))

	return out.toBytes(), nil
}

func (s *windowsStore) Save(c Credentials) error {
	plain, err := json.Marshal(c)
	if err != nil {
		return err
	}
	encrypted, err := dpapiProtect(plain)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}
	if err := os.WriteFile(s.path, encrypted, 0o600); err != nil {
		return fmt.Errorf("写入凭据失败: %w", err)
	}
	return nil
}

func (s *windowsStore) Load() (Credentials, error) {
	encrypted, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return Credentials{}, ErrNotFound
		}
		return Credentials{}, err
	}

	plain, err := dpapiUnprotect(encrypted)
	if err != nil {
		return Credentials{}, err
	}

	var c Credentials
	if err := json.Unmarshal(plain, &c); err != nil {
		return Credentials{}, fmt.Errorf("凭据内容格式不对: %w", err)
	}
	return c, nil
}

func (s *windowsStore) Delete() error {
	err := os.Remove(s.path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (s *windowsStore) Describe() string {
	return "Windows DPAPI（用当前用户账户加密，换机器或换用户都解不开）"
}
