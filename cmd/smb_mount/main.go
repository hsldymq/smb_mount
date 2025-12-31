package main

import (
	"fmt"
	"os"

	"github.com/hsldymq/smb_mount/internal/config"
	"github.com/hsldymq/smb_mount/internal/mount"
	"github.com/hsldymq/smb_mount/internal/privilege"
	"github.com/hsldymq/smb_mount/internal/prompt"
	"github.com/hsldymq/smb_mount/internal/tui"
	"github.com/spf13/cobra"
)

var (
	configPath string
)

var rootCmd = &cobra.Command{
	Use:   "smb_mount",
	Short: "便捷的 SMB/CIFS 挂载管理工具",
	Long: `smb_mount 是一个用于管理 Linux 系统 SMB/CIFS 共享的 CLI 工具。
它提供了挂载和卸载 SMB 共享的交互界面，
可配置选项存储在 YAML 文件中。`,
	Run: func(cmd *cobra.Command, args []string) {
		// 默认行为：显示帮助
		_ = cmd.Help()
	},
}

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"l"},
	Short:   "列出所有配置的挂载点",
	Long: `显示所有配置的 SMB 挂载条目及其当前挂载状态。
显示名称、SMB 地址、挂载路径以及每个共享是否已挂载。`,
	RunE: runList,
}

var mountCmd = &cobra.Command{
	Use:     "mount [name]",
	Aliases: []string{"m"},
	Short:   "挂载 SMB 共享",
	Long: `挂载 SMB 共享。如果提供了名称，则挂载该特定共享。
如果未提供名称，则显示交互式选择菜单。`,
	Args: cobra.MaximumNArgs(1),
	RunE: runMount,
}

