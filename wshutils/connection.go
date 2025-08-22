package wshutils

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"

	"github.com/gorilla/websocket"
	"golang.org/x/term"
)

type Endpoint struct {
	Name        string `yaml:"name"`
	URL         string `yaml:"url"`
	Description string `yaml:"description"`
}

type Config struct {
	Endpoints []Endpoint `yaml:"endpoints"`
}

type CmdMsg struct {
	Type string `json:"type"`
	Cmd  string `json:"cmd,omitempty"`
}

type ResizeMsg struct {
	Type string `json:"type"`
	Rows int    `json:"rows"`
	Cols int    `json:"cols"`
}

type HeartbeatMsg struct {
	Type string `json:"type"`
	Data string `json:"data"`
}

// Connection 封装WebSocket连接和相关功能
type Connection struct {
	conn *websocket.Conn
}

// GetDefaultConfigPath 获取默认配置文件路径
func GetDefaultConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "config.yaml" // fallback to local config.yaml
	}
	return filepath.Join(homeDir, ".config", "wsh.yaml")
}

// LoadConfig 加载配置文件
func LoadConfig(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file '%s': %v", configPath, err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file '%s': %v", configPath, err)
	}

	return &config, nil
}

// FindEndpoint 根据名称查找端点
func FindEndpoint(config *Config, name string) (*Endpoint, error) {
	for _, endpoint := range config.Endpoints {
		if endpoint.Name == name {
			return &endpoint, nil
		}
	}
	return nil, fmt.Errorf("endpoint '%s' not found in config", name)
}

// IsURL 检查字符串是否为URL
func IsURL(s string) bool {
	return len(s) > 6 && (s[:6] == "ws://" || s[:7] == "wss://")
}

// NewConnection 创建新的连接
func NewConnection(targetURL string) (*Connection, error) {
	u, err := url.Parse(targetURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %v", err)
	}

	logrus.SetLevel(logrus.ErrorLevel)

	fmt.Printf("Connecting to %s...\n", u.String())

	// 连接 WebSocket
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("dial error: %v", err)
	}

	return &Connection{conn: c}, nil
}

// Close 关闭连接
func (conn *Connection) Close() error {
	return conn.conn.Close()
}

// SendJSON 发送JSON消息
func (conn *Connection) SendJSON(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return conn.conn.WriteMessage(websocket.TextMessage, data)
}

// SendText 发送文本消息
func (conn *Connection) SendText(data string) error {
	return conn.conn.WriteMessage(websocket.TextMessage, []byte(data))
}

// ReadMessage 读取消息
func (conn *Connection) ReadMessage() (messageType int, p []byte, err error) {
	return conn.conn.ReadMessage()
}

// ResizeTerm 调整终端大小
func (conn *Connection) ResizeTerm() error {
	cols, rows, errGetSize := term.GetSize(int(os.Stdout.Fd()))
	if errGetSize != nil {
		rows = 47
		cols = 196
	}

	return conn.SendJSON(ResizeMsg{Type: "resize", Rows: rows, Cols: cols})
}

// SetupSignalHandlers 设置信号处理器
func (conn *Connection) SetupSignalHandlers() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGWINCH)
	go func() {
		for sig := range sigs {
			switch sig {
			case syscall.SIGINT:
				conn.SendJSON(CmdMsg{Type: "cmd", Cmd: string([]byte{3})}) // Ctrl+C
			case syscall.SIGWINCH:
				conn.ResizeTerm()
			}
		}
	}()
}

// StartHeartbeat 开始心跳
func (conn *Connection) StartHeartbeat() {
	go func() {
		for {
			time.Sleep(30 * time.Second)
			conn.SendJSON(HeartbeatMsg{Type: "heartbeat", Data: ""})
		}
	}()
}

// GetConn 获取原始连接
func (conn *Connection) GetConn() *websocket.Conn {
	return conn.conn
}
