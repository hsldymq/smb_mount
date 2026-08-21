package main

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"log/syslog"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/hsldymq/gomount/internal/config"
	"github.com/hsldymq/gomount/internal/daemon"
	"github.com/hsldymq/gomount/internal/daemonapi"
	"github.com/hsldymq/gomount/internal/drivers"
	smbDriver "github.com/hsldymq/gomount/internal/drivers/smb"
	sshfsDriver "github.com/hsldymq/gomount/internal/drivers/sshfs"
	"github.com/hsldymq/gomount/internal/interaction"
	"github.com/hsldymq/gomount/internal/tui"
	rclonelog "github.com/rclone/rclone/fs/log"
	"github.com/spf13/cobra"
)

var (
	//go:embed usage.tmpl
	usageTemplate string

	configPath string

	daemonLogTarget      string
	daemonLogFile        string
	daemonSocketPathFlag string
)

var rootCmd = &cobra.Command{
	Use:           "gomount",
	Short:         "便捷的挂载管理工具",
	SilenceUsage:  true,
	SilenceErrors: true,
	Long: `gomount 是一个管理多种网络共享挂载的 CLI 工具。
它提供了挂载和卸载网络共享的交互界面。`,
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"l"},
	Short:   "列出所有配置的挂载点",
	Long: `显示所有配置的挂载条目及其当前挂载状态。
显示名称、来源地址、挂载路径以及每个共享是否已挂载。`,
	RunE: runList,
}

var interactiveCmd = &cobra.Command{
	Use:     "interactive",
	Aliases: []string{"i"},
	Short:   "交互式挂载管理",
	Long: `显示交互式选择菜单，可以选择要挂载或卸载的共享。
这是默认的交互式操作模式。`,
	RunE: runInteractive,
}

var mountCmd = &cobra.Command{
	Use:     "mount [name...]",
	Aliases: []string{"m"},
	Short:   "挂载指定共享",
	Long:    `挂载指定名称的共享。可指定多个名称依次挂载。`,
	Args:    cobra.MinimumNArgs(0),
	RunE:    runMount,
}

var umountCmd = &cobra.Command{
	Use:     "unmount [name...]",
	Aliases: []string{"u"},
	Short:   "卸载指定共享",
	Long:    `卸载指定名称的已挂载共享。可指定多个名称依次卸载。`,
	Args:    cobra.MinimumNArgs(0),
	RunE:    runUmount,
}

var daemonCmd = &cobra.Command{
	Use:     "daemon",
	Aliases: []string{"d"},
	Short:   "管理 gomount daemon",
}

var daemonRunCmd = &cobra.Command{
	Use:    "run",
	Hidden: true,
	RunE:   runDaemon,
}

var daemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "显示 daemon 状态",
	RunE:  runDaemonStatus,
}

var daemonDownCmd = &cobra.Command{
	Use:   "down",
	Short: "停止 daemon",
	RunE:  runDaemonDown,
}

var mkdirDryRun bool

var mkdirCmd = &cobra.Command{
	Use:     "mkdir [name...]",
	Aliases: []string{"r"},
	Short:   "创建挂载目录",
	Long: `根据配置文件创建所有挂载条目的目录。
可指定名称创建，或不指定/使用 "*" 创建全部。
使用 --dry-run 仅预览操作。`,
	Args: cobra.MinimumNArgs(0),
	RunE: runMkdir,
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "",
		fmt.Sprintf("配置文件路径 (默认: %s)", config.DefaultConfigPath()))
	daemonRunCmd.Flags().StringVar(&daemonLogTarget, "log-target", "syslog", "daemon log target: syslog, file, or stderr")
	daemonRunCmd.Flags().StringVar(&daemonLogFile, "log-file", "", "daemon log file path when --log-target=file")
	daemonRunCmd.Flags().StringVar(&daemonSocketPathFlag, "socket-path", "", "daemon unix socket path")

	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(interactiveCmd)
	rootCmd.AddCommand(mountCmd)
	rootCmd.AddCommand(umountCmd)
	daemonCmd.AddCommand(daemonRunCmd)
	daemonCmd.AddCommand(daemonStatusCmd)
	daemonCmd.AddCommand(daemonDownCmd)
	rootCmd.AddCommand(daemonCmd)
	mkdirCmd.Flags().BoolVar(&mkdirDryRun, "dry-run", false, "仅预览将要执行的操作")
	rootCmd.AddCommand(mkdirCmd)

	cmdNameFunc := func(name string, aliases []string) string {
		if len(aliases) == 0 {
			return name
		}
		return name + " (" + strings.Join(aliases, ", ") + ")"
	}
	cobra.AddTemplateFunc("cmdName", cmdNameFunc)
	cobra.AddTemplateFunc("cmdNamePadding", func(cmd *cobra.Command) int {
		maxLen := 0
		for _, c := range cmd.Commands() {
			if !c.IsAvailableCommand() {
				continue
			}
			l := len(cmdNameFunc(c.Name(), c.Aliases))
			if l > maxLen {
				maxLen = l
			}
		}
		return maxLen
	})
	rootCmd.SetUsageTemplate(usageTemplate)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func createDriverManager(cfg *config.Config) *drivers.Manager {
	mgr := drivers.NewManager(cfg)
	mgr.RegisterDriver(smbDriver.NewDriver())
	mgr.RegisterDriver(sshfsDriver.NewDriver())
	return mgr
}

