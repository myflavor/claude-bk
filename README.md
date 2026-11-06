# claude-bk

Claude Code 对话备份工具。把**当前目录**对应的对话记录打包成压缩文件，上传到 NAS（或本地目录）；也能一条命令拉回来。

设计原则：只做备份，不做别的。没有数据库、没有索引、没有守护进程、没有增量清单。

## 安装

```bash
go build -o ~/.local/bin/claude-bk ./cmd/claude-bk
```

## 配置

配置放在 `~/.claude-bk/<provider>.json`。

SMB：

```json
{
  "type": "smb",
  "host": "192.168.6.100",
  "share": "backup",
  "user": "xin",
  "password": "your-password",
  "root": "claude-bk"
}
```

| 字段 | 必填 | 说明 |
|---|---|---|
| `type` | ✅ | `smb` 或 `local` |
| `host` | ✅ | IP 或主机名，不带 `smb://` |
| `share` | ✅ | SMB 共享名（Windows 里 `\\主机\共享名` 的那一段） |
| `user` | ✅ | 用户名 |
| `password` | ✅ | 密码 |
| `root` | ⭕ | 共享内的子目录，默认共享根，不存在会自动创建 |
| `port` | ⭕ | 默认 445 |

本地目录（测试或应急）：

```json
{ "type": "local", "path": "/mnt/d/claude-backup" }
```

> 配置文件里有明文密码。放在 `~/.claude-bk/`，不要提交到仓库；本仓库只放 `examples/*.example`。

## 用法

```bash
cd /home/xin/mg4
claude-bk nas push     # 打包并上传，覆盖同名备份
claude-bk nas pull     # 下载并还原
```

项目由**当前工作目录**决定，所以要在你启动 Claude Code 的同一个目录里执行。

## 备什么

只有一条路径：

```
$CLAUDE_CONFIG_DIR/projects/<enc>/
├── <session-uuid>.jsonl                  对话主干
├── <session-uuid>/subagents/*.jsonl|.meta.json   子 agent 对话
├── <session-uuid>/tool-results/*.txt     被截断的大工具输出
├── memory/                                项目记忆
└── MEMORY.md
```

不备份：`~/.claude.json`、`history.jsonl`、`plans/`、`file-history/`、`sessions/*.key`、`jobs/`、`paste-cache/`、`session-env/`、`shell-snapshots/`、`backups/`、`plugins/`。

## 存储布局

```
<share>/<root>/<enc>.tar.zst
```

`<enc>` 是 Claude Code 的目录名，例如 `/home/xin/mg4` → `-home-xin-mg4`。固定文件名，每次覆盖。

编码规则（与 Claude Code 一致）：把路径中所有非 `[a-zA-Z0-9]` 字符替换成 `-`；超过 200 字节时取前 200 字节，加上 `-` 和原路径 base36 哈希。

## 实现说明

- 打包与压缩全程流式，不写临时文件；压缩用 zstd（Better 档）
- 上传先写 `.part` 再改名，避免中断留下半个文件
- 还原时拒绝绝对路径和 `..`，防止恶意归档写出目录外
- 读取 `CLAUDE_CONFIG_DIR`，与 Claude Code 行为一致
- 路径先做 `EvalSymlinks`，因为 Claude Code 记录的是物理路径

实测参考（本机）：mg4 3.3 MB → 745 KB；massage 132.5 MB → 24.5 MB，push 0.73 秒。
