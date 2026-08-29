# SimpleEditWeb

跨平台文本编辑器 + SSH 终端管理工具，基于 [Wails v2](https://wails.io)（Go 后端 + Vue 3 前端）。

## 功能

**编辑器**（CodeMirror 6）
- 编码自动识别与写回（UTF-8 / GBK(GB18030) / UTF-16，老文件不乱码）
- 查找替换（⌘F）、多光标、行号、语法高亮（Markdown / JS / Python / JSON）
- 字号调节（⌘+/-/⌘0）、自动换行、状态栏（行列 / 字符数 / 已选）
- Markdown 预览：marked + DOMPurify，编辑/预览分屏 + 双向滚动同步，分栏比例可拖拽

**SSH 管理**
- 连接配置保存于 `~/.simpleedit/connections.json`（明文，0600 权限）
- 支持密码 / 私钥认证；连接列表可拖拽排序、一键复制配置
- 连接成功后可自动执行指定命令（autoCmd）

**终端**（xterm.js）
- iTerm2 风格标签：⌘1-9 切换、⌘←/→ 前后切换、⌘W 关闭、拖拽排序
- 分屏：⌘D 竖分屏 / ⇧⌘D 横分屏，⌘[ / ⌘] 轮换焦点，分割条可拖拽调整比例
- 切换标签/分屏自动聚焦终端，滚动历史不丢失
- 内置 SFTP 文件面板

## 开发环境

- Go 1.25+
- Node.js 22+ / npm
- Wails CLI v2.15：`go install github.com/wailsapp/wails/v2/cmd/wails@v2.15.0`
- Linux 另需：`libgtk-3-dev libwebkit2gtk-4.1-dev libayatana-appindicator3-dev`

## 本地开发与构建

```bash
wails dev          # 热重载开发
wails build        # 生产构建（产物在 build/bin）
```

## CI / 发布

推送到 `main` 会自动触发三平台编译检查（`.github/workflows/ci.yml`）。

**发布 Release**：推送以 `v` 开头的 tag 即自动构建 macOS / Windows / Linux 三平台产物并发布 GitHub Release：

```bash
git tag v3.5.0
git push origin v3.5.0
```

产物清单：

| 平台 | 产物 |
|---|---|
| macOS（Apple Silicon + Intel 通用） | `SimpleEditWeb-macos-universal.zip` |
| Windows amd64 | `SimpleEditWeb-windows-amd64.zip` |
| Linux amd64 | `SimpleEditWeb-linux-amd64.tar.gz` |

每个产物附带 `.sha256` 校验文件。