func runList(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return err
	}

	mgr := createDriverManager(cfg)
	if err := refreshAllStatus(context.Background(), mgr, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to refresh mount status: %v\n", err)
	}

	if err := tui.DisplayList(cfg.Mounts); err != nil {
		return fmt.Errorf("failed to display list: %w", err)
	}

	return nil
}

func runDaemon(cmd *cobra.Command, args []string) error {
	cleanupLogs, err := configureDaemonLogging(daemonLogTarget, daemonLogFile, os.Stderr)
	if err != nil {
		return err
	}
	defer cleanupLogs()

	socketPath, err := daemonSocketPathFromValue(daemonSocketPathFlag)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()
	server := daemon.NewServerWithShutdown(daemon.NewSessionManager(nil), cancel)
	err = daemon.Run(ctx, socketPath, server)
	if err != nil {
		logDaemonLine("ERROR : daemon stopped: " + err.Error())
	}
	return err
}

func runDaemonStatus(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return err
	}
	socketPath, err := daemonSocketPath(cfg)
	if err != nil {
		return err
	}
	client := daemon.NewClient(socketPath)
	health, err := client.Health(cmd.Context())
	if err != nil {
		fmt.Fprintln(cmd.OutOrStdout(), "daemon: not running")
		fmt.Fprintf(cmd.OutOrStdout(), "socket: %s\n", socketPath)
		return nil
	}
	fmt.Fprintln(cmd.OutOrStdout(), "daemon: running")
	fmt.Fprintf(cmd.OutOrStdout(), "pid: %d\n", health.PID)
	fmt.Fprintf(cmd.OutOrStdout(), "socket: %s\n", socketPath)
	fmt.Fprintf(cmd.OutOrStdout(), "managed_types: %s\n", strings.Join(health.ManagedTypes, ","))
	fmt.Fprintf(cmd.OutOrStdout(), "mounted_sessions: %d\n", health.MountedSessions)
	return nil
}

func runDaemonDown(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return err
	}
	socketPath, err := daemonSocketPath(cfg)
	if err != nil {
		return err
	}
	client := daemon.NewClient(socketPath)
	if _, err := client.Health(cmd.Context()); err != nil {
		fmt.Fprintln(cmd.OutOrStdout(), "daemon: not running")
		return nil
	}
	fmt.Fprintln(cmd.OutOrStdout(), "daemon: stopping")
	resp, err := client.Shutdown(cmd.Context())
	if err != nil {
		return err
	}
	if !resp.OK {
		for _, shutdownErr := range resp.Errors {
			fmt.Fprintf(cmd.ErrOrStderr(), "ERROR: failed to unmount %s: %s\n", shutdownErr.Name, shutdownErr.Message)
		}
		return fmt.Errorf("daemon shutdown failed")
	}
	fmt.Fprintln(cmd.OutOrStdout(), "daemon: stopped")
	return nil
}

func mountEntries(ctx context.Context, mgr *drivers.Manager, entries []*config.MountEntry) (success, fail int) {
	if len(entries) == 0 {
		return 0, 0
	}

	fmt.Printf("Mounting %d share(s)...\n", len(entries))

	needSudo, err := entriesNeedSudo(mgr, entries)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to inspect mount privileges: %v\n", err)
	} else if needSudo {
		if err := interaction.EnsureSudoCached(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: sudo authentication failed: %v\n", err)
		}
	}

	for i, entry := range entries {
		fmt.Printf("  [%d/%d] %s\n", i+1, len(entries), entry.Name)

		if entry.IsMounted {
			fmt.Printf("    Already mounted at: %s\n\n", entry.MountDirPath)
			success++
			continue
		}

		fmt.Printf("    From: %s\n", entrySource(entry))
		fmt.Printf("    To: %s\n", entry.MountDirPath)

		if err := mgr.Mount(ctx, entry.Name); err != nil {
			printIndentedError(err)
			fail++
			continue
		}

		fmt.Println("    Successfully mounted")
		success++
		fmt.Println()
	}

	return
}

