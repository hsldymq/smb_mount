package interaction

import (
    "fmt"
    "os"
    "os/exec"
    "os/user"
)

// IsRoot 检查当前进程是否以 root 身份运行
func IsRoot() bool {
    currentUser, err := user.Current()
    if err != nil {
        return false
    }
    return currentUser.Uid == "0"
}

// HasSudo 检查系统上是否可用 sudo
func HasSudo() bool {
    _, err := exec.LookPath("sudo")
    return err == nil
}

// NeedsPrivilege 返回当前操作是否需要权限提升
func NeedsPrivilege() bool {
    return !IsRoot()
}

// RunWithSudo 使用 sudo 执行命令
// 用 sudo 包装命令及其参数
func RunWithSudo(cmd *exec.Cmd) error {
    // If already root, just run the command directly
    if IsRoot() {
        return cmd.Run()
    }

    // Check if sudo is available
    if !HasSudo() {
        return fmt.Errorf("privilege escalation required but sudo is not available")
    }

    // Build sudo command
    // We use -S to read password from stdin if needed
    sudoArgs := []string{"-S", "--"}

    // Add the original command and its arguments
    sudoArgs = append(sudoArgs, cmd.Path)
    sudoArgs = append(sudoArgs, cmd.Args[1:]...)

    // Create new sudo command
    sudoCmd := exec.Command("sudo", sudoArgs...)

    // Set up stdin for password input
    sudoCmd.Stdin = os.Stdin
    sudoCmd.Stdout = cmd.Stdout
    sudoCmd.Stderr = cmd.Stderr

    // Run the sudo command
    if err := sudoCmd.Run(); err != nil {
        return fmt.Errorf("sudo command failed: %w", err)
    }

    return nil
}
