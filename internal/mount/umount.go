package mount

import (
    "fmt"
    "os"
    "os/exec"

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
