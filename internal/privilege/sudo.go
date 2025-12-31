package privilege

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
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

// RunWithSudoCombined 使用 sudo 执行命令并返回合并的输出
func RunWithSudoCombined(cmd *exec.Cmd) ([]byte, error) {
	// If already root, just run the command directly
	if IsRoot() {
		return cmd.CombinedOutput()
	}

	// Check if sudo is available
	if !HasSudo() {
		return nil, fmt.Errorf("privilege escalation required but sudo is not available")
	}

	// Build sudo command
	sudoArgs := []string{"-S", "--"}
	sudoArgs = append(sudoArgs, cmd.Path)
	sudoArgs = append(sudoArgs, cmd.Args[1:]...)

	sudoCmd := exec.Command("sudo", sudoArgs...)

	// Capture output
	var stdout, stderr bytes.Buffer
	sudoCmd.Stdin = os.Stdin
	sudoCmd.Stdout = &stdout
	sudoCmd.Stderr = &stderr

	// Run the command
	if err := sudoCmd.Run(); err != nil {
		// Combine stdout and stderr
		output := append(stdout.Bytes(), stderr.Bytes()...)
		return output, fmt.Errorf("sudo command failed: %w", err)
	}

	// Combine stdout and stderr
	output := append(stdout.Bytes(), stderr.Bytes()...)
	return output, nil
}

// CanSudo 检查当前用户是否可以在不提示输入密码的情况下运行 sudo
// 或者是否拥有 sudo 权限
func CanSudo() bool {
	if IsRoot() {
		return true
	}

	if !HasSudo() {
		return false
	}

	// Run sudo with -n (non-interactive) and -l (list) to check permissions
	cmd := exec.Command("sudo", "-n", "-l")
	err := cmd.Run()

	// If exit code is 0, we can sudo without password
	// If exit code is 1, we need password but have sudo access
	// Other errors mean no sudo access
	return err == nil || cmd.ProcessState.ExitCode() == 1
}

// SudoArgs 包装命令参数以供 sudo 使用
func SudoArgs(args []string) []string {
	wrapped := []string{"--"}
	wrapped = append(wrapped, args...)
	return wrapped
}

// ValidateSudoPermissions 检查用户是否具有挂载操作所需的 sudo 权限
func ValidateSudoPermissions() error {
	if IsRoot() {
		return nil
	}

	if !HasSudo() {
		return fmt.Errorf("sudo is required for mount operations but is not available")
	}

	// Try to validate we can run mount.cifs with sudo
	cmd := exec.Command("sudo", "-n", "mount.cifs")
	if err := cmd.Run(); err != nil {
		// mount.cifs without arguments fails, but we're checking if we have permission
		// If the error is about arguments, we have permission
		if strings.Contains(err.Error(), "usage") || strings.Contains(err.Error(), "invalid") {
			return nil
		}
		// If sudo asks for password, that's OK - user will be prompted
		if cmd.ProcessState != nil && cmd.ProcessState.ExitCode() == 1 {
			return nil
		}
	}

	return nil
}

// GetCurrentUser 返回当前用户
func GetCurrentUser() (*user.User, error) {
	return user.Current()
}

// GetCurrentUserUID 返回当前用户的 UID
func GetCurrentUserUID() int {
	currentUser, err := user.Current()
	if err != nil {
		return 0
	}
	uid, _ := strconv.Atoi(currentUser.Uid)
	return uid
}

// GetCurrentUserGID 返回当前用户的 GID
func GetCurrentUserGID() int {
	currentUser, err := user.Current()
	if err != nil {
		return 0
	}
	gid, _ := strconv.Atoi(currentUser.Gid)
	return gid
}

// EnsureMountCommand 检查是否可以执行挂载命令
func EnsureMountCommand() error {
	// Check if mount.cifs exists
	_, err := exec.LookPath("mount.cifs")
	if err != nil {
		return fmt.Errorf("mount.cifs not found. Please install cifs-utils: %w", err)
	}

	// Check if umount exists
	_, err = exec.LookPath("umount")
	if err != nil {
		return fmt.Errorf("umount command not found: %w", err)
	}

	return nil
}
