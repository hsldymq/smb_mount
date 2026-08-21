package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"

	"github.com/hsldymq/gomount/internal/config"
	"github.com/hsldymq/gomount/internal/drivers"
	sshfsDriver "github.com/hsldymq/gomount/internal/drivers/sshfs"
	"github.com/spf13/cobra"
)

func writeMainTestFile(t *testing.T, dir string, name string, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestEntriesNeedSudoReturnsFalseWhenSelectedDriversDoNotNeedSudo(t *testing.T) {
	entry := &config.MountEntry{Name: "dev", Type: "sshfs", SSHFS: &config.SSHFSConfig{Host: "dev", RemotePath: "/src"}}
	mgr := drivers.NewManager(&config.Config{Mounts: []config.MountEntry{*entry}})
	mgr.RegisterDriver(sshfsDriver.NewDriver())

	need, err := entriesNeedSudo(mgr, []*config.MountEntry{entry})
	if err != nil {
		t.Fatalf("entriesNeedSudo returned error: %v", err)
	}
	if need {
		t.Fatal("expected sshfs-only selection not to need sudo")
	}
}

func TestDaemonCommandAliasIsD(t *testing.T) {
	if len(daemonCmd.Aliases) != 1 || daemonCmd.Aliases[0] != "d" {
		t.Fatalf("daemon aliases = %+v, want [d]", daemonCmd.Aliases)
	}
}

func TestMkdirCommandAliasIsR(t *testing.T) {
	if len(mkdirCmd.Aliases) != 1 || mkdirCmd.Aliases[0] != "r" {
		t.Fatalf("mkdir aliases = %+v, want [r]", mkdirCmd.Aliases)
	}
}

func TestPrepareDaemonCommandDefaultsToSyslog(t *testing.T) {
	var stdout, stderr bytes.Buffer
	socketPath := t.TempDir() + "/gomount.sock"
	cmd, cleanup, err := prepareDaemonCommand("/bin/gomount", &config.Config{Daemon: &config.DaemonConfig{SocketPath: socketPath}}, &stdout, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	if cmd.Stdout != &stdout || cmd.Stderr != &stderr {
		t.Fatal("expected syslog fallback stderr to inherit parent stdio")
	}
	if !containsArg(cmd.Args, "--log-target=syslog") {
		t.Fatalf("expected daemon args to include syslog target, got %+v", cmd.Args)
	}
	if len(cmd.Args) < 3 || cmd.Args[1] != "daemon" || cmd.Args[2] != "run" {
		t.Fatalf("expected daemon run command, got %+v", cmd.Args)
	}
	if !containsArg(cmd.Args, "--socket-path="+socketPath) {
		t.Fatalf("expected daemon args to include socket path, got %+v", cmd.Args)
	}
}

func TestPrepareDaemonCommandCanUseLogFile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	logFile := t.TempDir() + "/daemon.log"
	cmd, cleanup, err := prepareDaemonCommand("/bin/gomount", &config.Config{Daemon: &config.DaemonConfig{LogTarget: "file", LogFile: logFile}}, &stdout, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	if cmd.Stdout == &stdout || cmd.Stderr == &stderr {
		t.Fatal("expected daemon output to be redirected away from parent stdio")
	}
	if cmd.Stdout == nil || cmd.Stderr == nil {
		t.Fatal("expected daemon stdout and stderr to be configured")
	}
	info, err := os.Stat(logFile)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("expected daemon log file mode 0600, got %o", info.Mode().Perm())
	}
	if !containsArg(cmd.Args, "--log-target=file") || !containsArg(cmd.Args, "--log-file="+logFile) {
		t.Fatalf("expected daemon args to include file target and path, got %+v", cmd.Args)
	}
}

func TestPrepareDaemonCommandCanUseStderr(t *testing.T) {
	var stdout, stderr bytes.Buffer
	cmd, cleanup, err := prepareDaemonCommand("/bin/gomount", &config.Config{Daemon: &config.DaemonConfig{LogTarget: "stderr"}}, &stdout, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	if cmd.Stdout != &stdout || cmd.Stderr != &stderr {
		t.Fatal("expected daemon output to inherit parent stdio for stderr target")
	}
	if !containsArg(cmd.Args, "--log-target=stderr") {
		t.Fatalf("expected daemon args to include stderr target, got %+v", cmd.Args)
	}
}

func containsArg(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
}

func TestRunDaemonStatusReportsNotRunningWhenDaemonUnavailable(t *testing.T) {
	dir := t.TempDir()
	path := writeMainTestFile(t, dir, "config.yaml", `
daemon:
  socket_path: ./custom.sock
`)
	oldConfigPath := configPath
	configPath = path
	t.Cleanup(func() { configPath = oldConfigPath })
	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := runDaemonStatus(cmd, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "daemon: not running") {
		t.Fatalf("expected not running output, got %q", out.String())
	}
	if strings.Contains(out.String(), "hint:") {
		t.Fatalf("expected no hint output, got %q", out.String())
	}
	if !strings.Contains(out.String(), "custom.sock") {
		t.Fatalf("expected configured socket path, got %q", out.String())
	}
}

func TestRunDaemonDownReportsNotRunningWhenDaemonUnavailable(t *testing.T) {
	dir := t.TempDir()
	path := writeMainTestFile(t, dir, "config.yaml", `
daemon:
  socket_path: ./down.sock
`)
	oldConfigPath := configPath
	configPath = path
	t.Cleanup(func() { configPath = oldConfigPath })
	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := runDaemonDown(cmd, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "daemon: not running") {
		t.Fatalf("expected not running output, got %q", out.String())
	}
}

func TestDaemonUnavailableErrorIncludesConnectionRefused(t *testing.T) {
	err := &os.PathError{Op: "dial", Path: "/tmp/gomount.sock", Err: syscall.ECONNREFUSED}
	if !isDaemonUnavailableError(err) {
		t.Fatalf("expected connection refused to mean daemon unavailable, got %v", err)
	}
}

func TestDaemonUnavailableErrorRejectsOtherErrors(t *testing.T) {
	if isDaemonUnavailableError(os.ErrPermission) {
		t.Fatal("expected permission errors to remain visible")
	}
}

func TestConfigureDaemonLoggingUsesSyslog(t *testing.T) {
	fake := &fakeSyslogLogger{}
	originalOpen := openSyslog
	openSyslog = func() (syslogLogger, error) { return fake, nil }
	t.Cleanup(func() { openSyslog = originalOpen })

	var stderr bytes.Buffer
	cleanup, err := configureDaemonLogging("syslog", "", &stderr)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	logDaemonLine("ERROR : upload failed")
	if len(fake.errors) != 1 || !strings.Contains(fake.errors[0], "upload failed") {
		t.Fatalf("expected syslog error message, got %+v", fake.errors)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected stderr to stay empty, got %q", stderr.String())
	}
}

func TestConfigureDaemonLoggingFallsBackToStderrWhenSyslogUnavailable(t *testing.T) {
	originalOpen := openSyslog
	openSyslog = func() (syslogLogger, error) { return nil, io.ErrClosedPipe }
	t.Cleanup(func() { openSyslog = originalOpen })

	var stderr bytes.Buffer
	cleanup, err := configureDaemonLogging("syslog", "", &stderr)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	if !strings.Contains(stderr.String(), "syslog unavailable, falling back to stderr") {
		t.Fatalf("expected fallback warning on stderr, got %q", stderr.String())
	}
}

type fakeSyslogLogger struct {
	errors []string
	infos  []string
}

func (f *fakeSyslogLogger) Err(message string) error {
	f.errors = append(f.errors, message)
	return nil
}

func (f *fakeSyslogLogger) Warning(message string) error { return f.Info(message) }
func (f *fakeSyslogLogger) Notice(message string) error  { return f.Info(message) }

func (f *fakeSyslogLogger) Info(message string) error {
	f.infos = append(f.infos, message)
	return nil
}

func (f *fakeSyslogLogger) Debug(message string) error { return f.Info(message) }
func (f *fakeSyslogLogger) Close() error               { return nil }

func TestPrepareDaemonCommandRejectsUnknownLogTarget(t *testing.T) {
	_, cleanup, err := prepareDaemonCommand("/bin/gomount", &config.Config{Daemon: &config.DaemonConfig{LogTarget: "system"}}, io.Discard, io.Discard)
	if cleanup != nil {
		cleanup()
	}
	if err == nil {
		t.Fatal("expected unknown log target to fail")
	}
	if !strings.Contains(err.Error(), "unsupported daemon log target") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEntriesNeedSudoIgnoresDaemonManagedEntries(t *testing.T) {
	entry := &config.MountEntry{Name: "docs", Type: "webdav", WebDAV: &config.WebDAVConfig{URL: "https://cloud.example.com/dav"}}
	mgr := createDriverManager(&config.Config{Mounts: []config.MountEntry{*entry}})

	need, err := entriesNeedSudo(mgr, []*config.MountEntry{entry})
	if err != nil {
		t.Fatalf("entriesNeedSudo returned error: %v", err)
	}
	if need {
		t.Fatal("expected webdav daemon-managed entry not to need local sudo")
	}
}

func TestSplitDaemonManagedEntries(t *testing.T) {
	nas := &config.MountEntry{Name: "nas", Type: "smb"}
	docs := &config.MountEntry{Name: "docs", Type: "webdav", WebDAV: &config.WebDAVConfig{URL: "https://cloud.example.com/dav"}}
	dev := &config.MountEntry{Name: "dev", Type: "sshfs"}

	local, managed := splitDaemonManagedEntries([]*config.MountEntry{nas, docs, dev})
	if len(local) != 2 || local[0].Name != "nas" || local[1].Name != "dev" {
		t.Fatalf("unexpected local entries: %+v", local)
	}
	if len(managed) != 1 || managed[0].Name != "docs" {
		t.Fatalf("unexpected managed entries: %+v", managed)
	}
}

func TestEntrySourceForWebDAV(t *testing.T) {
	entry := &config.MountEntry{Name: "docs", Type: "webdav", WebDAV: &config.WebDAVConfig{URL: "https://cloud.example.com/dav", Path: "/team"}}

	if got := entrySource(entry); got != "https://cloud.example.com/dav:/team" {
		t.Fatalf("entrySource() = %q", got)
	}
}

func TestEntrySourceForWebDAVRedactsURLUserinfo(t *testing.T) {
	entry := &config.MountEntry{Name: "docs", Type: "webdav", WebDAV: &config.WebDAVConfig{URL: "https://user:secret@cloud.example.com/dav", Path: "/team"}}

	if got := entrySource(entry); got != "https://xxxxx@cloud.example.com/dav:/team" {
		t.Fatalf("entrySource() = %q", got)
	}
}

func TestSnapshotsFromEntriesCanOmitWebDAVPassword(t *testing.T) {
	entry := &config.MountEntry{Name: "docs", Type: "webdav", WebDAV: &config.WebDAVConfig{URL: "https://cloud.example.com/dav", Username: "user", Password: "secret"}}

	snapshots := snapshotsFromEntries(entriesWithoutPasswords, []*config.MountEntry{entry})
	if len(snapshots) != 1 {
		t.Fatalf("expected one snapshot, got %d", len(snapshots))
	}
	if snapshots[0].Source.Password != "" {
		t.Fatalf("expected password to be omitted, got %q", snapshots[0].Source.Password)
	}
}

func TestSnapshotsFromEntriesCanIncludeWebDAVPassword(t *testing.T) {
	entry := &config.MountEntry{Name: "docs", Type: "webdav", WebDAV: &config.WebDAVConfig{URL: "https://cloud.example.com/dav", Username: "user", Password: "secret"}}

	snapshots := snapshotsFromEntries(entriesWithPasswords, []*config.MountEntry{entry})
	if len(snapshots) != 1 {
		t.Fatalf("expected one snapshot, got %d", len(snapshots))
	}
	if snapshots[0].Source.Password != "secret" {
		t.Fatalf("expected password to be included, got %q", snapshots[0].Source.Password)
	}
}

func TestS3EntrySourceAndSnapshotCredentialRedaction(t *testing.T) {
	entry := &config.MountEntry{Name: "archive", Type: "s3", MountDirPath: "/mnt/s3", S3: &config.S3Config{
		Provider: "aliyun_oss", Bucket: "my-bucket", Path: "/team/backups/", Endpoint: "oss-cn-hangzhou.aliyuncs.com",
		AccessKeyID: "id", SecretAccessKey: "secret", SessionToken: "token",
	}}
	if got := entrySource(entry); got != "s3://my-bucket/team/backups@oss-cn-hangzhou.aliyuncs.com" {
		t.Fatalf("entrySource() = %q", got)
	}
	snapshots := snapshotsFromEntries(entriesWithoutPasswords, []*config.MountEntry{entry})
	if len(snapshots) != 1 || snapshots[0].Source.AccessKeyID != "" || snapshots[0].Source.SecretAccessKey != "" || snapshots[0].Source.SessionToken != "" {
		t.Fatalf("expected S3 credentials to be redacted, got %+v", snapshots)
	}
}

func TestEntriesNeedSudoReturnsTrueWhenAnySelectedDriverNeedsSudo(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-specific SMB sudo behavior")
	}

	entry := &config.MountEntry{Name: "nas", Type: "smb", SMB: &config.SMBConfig{Addr: "nas", ShareName: "data", Username: "me"}}
	mgr := createDriverManager(&config.Config{Mounts: []config.MountEntry{*entry}})

	need, err := entriesNeedSudo(mgr, []*config.MountEntry{entry})
	if err != nil {
		t.Fatalf("entriesNeedSudo returned error: %v", err)
	}
	if !need {
		t.Fatal("expected smb selection to need sudo on Linux")
	}
}
