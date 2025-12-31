# smb_mount

一个方便的 Linux 系统上管理 SMB/CIFS 挂载的命令行工具，提供交互式 TUI 界面。

## 特性

- **简单配置**：在单个 YAML 文件中定义所有 SMB 共享
- **交互式 TUI**：美观的终端界面用于浏览和选择共享
- **挂载状态跟踪**：查看当前已挂载的共享
- **交互式选择**：轻松选择挂载/卸载操作
- **密码提示**：安全的密码输入，带有视觉反馈
- **权限提升**：需要时自动处理 sudo
- **挂载点管理**：自动创建和清理目录

## 安装

### 从源码安装

```bash
git clone https://github.com/hsldymq/smb_mount.git
cd smb_mount
make install
```

### 使用 Go 安装

```bash
go install github.com/hsldymq/smb_mount@latest
```

### 手动安装

```bash
git clone https://github.com/hsldymq/smb_mount.git
cd smb_mount
make build
sudo cp bin/smb_mount /usr/local/bin/
```

## 系统要求

- Linux 操作系统
- `mount.cifs` 命令（需安装 `cifs-utils` 软件包）
- 挂载操作需要 `sudo` 权限
- Go 1.25+ （从源码构建时需要）

### 安装依赖

在 Debian/Ubuntu 上：
```bash
sudo apt-get install cifs-utils
```

在 Fedora/RHEL 上：
```bash
sudo dnf install cifs-utils
```

在 Arch Linux 上：
```bash
sudo pacman -S cifs-utils
```

## 配置

在 `~/.config/smb_mount_config.yaml` 创建配置文件：

```yaml
base_dir: /mnt/smb_share

mounts:
  - name: nas1
    smb_addr: 10.0.1.2
    smb_port: 445
    share_name: shared_folder
    username: user1
    password: pass1          # 可选，如省略将提示输入
    mount_dir_name: nas1_mount

  - name: media_server
    smb_addr: 10.0.1.3
    share_name: media
    username: user2
    # 密码未存储 - 挂载时会提示
    mount_dir_path: /mnt/media
```

### 配置选项

| 字段 | 必需 | 默认值 | 描述 |
|-----|------|--------|------|
| `name` | 是 | - | 此挂载的唯一标识符 |
| `smb_addr` | 是 | - | SMB 服务器地址 |
| `smb_port` | 否 | 445 | SMB 服务器端口 |
| `share_name` | 是 | - | 服务器上的共享名称 |
| `username` | 是 | - | 登录用户名 |
| `password` | 否 | - | 登录密码（为空时提示输入） |
| `mount_dir_name` | 否 | `<name>` | base_dir 内的目录名 |
| `mount_dir_path` | 否 | - | 完整的自定义挂载路径（覆盖 base_dir 和 mount_dir_name） |

示例配置文件位于 `configs/smb_mount_config.yaml.example`。

## 使用方法

### 列出所有挂载点

显示所有配置的共享及其挂载状态：

```bash
smb_mount list
# 或
smb_mount -l
```

### 挂载共享

通过名称挂载特定共享：

```bash
smb_mount mount nas1
# 或
smb_mount -m nas1
```

交互式选择（支持多选）：
- 使用 `space` 切换多个共享的选中状态
- 按 `enter` 确认并挂载所有选中的共享

```bash
smb_mount mount
# 或
smb_mount -m
```

**注意**：挂载多个共享时，如果某个失败，其他共享会继续挂载。最后会显示汇总结果。

### 卸载共享

通过名称卸载特定共享：

```bash
smb_mount umount nas1
# 或
smb_mount -u nas1
```

交互式选择（支持多选）：

```bash
smb_mount umount
# 或
smb_mount -u
```

**注意**：批量操作时即使单个操作失败也会继续执行。最后会显示成功/失败计数。

### 自定义配置路径

使用来自自定义位置的配置文件：

```bash
smb_mount -c /path/to/config.yaml list
```

### 帮助

```bash
smb_mount --help
smb_mount list --help
smb_mount mount --help
smb_mount umount --help
```

## CLI 参考

```
smb_mount                  显示帮助（默认）
smb_mount list             列出所有配置的挂载点
smb_mount mount [name]     挂载 SMB 共享（不带名称时为交互式）
smb_mount umount [name]    卸载 SMB 共享（不带名称时为交互式）

全局选项：
  -c, --config string   配置文件路径（默认：~/.config/smb_mount_config.yaml）
  -h, --help            显示帮助
```

## TUI 导航

### 列表视图
- `↑` / `k` - 向上移动光标
- `↓` / `j` - 向下移动光标
- `g` / `home` - 跳到第一项
- `G` / `end` - 跳到最后一项
- `q` / `esc` - 退出

### 选择菜单（挂载/卸载）
- `↑` / `k` - 向上移动光标
- `↓` / `j` - 向下移动光标
- `space` - 切换当前项目的选中状态（多选）
- `enter` - 确认并挂载/卸载所有选中的项目
- `q` / `esc` - 取消

## 安全注意事项

- **密码存储**：避免在配置文件中以明文存储密码。省略 `password` 字段以交互式提示输入。
- **文件权限**：为配置文件设置限制性权限：
  ```bash
  chmod 600 ~/.config/smb_mount_config.yaml
  ```
- **凭据文件**：临时凭据文件以 0600 权限创建，使用后立即删除。

## 开发

### 构建

```bash
make build
```

### 为多平台构建

```bash
make build-all
```

### 运行测试

```bash
make test
```

### 代码检查

```bash
make lint
```

### 格式化代码

```bash
make fmt
```

## 项目结构

```
smb_mount/
├── cmd/smb_mount/          # 应用入口点
├── internal/
│   ├── config/             # 配置管理
│   ├── mount/              # 挂载/卸载操作
│   ├── tui/                # 终端 UI 组件
│   ├── prompt/             # 交互式提示
│   └── privilege/          # Sudo 处理
├── pkg/smb/                # 公共类型和错误
├── configs/                # 示例配置
├── Makefile                # 构建自动化
└── README.md
```

## 贡献

欢迎贡献！请随时提交 Pull Request。

## 许可证

本项目在 MIT 许可证下授权 - 详见 [LICENSE](LICENSE) 文件。

## 致谢

使用优秀的开源库构建：
- [Cobra](https://github.com/spf13/cobra) - CLI 框架
- [Viper](https://github.com/spf13/viper) - 配置管理
- [BubbleTea](https://github.com/charmbracelet/bubbletea) - TUI 框架
- [Lipgloss](https://github.com/charmbracelet/lipgloss) - 样式
- [moby/sys/mountinfo](https://github.com/moby/sys/mountinfo) - 挂载状态检测