func mountDaemonEntries(ctx context.Context, cfg *config.Config, entries []*config.MountEntry) (success, fail int) {
	if len(entries) == 0 {
		return 0, 0
	}
	client, err := ensureDaemonClient(ctx, cfg)
	if err != nil {
		for _, entry := range entries {
			fmt.Printf("  %s\n", entry.Name)
			fmt.Fprintf(os.Stderr, "    ERROR: failed to start daemon: %v\n", err)
			fail++
		}
		return success, fail
	}
	snapshots := snapshotsFromEntries(entriesWithPasswords, entries)
	resp, err := client.Mount(ctx, snapshots)
	if err != nil {
		for _, entry := range entries {
			fmt.Printf("  %s\n", entry.Name)
			fmt.Fprintf(os.Stderr, "    ERROR: daemon mount request failed: %v\n", err)
			fail++
		}
		return success, fail
	}
	return printDaemonResults(entries, resp.Results, "mounted")
}

func unmountDaemonEntries(ctx context.Context, cfg *config.Config, entries []*config.MountEntry) (success, fail int) {
	if len(entries) == 0 {
		return 0, 0
	}
	client, err := ensureDaemonClient(ctx, cfg)
	if err != nil {
		for _, entry := range entries {
			fmt.Printf("  %s\n", entry.Name)
			fmt.Fprintf(os.Stderr, "    ERROR: failed to start daemon: %v\n", err)
			fail++
		}
		return success, fail
	}
	snapshots := snapshotsFromEntries(entriesWithoutPasswords, entries)
	resp, err := client.Unmount(ctx, snapshots)
	if err != nil {
		for _, entry := range entries {
			fmt.Printf("  %s\n", entry.Name)
			fmt.Fprintf(os.Stderr, "    ERROR: daemon unmount request failed: %v\n", err)
			fail++
		}
		return success, fail
	}
	return printDaemonResults(entries, resp.Results, "unmounted")
}

func entriesNeedSudo(mgr *drivers.Manager, entries []*config.MountEntry) (bool, error) {
	for _, entry := range entries {
		if isDaemonManagedEntry(entry) {
			continue
		}
		driver, err := mgr.DetectDriver(entry)
		if err != nil {
			return false, err
		}
		if driver.NeedsSudo() {
			return true, nil
		}
	}
	return false, nil
}

func isDaemonManagedEntry(entry *config.MountEntry) bool {
	return entry != nil && daemonapi.IsManagedType(entry.Type)
}

func splitDaemonManagedEntries(entries []*config.MountEntry) (local, managed []*config.MountEntry) {
	for _, entry := range entries {
		if isDaemonManagedEntry(entry) {
			managed = append(managed, entry)
			continue
		}
		local = append(local, entry)
	}
	return local, managed
}

func entrySource(entry *config.MountEntry) string {
	switch entry.Type {
	case "smb":
		return fmt.Sprintf("//%s:%d/%s", entry.SMB.Addr, entry.SMB.GetPort(), entry.SMB.ShareName)
	case "sshfs":
		return fmt.Sprintf("%s:%s", entry.SSHFS.Host, entry.SSHFS.RemotePath)
	case "webdav":
		if entry.WebDAV == nil {
			return ""
		}
		return webdavSource(entry.WebDAV.URL, entry.WebDAV.Path)
	case "s3":
		if entry.S3 == nil {
			return ""
		}
		return s3Source(entry.S3)
	default:
		return ""
	}
}

func s3Source(cfg *config.S3Config) string {
	root := cfg.Bucket
	if cfg.Path != "" {
		root += "/" + strings.Trim(cfg.Path, "/")
	}
	target := cfg.Endpoint
	if target == "" {
		target = cfg.Provider
	}
	return fmt.Sprintf("s3://%s@%s", root, target)
}

func webdavSource(rawURL, path string) string {
	rawURL = redactURLUserinfo(rawURL)
	if path == "" {
		return rawURL
	}
	return rawURL + ":" + path
}

func redactURLUserinfo(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.User == nil {
		return rawURL
	}
	parsed.User = url.User("xxxxx")
	return parsed.String()
}

type daemonClient interface {
	Health(ctx context.Context) (*daemonapi.HealthResponse, error)
	Mount(ctx context.Context, entries []daemonapi.EntrySnapshot) (*daemonapi.BatchResponse, error)
	Unmount(ctx context.Context, entries []daemonapi.EntrySnapshot) (*daemonapi.BatchResponse, error)
	Status(ctx context.Context, entries []daemonapi.EntrySnapshot) (*daemonapi.StatusResponse, error)
}

func newDaemonClient(cfg *config.Config) (daemonClient, error) {
	socketPath, err := daemonSocketPath(cfg)
	if err != nil {
		return nil, err
	}
	return daemon.NewClient(socketPath), nil
}

