package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
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

func getDefaultConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "config.yaml" // fallback to local config.yaml
	}
	return filepath.Join(homeDir, ".config", "wsh.yaml")
}

func loadConfig(configPath string) (*Config, error) {
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

func findEndpoint(config *Config, name string) (*Endpoint, error) {
	for _, endpoint := range config.Endpoints {
		if endpoint.Name == name {
			return &endpoint, nil
		}
	}
	return nil, fmt.Errorf("endpoint '%s' not found in config", name)
}

func printUsage(configPath string, config *Config) {
	fmt.Println("Usage:")
	fmt.Println("  wsh <endpoint-name>                    - Connect to a predefined endpoint")
	fmt.Println("  wsh <websocket-url>                    - Connect to a custom WebSocket URL")
	fmt.Println("  wsh -c <config-file> <endpoint-name>   - Use custom config file")
	fmt.Println("")
	fmt.Printf("Config file: %s\n", configPath)
	fmt.Println("")
	if config != nil && len(config.Endpoints) > 0 {
		fmt.Println("Available endpoints:")
		for _, endpoint := range config.Endpoints {
			fmt.Printf("  %-15s - %s\n", endpoint.Name, endpoint.Description)
		}
		fmt.Println("")
	}
}

func main() {
	var configPath string
	var targetURL string
	var arg string

	// 解析命令行参数
	switch len(os.Args) {
	case 1:
		// 没有参数，显示帮助
		configPath = getDefaultConfigPath()
		config, _ := loadConfig(configPath)
		printUsage(configPath, config)
		os.Exit(1)
	case 2:
		// 一个参数：可能是端点名称或URL
		arg = os.Args[1]
		configPath = getDefaultConfigPath()
	case 4:
		// 四个参数：-c <config-file> <endpoint-name>
		if os.Args[1] != "-c" {
			fmt.Println("Error: Invalid arguments")
			fmt.Println("Usage: wsh -c <config-file> <endpoint-name>")
			os.Exit(1)
		}
		configPath = os.Args[2]
		arg = os.Args[3]
	default:
		fmt.Println("Error: Invalid number of arguments")
		fmt.Println("Usage:")
		fmt.Println("  wsh <endpoint-name>")
		fmt.Println("  wsh <websocket-url>")
		fmt.Println("  wsh -c <config-file> <endpoint-name>")
		os.Exit(1)
	}

	// 检查是否是预定义的端点名称
	if !isURL(arg) {
		// 尝试从配置文件加载端点
		config, err := loadConfig(configPath)
		if err != nil {
			log.Fatal("failed to load config:", err)
		}

		endpoint, err := findEndpoint(config, arg)
		if err != nil {
			fmt.Printf("Error: %v\n\n", err)
			printUsage(configPath, config)
			os.Exit(1)
		}

		targetURL = endpoint.URL
		fmt.Printf("Connecting to endpoint '%s' (%s)...\n", endpoint.Name, endpoint.Description)
	} else {
		targetURL = arg
	}

	// 解析 URL
	u, err := url.Parse(targetURL)
	if err != nil {
		log.Fatal("invalid URL:", err)
	}

	logrus.SetLevel(logrus.ErrorLevel)

	fmt.Printf("Connecting to %s...\n", u.String())

	// 连接 WebSocket
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("dial error:", err)
	}
	defer c.Close()

	// 切换终端 raw 模式
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		log.Fatal(err)
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	// 捕获 Ctrl+C / 窗口大小变化
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGWINCH)
	go func() {
		for sig := range sigs {
			switch sig {
			case syscall.SIGINT:
				sendJSON(c, CmdMsg{Type: "cmd", Cmd: string([]byte{3})}) // Ctrl+C
			case syscall.SIGWINCH:
				resizeTerm(c)
			}
		}
	}()

	// 心跳
	go func() {
		for {
			time.Sleep(30 * time.Second)
			sendJSON(c, HeartbeatMsg{Type: "heartbeat", Data: ""})
		}
	}()

	// 接收服务端 raw 数据
	go func() {
		for {
			_, msg, err := c.ReadMessage()
			if err != nil {
				os.Exit(0)
			}
			os.Stdout.Write(msg)
		}
	}()

	// 启动时先发一次窗口大小
	resizeTerm(c)
	// 发送必要的环境变量
	sendJSON(c, CmdMsg{Type: "cmd", Cmd: "export TERM=xterm-256color\n"})

	// 从 stdin 读输入并发 JSON
	buf := make([]byte, 1024)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil {
			logrus.Errorf(`input error: %v`, err)
			return
		}
		logrus.Debugf("send message %v", buf[:n])
		if bytes.Equal(buf[:n], []byte{27, 91, 49, 53, 126}) {
			// 预留F5，用来杀连接
			c.Close()
			break
		}

		sendJSON(c, CmdMsg{Type: "cmd", Cmd: string(buf[:n])})
	}
}

func isURL(s string) bool {
	return len(s) > 6 && (s[:6] == "ws://" || s[:7] == "wss://")
}

func resizeTerm(c *websocket.Conn) {
	cols, rows, errGetSize := term.GetSize(int(os.Stdout.Fd()))
	if errGetSize != nil {
		rows = 47
		cols = 196
	}

	sendJSON(c, ResizeMsg{Type: "resize", Rows: rows, Cols: cols})
}

func sendJSON(c *websocket.Conn, v interface{}) {
	data, _ := json.Marshal(v)
	c.WriteMessage(websocket.TextMessage, data)
}
