package mount

import (
    "fmt"
    "github.com/hsldymq/smb_mount/internal/config"
    "github.com/hsldymq/smb_mount/pkg/smb"
    "os"
    "os/exec"
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
