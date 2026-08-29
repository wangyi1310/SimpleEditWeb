package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/pkg/sftp"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/crypto/ssh"
)

// SftpEntry SFTP 目录条目
type SftpEntry struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	IsDir   bool   `json:"isDir"`
	ModTime string `json:"modTime"`
}

type sshSession struct {
	id      string
	connID  string
	sess    *ssh.Session
	stdin   io.WriteCloser
	closing bool
}

type sshClient struct {
	client   *ssh.Client
	sftp     *sftp.Client
	sessions map[string]*sshSession
}

// SSHManager SSH 会话与 SFTP 管理器
type SSHManager struct {
	ctx      context.Context
	mu       sync.Mutex
	clients  map[string]*sshClient
	sessions map[string]*sshSession
	store    *secretStore
}

// NewSSHManager 创建管理器
func NewSSHManager(store *secretStore) *SSHManager {
	return &SSHManager{
		clients:  map[string]*sshClient{},
		sessions: map[string]*sshSession{},
		store:    store,
	}
}

func (m *SSHManager) setContext(ctx context.Context) { m.ctx = ctx }

// emit 发事件到前端
func (m *SSHManager) emit(name string, data ...interface{}) {
	if m.ctx == nil {
		return
	}
	runtime.EventsEmit(m.ctx, name, data...)
}

// buildAuth 根据连接配置构造认证方式
func buildAuth(conn Connection) ([]ssh.AuthMethod, error) {
	if conn.AuthType == "key" {
		keyPath := expandPath(conn.KeyPath)
		raw, err := os.ReadFile(keyPath)
		if err != nil {
			return nil, fmt.Errorf("读取私钥失败: %w", err)
		}
		var signer ssh.Signer
		if conn.KeyPassphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(raw, []byte(conn.KeyPassphrase))
		} else {
			signer, err = ssh.ParsePrivateKey(raw)
		}
		if err != nil {
			return nil, fmt.Errorf("解析私钥失败: %w", err)
		}
		return []ssh.AuthMethod{ssh.PublicKeys(signer)}, nil
	}
	if conn.Password == "" {
		return nil, fmt.Errorf("密码为空")
	}
	return []ssh.AuthMethod{ssh.Password(conn.Password)}, nil
}

func expandPath(p string) string {
	if strings.HasPrefix(p, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, p[2:])
	}
	return p
}

// Connect 建立连接并启动交互式 shell，返回会话 ID
func (m *SSHManager) Connect(conn Connection) (string, error) {
	auth, err := buildAuth(conn)
	if err != nil {
		return "", err
	}
	if conn.Port == 0 {
		conn.Port = 22
	}
	cfg := &ssh.ClientConfig{
		User:            conn.User,
		Auth:            auth,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // 简化处理：不校验主机指纹
		Timeout:         12 * time.Second,
	}
	client, err := ssh.Dial("tcp", net.JoinHostPort(conn.Host, fmt.Sprint(conn.Port)), cfg)
	if err != nil {
		return "", fmt.Errorf("连接失败: %w", err)
	}

	sessionID := fmt.Sprintf("%s-%d", conn.ID, rand.Int63())
	sess, err := client.NewSession()
	if err != nil {
		client.Close()
		return "", err
	}
	stdin, err := sess.StdinPipe()
	if err != nil {
		client.Close()
		return "", err
	}
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := sess.RequestPty("xterm-256color", 24, 80, modes); err != nil {
		client.Close()
		return "", fmt.Errorf("请求 PTY 失败: %w", err)
	}
	stdout, _ := sess.StdoutPipe()
	stderr, _ := sess.StderrPipe()
	if err := sess.Shell(); err != nil {
		client.Close()
		return "", fmt.Errorf("启动 shell 失败: %w", err)
	}

	m.mu.Lock()
	sc, ok := m.clients[conn.ID]
	if !ok {
		sc = &sshClient{client: client, sessions: map[string]*sshSession{}}
		m.clients[conn.ID] = sc
	}
	s := &sshSession{id: sessionID, connID: conn.ID, sess: sess, stdin: stdin}
	sc.sessions[sessionID] = s
	m.sessions[sessionID] = s
	m.mu.Unlock()

	// 读 stdout/stderr，base64 后推给前端
	pump := func(r io.Reader) {
		buf := make([]byte, 8192)
		for {
			n, err := r.Read(buf)
			if n > 0 {
				m.emit("ssh:data:"+sessionID, base64.StdEncoding.EncodeToString(buf[:n]))
			}
			if err != nil {
				break
			}
		}
	}
	go pump(stdout)
	go pump(stderr)

	// 连接成功后自动执行命令（模拟用户在终端输入并回车）
	if conn.AutoCmd != "" {
		go func(sessionID, cmd string) {
			time.Sleep(400 * time.Millisecond) // 等 shell 就绪
			if err := m.Write(sessionID, cmd+"\r"); err != nil {
				msg := "\r\n\x1b[90m[auto-cmd] 命令发送失败: " + err.Error() + "\x1b[0m\r\n"
				m.emit("ssh:data:"+sessionID, base64.StdEncoding.EncodeToString([]byte(msg)))
			}
		}(sessionID, conn.AutoCmd)
	}

	// 会话结束监听
	go func() {
		_ = sess.Wait()
		m.emit("ssh:exit:"+sessionID, "")
		m.removeSession(sessionID)
	}()

	return sessionID, nil
}

func (m *SSHManager) removeSession(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return
	}
	delete(m.sessions, id)
	if sc, ok := m.clients[s.connID]; ok {
		delete(sc.sessions, id)
		if len(sc.sessions) == 0 {
			if sc.sftp != nil {
				sc.sftp.Close()
			}
			sc.client.Close()
			delete(m.clients, s.connID)
		}
	}
}