func ensureDaemonClient(ctx context.Context, cfg *config.Config) (daemonClient, error) {
	client, err := newDaemonClient(cfg)
	if err != nil {
		return nil, err
	}
	if _, err := client.Health(ctx); err == nil {
		return client, nil
	}
	if err := startDaemonProcess(cfg); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := client.Health(ctx); err == nil {
			return client, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil, fmt.Errorf("daemon did not become ready")
}

func startDaemonProcess(cfg *config.Config) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd, cleanup, err := prepareDaemonCommand(exe, cfg, os.Stdout, os.Stderr)
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		cleanup()
		return err
	}
	cleanup()
	return nil
}

func daemonSocketPath(cfg *config.Config) (string, error) {
	if cfg != nil && cfg.Daemon != nil && cfg.Daemon.SocketPath != "" {
		return cfg.Daemon.SocketPath, nil
	}
	return daemon.DefaultSocketPath()
}

func daemonSocketPathFromValue(socketPath string) (string, error) {
	if socketPath != "" {
		return socketPath, nil
	}
	return daemon.DefaultSocketPath()
}

func prepareDaemonCommand(exe string, cfg *config.Config, stdout, stderr io.Writer) (*exec.Cmd, func(), error) {
	cmd := exec.Command(exe, "daemon", "run")
	cmd.Stdin = nil
	cleanup := func() {}
	logTarget := "syslog"
	logFile := ""
	if cfg != nil && cfg.Daemon != nil {
		logTarget = cfg.Daemon.GetLogTarget()
		logFile = cfg.Daemon.LogFile
	}
	socketPath, err := daemonSocketPath(cfg)
	if err != nil {
		return nil, cleanup, err
	}
	cmd.Args = append(cmd.Args, "--socket-path="+socketPath)
	cmd.Args = append(cmd.Args, "--log-target="+logTarget)
	switch logTarget {
	case "stderr", "syslog":
		cmd.Stdout = stdout
		cmd.Stderr = stderr
	case "file":
		if logFile == "" {
			var err error
			logFile, err = daemon.DefaultLogPath()
			if err != nil {
				return nil, cleanup, err
			}
		}
		if err := os.MkdirAll(filepath.Dir(logFile), 0755); err != nil {
			return nil, cleanup, err
		}
		f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			return nil, cleanup, err
		}
		cmd.Args = append(cmd.Args, "--log-file="+logFile)
		cmd.Stdout = f
		cmd.Stderr = f
		cleanup = func() { _ = f.Close() }
	default:
		return nil, cleanup, fmt.Errorf("unsupported daemon log target %q", logTarget)
	}
	return cmd, cleanup, nil
}

type syslogLogger interface {
	Err(string) error
	Warning(string) error
	Notice(string) error
	Info(string) error
	Debug(string) error
	Close() error
}

var openSyslog = func() (syslogLogger, error) {
	return syslog.New(syslog.LOG_DAEMON|syslog.LOG_INFO, "gomount-daemon")
}

var daemonLogOutput func(slog.Level, string)

func configureDaemonLogging(logTarget, logFile string, stderr io.Writer) (func(), error) {
	if logTarget == "" {
		logTarget = "syslog"
	}
	switch logTarget {
	case "stderr":
		daemonLogOutput = nil
		return func() {}, nil
	case "file":
		if logFile == "" {
			var err error
			logFile, err = daemon.DefaultLogPath()
			if err != nil {
				return nil, err
			}
		}
		if err := os.MkdirAll(filepath.Dir(logFile), 0755); err != nil {
			return nil, err
		}
		f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			return nil, err
		}
		daemonLogOutput = func(_ slog.Level, text string) { _, _ = f.WriteString(text) }
		rclonelog.Handler.SetOutput(daemonLogOutput)
		return func() {
			rclonelog.Handler.ResetOutput()
			daemonLogOutput = nil
			_ = f.Close()
		}, nil
	case "syslog":
		writer, err := openSyslog()
		if err != nil {
			fmt.Fprintf(stderr, "gomount daemon: syslog unavailable, falling back to stderr: %v\n", err)
			daemonLogOutput = nil
			return func() {}, nil
		}
		daemonLogOutput = func(level slog.Level, text string) { writeSyslogLine(writer, level, text) }
		rclonelog.Handler.SetOutput(daemonLogOutput)
		return func() {
			rclonelog.Handler.ResetOutput()
			daemonLogOutput = nil
			_ = writer.Close()
		}, nil
	default:
		return nil, fmt.Errorf("unsupported daemon log target %q", logTarget)
	}
}