var umountCmd = &cobra.Command{
	Use:     "umount [name]",
	Aliases: []string{"u", "unmount"},
	Short:   "卸载 SMB 共享",
	Long: `卸载 SMB 共享。如果提供了名称，则卸载该特定共享。
如果未提供名称，则显示已挂载共享的交互式选择菜单。`,
	Args: cobra.MaximumNArgs(1),
	RunE: runUmount,
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "",
		fmt.Sprintf("配置文件路径 (默认: %s)", config.DefaultConfigPath()))

	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(mountCmd)
	rootCmd.AddCommand(umountCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// loadConfig 从指定或默认路径加载配置
func loadConfig() (*config.Config, error) {
	path := configPath
	if path == "" {
		path = config.DefaultConfigPath()
	}

	cfg, err := config.Load(path)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Check config file permissions
	warnings, _ := config.CheckConfigPermissions(path)
	for _, w := range warnings {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", w)
	}

	return cfg, nil
}

// prepareMountEntry 准备挂载条目，如果需要则提示输入密码
func prepareMountEntry(entry *config.MountEntry) error {
	// If password is not in config, prompt for it
	if !entry.HasPassword() {
		fmt.Printf("Mounting: %s\n", entry.Name)
		fmt.Printf("SMB Address: %s:%d\n", entry.SMBAddr, entry.GetSMBPort())
		fmt.Printf("Username: %s\n", entry.Username)
		fmt.Println()

		password, err := prompt.PromptPassword("Enter password: ", true)
		if err != nil {
			return fmt.Errorf("failed to read password: %w", err)
		}
		entry.Password = password
	}

	return nil
}

// runList 实现列表命令
func runList(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	// Refresh mount status
	if err := mount.RefreshAllStatus(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to refresh mount status: %v\n", err)
	}

	// Display list using TUI
	if err := tui.DisplayList(cfg.Mounts); err != nil {
		return fmt.Errorf("failed to display list: %w", err)
	}

	return nil
}

// runMount 实现挂载命令
func runMount(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	// 首先刷新挂载状态
	if err := mount.RefreshAllStatus(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to refresh mount status: %v\n", err)
	}

	var entries []*config.MountEntry

	// 确定要挂载的条目
	if len(args) == 0 {
		// 交互式选择
		selected, cancelled := tui.SelectMountEntry(cfg.Mounts)
		if cancelled {
			fmt.Println("Cancelled")
			return nil
		}
		if selected == nil || len(selected) == 0 {
			fmt.Println("No shares selected")
			return nil
		}
		entries = selected
	} else {
		// 按名称查找
		name := args[0]
		entry, found := cfg.FindByName(name)
		if !found {
			return fmt.Errorf("mount entry '%s' not found", name)
		}
		entries = []*config.MountEntry{entry}
	}

	// 确保基础目录存在
	if err := mount.EnsureBaseDir(cfg.BaseDir); err != nil {
		return fmt.Errorf("failed to prepare base directory: %w", err)
	}

	// 批量挂载
	var successCount, failCount int
	fmt.Printf("Mounting %d share(s)...\n\n", len(entries))

	for i, entry := range entries {
		fmt.Printf("[%d/%d] %s\n", i+1, len(entries), entry.Name)

		// 检查是否已挂载
		if entry.IsMounted {
			fmt.Printf("  Already mounted at: %s\n\n", entry.ActualMountPath)
			successCount++
			continue
		}

		// 准备条目（如果需要则提示输入密码）
		if err := prepareMountEntry(entry); err != nil {
			fmt.Fprintf(os.Stderr, "  Failed to prepare: %v\n\n", err)
			failCount++
			continue
		}

		// 执行挂载
		fmt.Printf("  From: //%s:%d/%s\n", entry.SMBAddr, entry.GetSMBPort(), entry.ShareName)
		fmt.Printf("  To: %s\n", entry.ActualMountPath)

		if err := mount.Mount(entry); err != nil {
			// 检查是否需要使用 sudo 重试
			if privilege.NeedsPrivilege() {
				fmt.Println("  Privilege escalation required...")
				if err := mountWithSudo(entry); err != nil {
					fmt.Fprintf(os.Stderr, "  Failed: %v\n\n", err)
					failCount++
					continue
				}
			} else {
				fmt.Fprintf(os.Stderr, "  Failed: %v\n\n", err)
				failCount++
				continue
			}
		}

		fmt.Println("  Successfully mounted")
		successCount++
		fmt.Println()
	}

	// 汇总结果
	fmt.Println("==========================================")
	fmt.Printf("Mount complete: %d succeeded, %d failed\n", successCount, failCount)
	fmt.Println("==========================================")

	// 如果有任何失败，返回错误但不为 0（因为这是部分成功）
	if failCount > 0 && successCount == 0 {
		return fmt.Errorf("%d mount(s) failed", failCount)
	}

	return nil
}

// runUmount 实现卸载命令
func runUmount(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	// 首先刷新挂载状态
	if err := mount.RefreshAllStatus(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to refresh mount status: %v\n", err)
	}

	var entries []*config.MountEntry

	// 确定要卸载的条目
	if len(args) == 0 {
		// 交互式选择（只显示已挂载的条目）
		selected, cancelled := tui.SelectUnmountEntry(cfg.Mounts)
		if cancelled {
			fmt.Println("Cancelled")
			return nil
		}
		if selected == nil || len(selected) == 0 {
			fmt.Println("No mounted shares available")
			return nil
		}
		entries = selected
	} else {
		// 按名称查找
		name := args[0]
		entry, found := cfg.FindByName(name)
		if !found {
			return fmt.Errorf("mount entry '%s' not found", name)
		}

		// 检查是否已挂载
		if !entry.IsMounted {
			return fmt.Errorf("not mounted: %s", entry.Name)
		}
		entries = []*config.MountEntry{entry}
	}

	// 批量卸载
	var successCount, failCount int
	fmt.Printf("Unmounting %d share(s)...\n\n", len(entries))

	for i, entry := range entries {
		fmt.Printf("[%d/%d] %s\n", i+1, len(entries), entry.Name)
		fmt.Printf("  From: %s\n", entry.ActualMountPath)

		// 执行卸载
		if err := mount.Unmount(entry.ActualMountPath); err != nil {
			// 检查是否需要使用 sudo 重试
			if privilege.NeedsPrivilege() {
				fmt.Println("  Privilege escalation required...")
				if err := umountWithSudo(entry.ActualMountPath); err != nil {
					fmt.Fprintf(os.Stderr, "  Failed: %v\n\n", err)
					failCount++
					continue
				}
			} else {
				fmt.Fprintf(os.Stderr, "  Failed: %v\n\n", err)
				failCount++
				continue
			}
		}

		fmt.Println("  Successfully unmounted")
		successCount++
		fmt.Println()
	}

	// 汇总结果
	fmt.Println("==========================================")
	fmt.Printf("Unmount complete: %d succeeded, %d failed\n", successCount, failCount)
	fmt.Println("==========================================")

	// 如果有任何失败，返回错误但不为 0（因为这是部分成功）
	if failCount > 0 && successCount == 0 {
		return fmt.Errorf("%d unmount(s) failed", failCount)
	}

	return nil
}

// mountWithSudo 尝试使用权限提升进行挂载
func mountWithSudo(entry *config.MountEntry) error {
	// Create a temporary credentials file
	credsFile, err := mount.CreateCredentialFile(entry)
	if err != nil {
		return err
	}
	defer os.Remove(credsFile)

	// Build mount command
	cmd := mount.BuildMountCommand(entry, credsFile)

	// Execute with sudo
	if err := privilege.RunWithSudo(cmd); err != nil {
		return fmt.Errorf("mount with sudo failed: %w", err)
	}

	return nil
}

// umountWithSudo 尝试使用权限提升进行卸载
func umountWithSudo(mountPath string) error {
	cmd := mount.BuildUmountCommand(mountPath)

	if err := privilege.RunWithSudo(cmd); err != nil {
		return fmt.Errorf("unmount with sudo failed: %w", err)
	}

	return nil
}