// Write 终端输入
func (m *SSHManager) Write(sessionID, data string) error {
	m.mu.Lock()
	s, ok := m.sessions[sessionID]
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("会话不存在或已关闭")
	}
	_, err := s.stdin.Write([]byte(data))
	return err
}

// Resize 终端尺寸变化
func (m *SSHManager) Resize(sessionID string, cols, rows int) error {
	m.mu.Lock()
	s, ok := m.sessions[sessionID]
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("会话不存在")
	}
	if cols < 2 || cols > 1000 || rows < 2 || rows > 1000 {
		return nil
	}
	return s.sess.WindowChange(rows, cols)
}

// CloseSession 主动关闭会话
func (m *SSHManager) CloseSession(sessionID string) error {
	m.mu.Lock()
	s, ok := m.sessions[sessionID]
	m.mu.Unlock()
	if !ok {
		return nil
	}
	s.closing = true
	s.sess.Close()
	m.emit("ssh:exit:"+sessionID, "")
	m.removeSession(sessionID)
	return nil
}

// getSftp 取（或懒建）某连接的 SFTP 客户端
func (m *SSHManager) getSftp(sessionID string) (*sftp.Client, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("会话不存在，请先连接")
	}
	sc := m.clients[s.connID]
	if sc.sftp == nil {
		cl, err := sftp.NewClient(sc.client)
		if err != nil {
			return nil, fmt.Errorf("打开 SFTP 通道失败: %w", err)
		}
		sc.sftp = cl
	}
	return sc.sftp, nil
}

// SftpList 列目录
func (m *SSHManager) SftpList(sessionID, path string) ([]SftpEntry, error) {
	cl, err := m.getSftp(sessionID)
	if err != nil {
		return nil, err
	}
	if path == "" {
		path = "."
	}
	infos, err := cl.ReadDir(path)
	if err != nil {
		return nil, err
	}
	entries := make([]SftpEntry, 0, len(infos))
	for _, fi := range infos {
		entries = append(entries, SftpEntry{
			Name:    fi.Name(),
			Size:    fi.Size(),
			IsDir:   fi.IsDir(),
			ModTime: fi.ModTime().Format("2006-01-02 15:04"),
		})
	}
	return entries, nil
}

// SftpReadFile 读远端文本文件（10MB 上限）
func (m *SSHManager) SftpReadFile(sessionID, path string) (string, error) {
	cl, err := m.getSftp(sessionID)
	if err != nil {
		return "", err
	}
	f, err := cl.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	buf := make([]byte, 10*1024*1024)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return "", err
	}
	content, _ := decodeBytes(buf[:n])
	return content, nil
}

// SftpWriteFile 写远端文本文件
func (m *SSHManager) SftpWriteFile(sessionID, path, content string) error {
	cl, err := m.getSftp(sessionID)
	if err != nil {
		return err
	}
	f, err := cl.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write([]byte(content))
	return err
}

// SftpDownload 下载远端文件到本地（弹保存对话框）
func (m *SSHManager) SftpDownload(sessionID, remotePath string) (string, error) {
	cl, err := m.getSftp(sessionID)
	if err != nil {
		return "", err
	}
	local, err := runtime.SaveFileDialog(m.ctx, runtime.SaveDialogOptions{
		DefaultFilename: filepath.Base(remotePath),
		Title:           "下载到本地",
	})
	if err != nil || local == "" {
		return "", err
	}
	rf, err := cl.Open(remotePath)
	if err != nil {
		return "", err
	}
	defer rf.Close()
	lf, err := os.Create(local)
	if err != nil {
		return "", err
	}
	defer lf.Close()
	if _, err := io.Copy(lf, rf); err != nil {
		return "", err
	}
	return local, nil
}

// SftpUpload 从本地上传（弹多选对话框）
func (m *SSHManager) SftpUpload(sessionID, remoteDir string) (int, error) {
	cl, err := m.getSftp(sessionID)
	if err != nil {
		return 0, err
	}
	files, err := runtime.OpenMultipleFilesDialog(m.ctx, runtime.OpenDialogOptions{
		Title: "选择要上传的文件",
	})
	if err != nil || len(files) == 0 {
		return 0, err
	}
	ok := 0
	var lastErr error
	for _, local := range files {
		lf, err := os.Open(local)
		if err != nil {
			lastErr = err
			continue
		}
		remote := remoteDir + "/" + filepath.Base(local)
		rf, err := cl.Create(remote)
		if err != nil {
			lf.Close()
			lastErr = err
			continue
		}
		if _, err := io.Copy(rf, lf); err != nil {
			lastErr = err
		} else {
			ok++
		}
		lf.Close()
		rf.Close()
	}
	if ok == 0 && lastErr != nil {
		return 0, lastErr
	}
	return ok, nil
}

// SftpMkdir 新建远端目录
func (m *SSHManager) SftpMkdir(sessionID, path string) error {
	cl, err := m.getSftp(sessionID)
	if err != nil {
		return err
	}
	return cl.Mkdir(path)
}

// SftpRemove 删除远端文件/目录
func (m *SSHManager) SftpRemove(sessionID, path string, isDir bool) error {
	cl, err := m.getSftp(sessionID)
	if err != nil {
		return err
	}
	if isDir {
		return cl.RemoveDirectory(path)
	}
	return cl.Remove(path)
}

// SftpRename 重命名
func (m *SSHManager) SftpRename(sessionID, oldPath, newPath string) error {
	cl, err := m.getSftp(sessionID)
	if err != nil {
		return err
	}
	return cl.Rename(oldPath, newPath)
}