func logDaemonLine(text string) {
	level := slog.LevelInfo
	if strings.Contains(text, "ERROR") {
		level = slog.LevelError
	} else if strings.Contains(text, "WARNING") {
		level = slog.LevelWarn
	} else if strings.Contains(text, "DEBUG") {
		level = slog.LevelDebug
	}
	if daemonLogOutput != nil {
		daemonLogOutput(level, text)
	}
}

func writeSyslogLine(writer syslogLogger, level slog.Level, text string) {
	switch {
	case level >= slog.LevelError:
		_ = writer.Err(text)
	case level >= slog.LevelWarn:
		_ = writer.Warning(text)
	case level <= slog.LevelDebug:
		_ = writer.Debug(text)
	default:
		if strings.Contains(text, "NOTICE") {
			_ = writer.Notice(text)
			return
		}
		_ = writer.Info(text)
	}
}

type snapshotPasswordMode bool

const (
	entriesWithoutPasswords snapshotPasswordMode = false
	entriesWithPasswords    snapshotPasswordMode = true
)

func snapshotsFromEntries(passwordMode snapshotPasswordMode, entries []*config.MountEntry) []daemonapi.EntrySnapshot {
	snapshots := make([]daemonapi.EntrySnapshot, 0, len(entries))
	for _, entry := range entries {
		snapshot, _ := daemonapi.FromMountEntry(entry)
		if passwordMode == entriesWithoutPasswords {
			snapshot.Source.Password = ""
			snapshot.Source.AccessKeyID = ""
			snapshot.Source.SecretAccessKey = ""
			snapshot.Source.SessionToken = ""
		}
		snapshots = append(snapshots, snapshot)
	}
	return snapshots
}

func printDaemonResults(entries []*config.MountEntry, results []daemonapi.OperationResult, successWord string) (success, fail int) {
	byName := make(map[string]daemonapi.OperationResult, len(results))
	for _, result := range results {
		byName[result.Name] = result
	}
	for i, entry := range entries {
		fmt.Printf("  [%d/%d] %s\n", i+1, len(entries), entry.Name)
		if successWord == "mounted" {
			fmt.Printf("    From: %s\n", entrySource(entry))
			fmt.Printf("    To: %s\n", entry.MountDirPath)
		} else {
			fmt.Printf("    At: %s\n", entry.MountDirPath)
		}
		result, ok := byName[entry.Name]
		if !ok {
			fmt.Fprintln(os.Stderr, "    ERROR: daemon did not return a result")
			fail++
			continue
		}
		if !result.Success {
			if result.Error != nil {
				fmt.Fprintf(os.Stderr, "    ERROR: %s\n", result.Error.Message)
				if result.Error.Detail != "" {
					fmt.Fprintf(os.Stderr, "           %s\n", result.Error.Detail)
				}
			} else {
				fmt.Fprintf(os.Stderr, "    ERROR: %s\n", result.Message)
			}
			fail++
			continue
		}
		if result.Skipped {
			fmt.Printf("    %s\n\n", result.Message)
		} else {
			fmt.Printf("    Successfully %s\n\n", successWord)
		}
		success++
	}
	return success, fail
}

func refreshAllStatus(ctx context.Context, mgr *drivers.Manager, cfg *config.Config) error {
	var managed []*config.MountEntry
	for i := range cfg.Mounts {
		entry := &cfg.Mounts[i]
		if isDaemonManagedEntry(entry) {
			entry.IsMounted = false
			managed = append(managed, entry)
			continue
		}
		status, err := mgr.Status(ctx, entry.Name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to get status for %s: %v\n", entry.Name, err)
			entry.IsMounted = false
			continue
		}
		entry.IsMounted = status.Mounted
	}
	if len(managed) == 0 {
		return nil
	}
	client, err := newDaemonClient(cfg)
	if err != nil {
		return nil
	}
	resp, err := client.Status(ctx, snapshotsFromEntries(entriesWithoutPasswords, managed))
	if err != nil {
		if isDaemonUnavailableError(err) {
			return nil
		}
		fmt.Fprintf(os.Stderr, "Warning: failed to query daemon status: %v\n", err)
		return nil
	}
	byName := make(map[string]daemonapi.OperationResult, len(resp.Statuses))
	for _, status := range resp.Statuses {
		byName[status.Name] = status
	}
	for _, entry := range managed {
		if status, ok := byName[entry.Name]; ok && status.Managed {
			entry.IsMounted = status.Mounted
		}
	}
	return nil
}

func isDaemonUnavailableError(err error) bool {
	return errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ECONNREFUSED)
}

