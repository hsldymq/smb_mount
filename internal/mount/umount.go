package mount

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/hsldymq/smb_mount/internal/config"
	"github.com/hsldymq/smb_mount/pkg/smb"
)

// BuildUmountCommand 为外部使用构建 umount 命令
func BuildUmountCommand(mountPath string) *exec.Cmd {
	return exec.Command("umount", mountPath)
}

// Unmount 卸载已挂载的 SMB 共享
func Unmount(mountPath string) error {
	// Check if mounted
	mounted, err := CheckStatus(mountPath)
	if err != nil {
		return &smb.MountError{Op: "umount", Path: mountPath, Err: fmt.Errorf("failed to check mount status: %w", err)}
	}
	if !mounted {
		return &smb.MountError{Op: "umount", Path: mountPath, Err: fmt.Errorf("not mounted")}
	}

	// Build umount command
	cmd := exec.Command("umount", mountPath)

	// Execute umount command
	output, err := cmd.CombinedOutput()
	if err != nil {
		return &smb.MountError{Op: "umount", Path: mountPath, Err: fmt.Errorf("umount failed: %w\nOutput: %s", err, string(output))}
	}

	return nil
}

// UnmountEntry 卸载挂载条目
func UnmountEntry(entry *config.MountEntry) error {
	return Unmount(entry.ActualMountPath)
}

// ForceUnmount 尝试强制卸载挂载点
// 使用 -l (lazy) 标志，即使有打开的文件也能卸载文件系统
// 并在可能的情况下清理引用
func ForceUnmount(mountPath string) error {
	// Check if mounted
	mounted, err := CheckStatus(mountPath)
	if err != nil {
		return &smb.MountError{Op: "umount", Path: mountPath, Err: fmt.Errorf("failed to check mount status: %w", err)}
	}
	if !mounted {
		return &smb.MountError{Op: "umount", Path: mountPath, Err: fmt.Errorf("not mounted")}
	}

	// Try lazy unmount first
	cmd := exec.Command("umount", "-l", mountPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Lazy unmount failed, try force unmount
		cmd = exec.Command("umount", "-f", mountPath)
		output, err = cmd.CombinedOutput()
		if err != nil {
			return &smb.MountError{Op: "umount", Path: mountPath, Err: fmt.Errorf("force unmount failed: %w\nOutput: %s", err, string(output))}
		}
	}

	return nil
}

// UnmountAll 卸载配置中所有已挂载的共享
func UnmountAll(cfg *config.Config) error {
	var lastErr error

	for i := range cfg.Mounts {
		if cfg.Mounts[i].IsMounted {
			if err := UnmountEntry(&cfg.Mounts[i]); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to unmount %s: %v\n", cfg.Mounts[i].Name, err)
				lastErr = err
			} else {
				cfg.Mounts[i].IsMounted = false
				fmt.Printf("Unmounted: %s\n", cfg.Mounts[i].Name)
			}
		}
	}

	return lastErr
}

// CleanupMountPoint 如果挂载点目录为空则删除
func CleanupMountPoint(mountPath string) error {
	// Check if mounted
	mounted, err := CheckStatus(mountPath)
	if err != nil {
		return err
	}
	if mounted {
		return fmt.Errorf("cannot cleanup: still mounted")
	}

	// Check if directory exists
	if _, err := os.Stat(mountPath); os.IsNotExist(err) {
		return nil // Already gone
	}

	// Remove the directory
	if err := os.Remove(mountPath); err != nil {
		return fmt.Errorf("failed to remove mount point: %w", err)
	}

	return nil
}

// UnmountAndCleanup 卸载共享并在挂载点为空时删除
func UnmountAndCleanup(mountPath string) error {
	if err := Unmount(mountPath); err != nil {
		return err
	}

	// Attempt to clean up the mount point
	// Ignore errors - the directory might not be empty
	_ = CleanupMountPoint(mountPath)

	return nil
}
