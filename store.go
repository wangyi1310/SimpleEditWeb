package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Connection 一条 SSH 连接配置（含凭据，明文保存，可直接复用）
type Connection struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Host          string `json:"host"`
	Port          int    `json:"port"`
	User          string `json:"user"`
	AuthType      string `json:"authType"` // "password" | "key"
	Password      string `json:"password,omitempty"`
	KeyPath       string `json:"keyPath,omitempty"`
	KeyPassphrase string `json:"keyPassphrase,omitempty"`
	AutoCmd       string `json:"autoCmd,omitempty"` // 连接成功后自动执行的命令
}

// storePath 连接配置文件：明文 JSON，保存后可直接使用
func storePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".simpleedit", "connections.json")
}

// secretStore 保留类型名以兼容 ssh.go 引用；实际就是明文 JSON 读写器
type secretStore struct{}

// loadConns 读取连接列表（文件不存在时返回空列表）
func (s *secretStore) loadConns() ([]Connection, error) {
	raw, err := os.ReadFile(storePath())
	if err != nil {
		if os.IsNotExist(err) {
			return []Connection{}, nil
		}
		return nil, err
	}
	var conns []Connection
	if err := json.Unmarshal(raw, &conns); err != nil {
		// 文件损坏时不阻塞使用，返回空列表
		return []Connection{}, nil
	}
	return conns, nil
}

// saveConns 写回连接列表
func (s *secretStore) saveConns(conns []Connection) error {
	p := storePath()
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return err
	}
	if conns == nil {
		conns = []Connection{}
	}
	raw, err := json.MarshalIndent(conns, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, raw, 0600)
}