func unmountEntries(ctx context.Context, mgr *drivers.Manager, entries []*config.MountEntry) (success, fail int) {
	if len(entries) == 0 {
		return 0, 0
	}

	fmt.Printf("Unmounting %d share(s)...\n", len(entries))

	needSudo, err := entriesNeedSudo(mgr, entries)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to inspect mount privileges: %v\n", err)
	} else if needSudo {
		if err := interaction.EnsureSudoCached(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: sudo authentication failed: %v\n", err)
		}
	}

	for i, entry := range entries {
		fmt.Printf("  [%d/%d] %s\n", i+1, len(entries), entry.Name)
		fmt.Printf("    At: %s\n", entry.MountDirPath)

		if !entry.IsMounted {
			fmt.Printf("    Already unmounted\n\n")
			success++
			continue
		}

		if err := mgr.Unmount(ctx, entry.Name); err != nil {
			printIndentedError(err)
			fail++
			continue
		}

		fmt.Println("    Successfully unmounted")
		success++
		fmt.Println()
	}

	return
}

func runMount(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return err
	}

	mgr := createDriverManager(cfg)
	ctx := context.Background()

	if err := refreshAllStatus(ctx, mgr, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to refresh mount status: %v\n", err)
	}

	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "No mount entry specified. Use 'gomount interactive' for interactive selection.")
		return nil
	}

	var entries []*config.MountEntry
	for _, name := range args {
		entry, found := cfg.FindByName(name)
		if !found {
			return fmt.Errorf("mount entry '%s' not found", name)
		}
		entries = append(entries, entry)
	}

	local, managed := splitDaemonManagedEntries(entries)
	success, fail := mountEntries(ctx, mgr, local)
	dSuccess, dFail := mountDaemonEntries(ctx, cfg, managed)
	success += dSuccess
	fail += dFail
	if fail > 0 && success == 0 {
		return fmt.Errorf("%d mount(s) failed", fail)
	}

	return nil
}

func runUmount(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return err
	}

	mgr := createDriverManager(cfg)
	ctx := context.Background()

	if err := refreshAllStatus(ctx, mgr, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to refresh mount status: %v\n", err)
	}

	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "No mount entry specified. Use 'gomount interactive' for interactive selection.")
		return nil
	}

	var entries []*config.MountEntry
	for _, name := range args {
		entry, found := cfg.FindByName(name)
		if !found {
			return fmt.Errorf("mount entry '%s' not found", name)
		}
		entries = append(entries, entry)
	}

	local, managed := splitDaemonManagedEntries(entries)
	success, fail := unmountEntries(ctx, mgr, local)
	dSuccess, dFail := unmountDaemonEntries(ctx, cfg, managed)
	success += dSuccess
	fail += dFail
	if fail > 0 && success == 0 {
		return fmt.Errorf("%d unmount(s) failed", fail)
	}

	return nil
}

func runInteractive(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return err
	}

	mgr := createDriverManager(cfg)
	ctx := context.Background()

	if err := refreshAllStatus(ctx, mgr, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to refresh mount status: %v\n", err)
	}

	result := tui.SelectMountAction(cfg.Mounts)
	if result.Cancelled {
		fmt.Println("Cancelled")
		return nil
	}
	if len(result.ToMount) == 0 && len(result.ToUnmount) == 0 {
		return nil
	}

	uLocal, uManaged := splitDaemonManagedEntries(result.ToUnmount)
	mLocal, mManaged := splitDaemonManagedEntries(result.ToMount)
	uSuccess, uFail := unmountEntries(ctx, mgr, uLocal)
	duSuccess, duFail := unmountDaemonEntries(ctx, cfg, uManaged)
	mSuccess, mFail := mountEntries(ctx, mgr, mLocal)
	dmSuccess, dmFail := mountDaemonEntries(ctx, cfg, mManaged)

	totalSuccess := uSuccess + duSuccess + mSuccess + dmSuccess
	totalFail := uFail + duFail + mFail + dmFail
	if totalFail > 0 && totalSuccess == 0 {
		return fmt.Errorf("all operations failed")
	}

	return nil
}

func printIndentedError(err error) {
	var de *drivers.DriverError
	if errors.As(err, &de) {
		fmt.Fprintf(os.Stderr, "    ERROR: %s\n", de.CoreMessage())
		if ctx := drivers.ErrorContext(err, de.Op); ctx != "" {
			fmt.Fprintf(os.Stderr, "           %s\n", ctx)
		}
	} else {
		msg := err.Error()
		if idx := strings.Index(msg, "\n"); idx != -1 {
			msg = msg[:idx]
		}
		fmt.Fprintf(os.Stderr, "    ERROR: %s\n", strings.TrimSpace(msg))
	}
	fmt.Println()
}

type mkdirResultKind int

const (
	mkdirResultCreated mkdirResultKind = iota
	mkdirResultOwnerOK
	mkdirResultOwnerMismatch
)

