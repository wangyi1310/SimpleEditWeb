package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/unicode"
)

// OpenResult 打开文件的结果
type OpenResult struct {
	Path     string `json:"path"`
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
}

// LocalEntry 本地目录条目（目录树）
type LocalEntry struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	IsDir   bool   `json:"isDir"`
	Size    int64  `json:"size"`
	ModTime string `json:"modTime"`
}

// App 主应用结构
type App struct {
	ctx   context.Context
	store *secretStore
	ssh   *SSHManager
}

// NewApp 创建应用
func NewApp() *App {
	store := &secretStore{}
	return &App{
		store: store,
		ssh:   NewSSHManager(store),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.ssh.setContext(ctx)
}

// ---- 文件操作 ----

// decodeBytes 自动识别编码并解码为 UTF-8 文本
func decodeBytes(b []byte) (string, string) {
	if len(b) >= 2 {
		if b[0] == 0xFF && b[1] == 0xFE { // UTF-16LE BOM
			if s, err := unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewDecoder().Bytes(b); err == nil {
				return string(s), "UTF-16LE"
			}
		}
		if b[0] == 0xFE && b[1] == 0xFF { // UTF-16BE BOM
			if s, err := unicode.UTF16(unicode.BigEndian, unicode.UseBOM).NewDecoder().Bytes(b); err == nil {
				return string(s), "UTF-16BE"
			}
		}
	}
	if utf8.Valid(b) {
		return strings.TrimPrefix(string(b), "\xEF\xBB\xBF"), "UTF-8"
	}
	// GB18030（覆盖 GBK/GB2312）
	if s, err := simplifiedchinese.GB18030.NewDecoder().Bytes(b); err == nil && utf8.Valid(s) {
		return string(s), "GB18030"
	}
	// 兜底：原样返回
	return string(b), "未知(按UTF-8处理)"
}

// encodeBytes 按指定编码编码
func encodeBytes(s, encoding string) []byte {
	switch encoding {
	case "GB18030":
		if b, err := simplifiedchinese.GB18030.NewEncoder().Bytes([]byte(s)); err == nil {
			return b
		}
	case "UTF-16LE":
		if b, err := unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewEncoder().Bytes([]byte(s)); err == nil {
			return b
		}
	case "UTF-16BE":
		if b, err := unicode.UTF16(unicode.BigEndian, unicode.UseBOM).NewEncoder().Bytes([]byte(s)); err == nil {
			return b
		}
	}
	return []byte(s)
}

// OpenFileDialogImpl 打开文本文件（原生对话框 + 编码识别）
func (a *App) OpenTextFile() (*OpenResult, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "打开文本文件",
		Filters: []runtime.FileFilter{
			{DisplayName: "所有文件 (*.*)", Pattern: "*.*"},
			{DisplayName: "文本文件 (*.txt;*.md;*.log;*.json;*.js;*.py)", Pattern: "*.txt;*.md;*.log;*.json;*.js;*.py"},
		},
	})
	if err != nil {
		return nil, err
	}
	if path == "" {
		return nil, nil // 用户取消
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取失败: %w", err)
	}
	content, enc := decodeBytes(raw)
	return &OpenResult{Path: path, Content: content, Encoding: enc}, nil
}

// SaveTextFile 保存到指定路径（按原编码写回）
func (a *App) SaveTextFile(path, content, encoding string) error {
	if path == "" {
		return fmt.Errorf("路径为空")
	}
	data := encodeBytes(content, encoding)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("写入失败: %w", err)
	}
	return nil
}

// SaveAsTextFile 另存为（原生对话框），返回新路径
func (a *App) SaveAsTextFile(content, encoding string) (string, error) {
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "另存为",
		DefaultFilename: "未命名.txt",
	})
	if err != nil {
		return "", err
	}
	if path == "" {
		return "", nil // 用户取消
	}
	if err := a.SaveTextFile(path, content, encoding); err != nil {
		return "", err
	}
	return path, nil
}

// ---- 目录树（打开文件夹）----

// OpenFolderDialog 弹出系统目录选择框，返回所选目录绝对路径（取消返回空串）
func (a *App) OpenFolderDialog() (string, error) {
	path, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择要打开的文件夹",
	})
	if err != nil {
		return "", err
	}
	return path, nil
}

// ListLocalDir 列出本地目录：目录在前、名称不区分大小写排序
func (a *App) ListLocalDir(dir string) ([]LocalEntry, error) {
	if dir == "" {
		dir = "/"
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("读取目录失败: %w", err)
	}
	out := make([]LocalEntry, 0, len(entries))
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue // 无权限等条目跳过，不让整棵树挂掉
		}
		le := LocalEntry{
			Name:    e.Name(),
			Path:    filepath.Join(dir, e.Name()),
			IsDir:   e.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime().Format("2006-01-02 15:04"),
		}
		out = append(out, le)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].IsDir != out[j].IsDir {
			return out[i].IsDir // 目录在前
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}

// ReadLocalPath 直接按绝对路径读取本地文件（自动识别编码），不弹对话框
func (a *App) ReadLocalPath(path string) (*OpenResult, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取失败: %w", err)
	}
	content, enc := decodeBytes(raw)
	return &OpenResult{Path: path, Content: content, Encoding: enc}, nil
}

// ---- 连接配置（明文 JSON，保存后直接可用）----

// ListConnections 列出全部连接
func (a *App) ListConnections() ([]Connection, error) { return a.store.loadConns() }

// SaveConnection 新增或更新连接（按 ID 判断；凭据随配置一起保存，下次直接使用）
func (a *App) SaveConnection(conn Connection) error {
	conns, err := a.store.loadConns()
	if err != nil {
		return err
	}
	if conn.ID == "" {
		b := make([]byte, 6)
		rand.Read(b)
		conn.ID = hex.EncodeToString(b)
	}
	found := false
	for i := range conns {
		if conns[i].ID == conn.ID {
			// 密码留空且是编辑已有连接：保留旧密码
			if conn.Password == "" && conns[i].Password != "" {
				conn.Password = conns[i].Password
			}
			conns[i] = conn
			found = true
			break
		}
	}
	if !found {
		conns = append(conns, conn)
	}
	return a.store.saveConns(conns)
}

// ReorderConnections 按 ID 列表顺序重排连接（前端拖拽排序后调用）
func (a *App) ReorderConnections(ids []string) error {
	conns, err := a.store.loadConns()
	if err != nil {
		return err
	}
	byID := make(map[string]Connection, len(conns))
	for _, c := range conns {
		byID[c.ID] = c
	}
	out := make([]Connection, 0, len(conns))
	for _, id := range ids {
		if c, ok := byID[id]; ok {
			out = append(out, c)
			delete(byID, id)
		}
	}
	// 未出现在 ids 中的连接（例如拖拽后新建的）追加到末尾，避免丢失
	for _, c := range conns {
		if _, ok := byID[c.ID]; ok {
			out = append(out, c)
		}
	}
	return a.store.saveConns(out)
}

// DeleteConnection 删除连接
func (a *App) DeleteConnection(id string) error {
	conns, err := a.store.loadConns()
	if err != nil {
		return err
	}
	out := conns[:0]
	for _, c := range conns {
		if c.ID != id {
			out = append(out, c)
		}
	}
	return a.store.saveConns(out)
}

// ---- 编辑器辅助 ----

// NowTimestamp 返回当前时间戳（前端标题栏用）
func (a *App) NowTimestamp() string { return time.Now().Format("2006-01-02 15:04:05") }
