package smb

import "fmt"

// MountError 挂载或卸载操作期间发生的错误
type MountError struct {
	Op   string // 操作类型："mount" 或 "umount"
	Path string
	Err  error
}

func (e *MountError) Error() string {
	return fmt.Sprintf("%s %s: %s", e.Op, e.Path, e.Err)
}

func (e *MountError) Unwrap() error {
	return e.Err
}

// ConfigError 配置加载或验证错误
type ConfigError struct {
	Path string
	Err  error
}

func (e *ConfigError) Error() string {
	return fmt.Sprintf("config error (%s): %s", e.Path, e.Err)
}

func (e *ConfigError) Unwrap() error {
	return e.Err
}