type mkdirResult struct {
	kind     mkdirResultKind
	entry    *config.MountEntry
	needSudo bool
	fileUid  int
	fileGid  int
}

type mkdirCollector struct {
	toCreate      []mkdirResult
	ownerOK       []*config.MountEntry
	ownerMismatch []mkdirResult
}

func (c *mkdirCollector) add(r mkdirResult) {
	switch r.kind {
	case mkdirResultCreated:
		c.toCreate = append(c.toCreate, r)
	case mkdirResultOwnerOK:
		c.ownerOK = append(c.ownerOK, r.entry)
	case mkdirResultOwnerMismatch:
		c.ownerMismatch = append(c.ownerMismatch, r)
	}
}

func resolveMkdirEntries(cfg *config.Config, args []string) ([]*config.MountEntry, error) {
	if len(args) == 1 && args[0] == "*" {
		entries := make([]*config.MountEntry, len(cfg.Mounts))
		for i := range cfg.Mounts {
			entries[i] = &cfg.Mounts[i]
		}
		return entries, nil
	}
	var entries []*config.MountEntry
	for _, name := range args {
		entry, found := cfg.FindByName(name)
		if !found {
			return nil, fmt.Errorf("mount entry '%s' not found", name)
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func processMkdirEntry(entry *config.MountEntry, dryRun bool, currentUid, currentGid int) mkdirResult {
	info, err := os.Stat(entry.MountDirPath)
	if err == nil {
		return checkExistingDir(entry, info, currentUid, currentGid)
	}
	if !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "  ERROR: %s: %v\n", entry.Name, err)
		return mkdirResult{kind: mkdirResultOwnerOK, entry: entry}
	}
	return handleMissingDir(entry, dryRun, currentUid, currentGid)
}

func checkExistingDir(entry *config.MountEntry, info os.FileInfo, currentUid, currentGid int) mkdirResult {
	if !info.IsDir() {
		fmt.Fprintf(os.Stderr, "  ERROR: %s: path exists but is not a directory: %s\n", entry.Name, entry.MountDirPath)
		return mkdirResult{kind: mkdirResultOwnerOK, entry: entry}
	}

	fileUid, fileGid, err := interaction.GetFileOwner(entry.MountDirPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  Warning: %s: failed to check owner: %v\n", entry.Name, err)
		return mkdirResult{kind: mkdirResultOwnerOK, entry: entry}
	}

	if fileUid == currentUid && fileGid == currentGid {
		return mkdirResult{kind: mkdirResultOwnerOK, entry: entry}
	}
	return mkdirResult{kind: mkdirResultOwnerMismatch, entry: entry, fileUid: fileUid, fileGid: fileGid, needSudo: true}
}

func handleMissingDir(entry *config.MountEntry, dryRun bool, currentUid, currentGid int) mkdirResult {
	if dryRun {
		return mkdirResult{kind: mkdirResultCreated, entry: entry, needSudo: predictNeedsSudo(entry.MountDirPath)}
	}

	if err := interaction.MkdirAll(entry.MountDirPath, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "  ERROR: %s: %v\n", entry.Name, err)
		return mkdirResult{kind: mkdirResultOwnerOK, entry: entry}
	}
	if err := interaction.Chown(entry.MountDirPath, currentUid, currentGid); err != nil {
		fmt.Fprintf(os.Stderr, "  Warning: %s: failed to set owner: %v\n", entry.Name, err)
	}
	return mkdirResult{kind: mkdirResultCreated, entry: entry}
}

func predictNeedsSudo(path string) bool {
	_, err := os.Stat(filepath.Dir(path))
	if err != nil {
		return true
	}
	owned, _ := interaction.IsOwnedByCurrentUser(filepath.Dir(path))
	return !owned
}

func runMkdir(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return err
	}

	if len(args) == 0 {
		if !interaction.Confirm(fmt.Sprintf("将创建所有 %d 个挂载条目的目录，是否继续?", len(cfg.Mounts))) {
			fmt.Println("Cancelled")
			return nil
		}
		args = []string{"*"}
	}

	entries, err := resolveMkdirEntries(cfg, args)
	if err != nil {
		return err
	}

	if len(entries) == 0 {
		fmt.Println("No mount entries found.")
		return nil
	}

	currentUser, err := interaction.GetCurrentUser()
	if err != nil {
		return fmt.Errorf("failed to get current user: %w", err)
	}
	currentUid, _ := strconv.Atoi(currentUser.Uid)
	currentGid, _ := strconv.Atoi(currentUser.Gid)

	var c mkdirCollector
	for _, entry := range entries {
		c.add(processMkdirEntry(entry, mkdirDryRun, currentUid, currentGid))
	}

	if mkdirDryRun {
		printMkdirDryRun(c.toCreate, c.ownerOK, c.ownerMismatch, currentUid, currentGid)
		return nil
	}

	printMkdirResult(c.toCreate, c.ownerOK)
	promptOwnerFix(c.ownerMismatch, currentUid, currentGid)
	return nil
}

