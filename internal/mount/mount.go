package mount

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/hsldymq/smb_mount/internal/config"
	"github.com/hsldymq/smb_mount/pkg/smb"
	"github.com/moby/sys/mountinfo"
)

// Mount 对单个挂载条目执行挂载操作
func Mount(entry *config.MountEntry) error {
	// Check if already mounted
	mounted, err := CheckEntryStatus(entry)
	if err != nil {
		return fmt.Errorf("failed to check mount status: %w", err)
	}
	if mounted {
		return fmt.Errorf("already mounted at %s", entry.ActualMountPath)
	}

	// Create mount directory if it doesn't exist
	if err := os.MkdirAll(entry.ActualMountPath, 0755); err != nil {
		return &smb.MountError{Op: "mount", Path: entry.ActualMountPath, Err: fmt.Errorf("failed to create mount directory: %w", err)}
	}

	// Create credentials file
	credsFile, err := createCredentialFile(entry)
	if err != nil {
		return &smb.MountError{Op: "mount", Path: entry.ActualMountPath, Err: err}
	}
	defer os.Remove(credsFile)

	// Build mount command
	cmd := buildMountCommand(entry, credsFile)

	// Execute mount command
	output, err := cmd.CombinedOutput()
	if err != nil {
		return &smb.MountError{Op: "mount", Path: entry.ActualMountPath, Err: fmt.Errorf("mount failed: %w\nOutput: %s", err, string(output))}
	}

	return nil
}

// CreateCredentialFile 为外部使用创建临时凭据文件
func CreateCredentialFile(entry *config.MountEntry) (string, error) {
	return createCredentialFile(entry)
}

// createCredentialFile 为 mount.cifs 创建临时凭据文件
func createCredentialFile(entry *config.MountEntry) (string, error) {
	// Create a temp file with restricted permissions
	tmpFile, err := os.CreateTemp("", "smb_mount_creds_*.txt")
	if err != nil {
		return "", fmt.Errorf("failed to create credentials file: %w", err)
	}
	defer tmpFile.Close()

	// Set file permissions to owner-read only (0600)
	if err := tmpFile.Chmod(0600); err != nil {
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to set credentials file permissions: %w", err)
	}

	// Write credentials
	content := fmt.Sprintf("username=%s\npassword=%s\ndomain=%s\n",
		entry.Username,
		entry.Password,
		"", // Could add domain field later
	)

	if _, err := tmpFile.WriteString(content); err != nil {
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to write credentials: %w", err)
	}

	return tmpFile.Name(), nil
}

// BuildMountCommand 为外部使用构建 mount.cifs 命令
func BuildMountCommand(entry *config.MountEntry, credsFile string) *exec.Cmd {
	return buildMountCommand(entry, credsFile)
}

// buildMountCommand 构建 mount.cifs 命令
func buildMountCommand(entry *config.MountEntry, credsFile string) *exec.Cmd {
	// Build SMB address
	smbAddr := fmt.Sprintf("//%s:%d/%s", entry.SMBAddr, entry.GetSMBPort(), entry.ShareName)

	// Build mount options
	// Using common mount options for better compatibility
	options := fmt.Sprintf("credentials=%s,file_mode=0755,dir_mode=0755,uid=%d,gid=%d",
		credsFile,
		os.Getuid(),
		os.Getgid(),
	)

	// Build command: mount.cifs //server/share /mount/path -o options
	args := []string{
		smbAddr,
		entry.ActualMountPath,
		"-o", options,
	}

	return exec.Command("mount.cifs", args...)
}

// EnsureBaseDir 如果基础目录不存在则创建
func EnsureBaseDir(baseDir string) error {
	// Check if base dir exists
	info, err := os.Stat(baseDir)
	if os.IsNotExist(err) {
		// Create the directory
		if err := os.MkdirAll(baseDir, 0755); err != nil {
			return fmt.Errorf("failed to create base directory: %w", err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to access base directory: %w", err)
	}

	// Check if it's actually a directory
	if !info.IsDir() {
		return fmt.Errorf("base_dir exists but is not a directory: %s", baseDir)
	}

	return nil
}

// ValidateMountPoint 检查挂载点是否有效
func ValidateMountPoint(mountPath string) error {
	// Check if path exists
	info, err := os.Stat(mountPath)
	if os.IsNotExist(err) {
		// Doesn't exist - that's OK, we'll create it
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to access mount point: %w", err)
	}

	// Check if it's a directory
	if !info.IsDir() {
		return fmt.Errorf("mount point exists but is not a directory: %s", mountPath)
	}

	// Check if already mounted
	mounted, err := CheckStatus(mountPath)
	if err != nil {
		return fmt.Errorf("failed to check mount status: %w", err)
	}
	if mounted {
		return fmt.Errorf("already mounted: %s", mountPath)
	}

	return nil
}

// GetMountDevice 返回挂载点的设备/源
func GetMountDevice(mountPath string) (string, error) {
	info, err := GetMountInfo(mountPath)
	if err != nil {
		return "", err
	}
	return info.Source, nil
}

// IsMountPoint 返回路径是否为挂载点
func IsMountPoint(path string) (bool, error) {
	// First check if path exists
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	// Use mountinfo to check if it's a mount point
	mounted, err := mountinfo.Mounted(path)
	if err != nil {
		return false, err
	}

	return mounted, nil
}

// NormalizePath 解析并规范化文件路径
func NormalizePath(path string) (string, error) {
	// Expand ~
	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to expand ~: %w", err)
		}
		path = home + path[1:]
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	return absPath, nil
}
