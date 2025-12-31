package config

import "time"

// Config 主配置结构
type Config struct {
	BaseDir string       `yaml:"base_dir" mapstructure:"base_dir" validate:"required"`
	Mounts  []MountEntry `yaml:"mounts" mapstructure:"mounts" validate:"required,min=1"`
}

// MountEntry 单个 SMB 挂载配置
type MountEntry struct {
	Name         string `yaml:"name" mapstructure:"name" validate:"required"`
	SMBAddr      string `yaml:"smb_addr" mapstructure:"smb_addr" validate:"required"`
	SMBPort      int    `yaml:"smb_port" mapstructure:"smb_port" validate:"min=1,max=65535"`
	ShareName    string `yaml:"share_name" mapstructure:"share_name" validate:"required"`
	Username     string `yaml:"username" mapstructure:"username" validate:"required"`
	Password     string `yaml:"password" mapstructure:"password"`
	MountDirName string `yaml:"mount_dir_name" mapstructure:"mount_dir_name"`
	MountDirPath string `yaml:"mount_dir_path" mapstructure:"mount_dir_path"`

	// 运行时字段（不从配置加载）
	ActualMountPath   string `yaml:"-" mapstructure:"-"`
	IsMounted         bool   `yaml:"-" mapstructure:"-"`
	mountPathResolved bool   `yaml:"-" mapstructure:"-"`
}

// GetMountPath 返回此条目的实际挂载路径
// 优先级：mount_dir_path > base_dir + mount_dir_name > base_dir + name
func (m *MountEntry) GetMountPath(baseDir string) string {
	if m.mountPathResolved {
		return m.ActualMountPath
	}

	if m.MountDirPath != "" {
		m.ActualMountPath = m.MountDirPath
	} else {
		dirName := m.MountDirName
		if dirName == "" {
			dirName = m.Name
		}
		m.ActualMountPath = baseDir + "/" + dirName
	}
	m.mountPathResolved = true
	return m.ActualMountPath
}

// GetSMBPort 返回 SMB 端口，如果未设置则默认为 445
func (m *MountEntry) GetSMBPort() int {
	if m.SMBPort == 0 {
		return 445
	}
	return m.SMBPort
}

// HasPassword 返回是否配置了密码
func (m *MountEntry) HasPassword() bool {
	return m.Password != ""
}

// SMBAddress 返回带端口的完整 SMB 地址
func (m *MountEntry) SMBAddress() string {
	return m.SMBAddr
}

// CredentialSource 表示凭据来源
type CredentialSource struct {
	Username     string
	Password     string
	PasswordFrom string // "config", "env", "prompt" 等
}

// MountStatus 表示挂载点的状态
type MountStatus struct {
	Name      string
	MountPath string
	IsMounted bool
	MountedAt time.Time
	Device    string
	Options   []string
}