func promptOwnerFix(mismatches []mkdirResult, currentUid, currentGid int) {
	if len(mismatches) == 0 {
		return
	}

	fmt.Println()
	fmt.Println("以下目录的所有者与当前用户不一致:")
	for _, m := range mismatches {
		fmt.Printf("  %-30s  当前: uid=%d gid=%d  期望: uid=%d gid=%d\n",
			m.entry.MountDirPath, m.fileUid, m.fileGid, currentUid, currentGid)
	}
	fmt.Println()

	if !interaction.Confirm("是否修正所有者?") {
		return
	}

	if err := interaction.EnsureSudoCached(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: sudo authentication failed: %v\n", err)
	}
	fixed := 0
	for _, m := range mismatches {
		if err := interaction.Chown(m.entry.MountDirPath, currentUid, currentGid); err != nil {
			fmt.Fprintf(os.Stderr, "  ERROR: %s: %v\n", m.entry.MountDirPath, err)
			continue
		}
		fixed++
	}
	fmt.Printf("已修正 %d 个目录的所有者\n", fixed)
}

func mkdirNameWidth(entries ...[]*config.MountEntry) int {
	maxLen := 0
	for _, list := range entries {
		for _, e := range list {
			l := len(e.Name) + 1
			if l > maxLen {
				maxLen = l
			}
		}
	}
	return maxLen
}

func printMkdirDryRun(toCreate []mkdirResult, ownerOK []*config.MountEntry, ownerMismatch []mkdirResult, currentUid, currentGid int) {
	allEntries := make([]*config.MountEntry, 0, len(toCreate)+len(ownerOK)+len(ownerMismatch))
	for _, r := range toCreate {
		allEntries = append(allEntries, r.entry)
	}
	allEntries = append(allEntries, ownerOK...)
	for _, r := range ownerMismatch {
		allEntries = append(allEntries, r.entry)
	}
	w := mkdirNameWidth(allEntries)
	fmtStr := fmt.Sprintf("  %%-%ds %%s", w)

	if len(toCreate) > 0 {
		fmt.Println("将创建以下目录:")
		for _, c := range toCreate {
			sudoTag := ""
			if c.needSudo {
				sudoTag = " (需要 sudo)"
			}
			fmt.Printf(fmtStr+"%s\n", c.entry.Name+":", c.entry.MountDirPath, sudoTag)
		}
		fmt.Println()
	}

	if len(ownerOK) > 0 {
		fmt.Println("已存在，所有者一致:")
		for _, e := range ownerOK {
			fmt.Printf(fmtStr+"\n", e.Name+":", e.MountDirPath)
		}
		fmt.Println()
	}

	if len(ownerMismatch) > 0 {
		fmt.Println("已存在，所有者不一致:")
		for _, m := range ownerMismatch {
			fmt.Printf(fmtStr+"  当前所有者: uid=%d gid=%d\n",
				m.entry.Name+":", m.entry.MountDirPath, m.fileUid, m.fileGid)
		}
		fmt.Println()
		fmt.Println("将修正所有者的目录:")
		for _, m := range ownerMismatch {
			sudoTag := ""
			if m.needSudo {
				sudoTag = " (需要 sudo)"
			}
			fmt.Printf("  %s → uid=%d gid=%d%s\n", m.entry.MountDirPath, currentUid, currentGid, sudoTag)
		}
	}
}

func printMkdirResult(toCreate []mkdirResult, ownerOK []*config.MountEntry) {
	allEntries := make([]*config.MountEntry, 0, len(toCreate)+len(ownerOK))
	for _, r := range toCreate {
		allEntries = append(allEntries, r.entry)
	}
	allEntries = append(allEntries, ownerOK...)
	w := mkdirNameWidth(allEntries)
	fmtStr := fmt.Sprintf("  %%-%ds %%s", w)

	if len(toCreate) > 0 {
		fmt.Printf("已创建 %d 个目录:\n", len(toCreate))
		for _, c := range toCreate {
			fmt.Printf(fmtStr+"\n", c.entry.Name+":", c.entry.MountDirPath)
		}
		fmt.Println()
	}
	if len(ownerOK) > 0 {
		fmt.Printf("已跳过 %d 个已存在的目录:\n", len(ownerOK))
		for _, e := range ownerOK {
			fmt.Printf(fmtStr+"\n", e.Name+":", e.MountDirPath)
		}
	}
}
