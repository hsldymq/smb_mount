package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-playground/validator/v10"
	"github.com/hsldymq/smb_mount/pkg/smb"
	"github.com/spf13/viper"
)

var validate *validator.Validate

func init() {
	validate = validator.New()
}

// Load 从指定路径加载配置
// 如果路径为空，则使用默认路径
func Load(path string) (*Config, error) {
	if path == "" {
		path = DefaultConfigPath()
	}

	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, &smb.ConfigError{Path: path, Err: fmt.Errorf("config file not found")}
	}

	// Create new viper instance
	v := viper.New()

	// Set config path and file
	v.SetConfigFile(path)

	// Read config file
	if err := v.ReadInConfig(); err != nil {
		return nil, &smb.ConfigError{Path: path, Err: fmt.Errorf("failed to read config: %w", err)}
	}

	// Unmarshal config
	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, &smb.ConfigError{Path: path, Err: fmt.Errorf("failed to parse config: %w", err)}
	}

	// Validate config
	if err := validate.Struct(cfg); err != nil {
		return nil, &smb.ConfigError{Path: path, Err: fmt.Errorf("config validation failed: %w", err)}
	}

	// Apply defaults and resolve paths
	if err := cfg.Normalize(); err != nil {
		return nil, &smb.ConfigError{Path: path, Err: fmt.Errorf("failed to normalize config: %w", err)}
	}

	return cfg, nil
}

// Normalize 应用默认值并解析路径
func (c *Config) Normalize() error {
	// Expand ~ in base_dir
	baseDir := c.BaseDir
	if len(baseDir) > 0 && baseDir[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to expand ~ in base_dir: %w", err)
		}
		baseDir = home + baseDir[1:]
	}

	// Convert to absolute path
	baseDir, err := filepath.Abs(baseDir)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path for base_dir: %w", err)
	}
	c.BaseDir = baseDir

	// Normalize each mount entry
	for i := range c.Mounts {
		if err := c.Mounts[i].Normalize(baseDir); err != nil {
			return err
		}
	}

	return nil
}

// Normalize 解析单个条目的挂载路径
func (m *MountEntry) Normalize(baseDir string) error {
	// Resolve mount path
	mountPath := m.MountDirPath
	if mountPath == "" {
		// Use mount_dir_name or name
		dirName := m.MountDirName
		if dirName == "" {
			dirName = m.Name
		}
		mountPath = filepath.Join(baseDir, dirName)
	} else {
		// Expand ~ if present
		if len(mountPath) > 0 && mountPath[0] == '~' {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("failed to expand ~ in mount_dir_path: %w", err)
			}
			mountPath = home + mountPath[1:]
		}

		// Convert to absolute path
		var err error
		mountPath, err = filepath.Abs(mountPath)
		if err != nil {
			return fmt.Errorf("failed to resolve absolute path for mount_dir_path: %w", err)
		}
	}

	m.ActualMountPath = mountPath
	m.mountPathResolved = true
	return nil
}

// ValidateBaseDir 检查 base_dir 是否存在或可以创建
func (c *Config) ValidateBaseDir() error {
	info, err := os.Stat(c.BaseDir)
	if os.IsNotExist(err) {
		// Base dir doesn't exist, that's OK - we'll create it when needed
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to access base_dir: %w", err)
	}

	// Check if it's actually a directory
	if !info.IsDir() {
		return fmt.Errorf("base_dir exists but is not a directory: %s", c.BaseDir)
	}

	return nil
}

// EnsureBaseDir 如果 base_dir 不存在则创建
func (c *Config) EnsureBaseDir() error {
	if err := os.MkdirAll(c.BaseDir, 0755); err != nil {
		return fmt.Errorf("failed to create base_dir: %w", err)
	}
	return nil
}

// FindByName 按名称查找挂载条目
func (c *Config) FindByName(name string) (*MountEntry, bool) {
	for i := range c.Mounts {
		if c.Mounts[i].Name == name {
			return &c.Mounts[i], true
		}
	}
	return nil, false
}

// GetMountEntryNames 返回所有挂载条目名称
func (c *Config) GetMountEntryNames() []string {
	names := make([]string, len(c.Mounts))
	for i, m := range c.Mounts {
		names[i] = m.Name
	}
	return names
}

// SetMountStatus 更新条目的挂载状态
func (c *Config) SetMountStatus(name string, isMounted bool) {
	for i := range c.Mounts {
		if c.Mounts[i].Name == name {
			c.Mounts[i].IsMounted = isMounted
			break
		}
	}
}

// CheckConfigPermissions 检查配置文件是否具有安全权限
func CheckConfigPermissions(path string) ([]string, []error) {
	var warnings []string
	var errors []error

	info, err := os.Stat(path)
	if err != nil {
		errors = append(errors, fmt.Errorf("failed to stat config file: %w", err))
		return warnings, errors
	}

	mode := info.Mode()

	// Check if world-readable
	if mode.Perm()&0004 != 0 {
		warnings = append(warnings, "Config file is world-readable. Consider chmod 600 for better security.")
	}

	// Check if group-readable (warn if not owner-only)
	if mode.Perm()&0040 != 0 {
		warnings = append(warnings, "Config file is group-readable. For best security, only the owner should read it.")
	}

	return warnings, errors
}

// DefaultConfigPath 返回默认配置文件路径
func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "smb_mount_config.yaml")
}
