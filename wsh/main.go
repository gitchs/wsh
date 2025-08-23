package main

import (
	"bytes"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gitchs/wsh/wshutils"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	configFile        string
	heartbeatInterval int
)

var rootCmd = &cobra.Command{
	Use:   "wsh [endpoint-name|websocket-url]",
	Short: "WebSocket Shell - Connect to remote shells via WebSocket",
	Long: `wsh is a WebSocket-based shell client that allows you to connect to remote shells.
You can connect using predefined endpoints from config file or direct WebSocket URLs.`,
	Args: cobra.MaximumNArgs(1),
	Run:  runWSH,
}

func init() {
	// 定义flags
	rootCmd.Flags().StringVarP(&configFile, "config", "c", "", "config file path")
	rootCmd.Flags().IntVar(&heartbeatInterval, "heartbeat-interval", 15, "heartbeat interval in seconds")
}

func setupLogging() {
	// 创建日志文件
	pid := os.Getpid()
	logFile := fmt.Sprintf("/tmp/wsh-%d.txt", pid)

	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		logrus.WithError(err).Error("Failed to open log file, using stdout")
	} else {
		logrus.SetOutput(file)
		logrus.Infof("Log file: %s", logFile)
	}

	// 设置日志格式
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	// 设置日志级别
	logrus.SetLevel(logrus.InfoLevel)
}

func runWSH(cmd *cobra.Command, args []string) {
	// 确定配置文件路径
	var configPath string
	if configFile != "" {
		configPath = configFile
	} else {
		configPath = wshutils.GetDefaultConfigPath()
	}

	// 如果没有参数，显示可用端点
	if len(args) == 0 {
		config, err := wshutils.LoadConfig(configPath)
		if err != nil {
			fmt.Printf("Error: Failed to load config: %v\n", err)
		}
		printAvailableEndpoints(configPath, config)
		return
	}

	arg := args[0]
	var targetURL string

	logrus.Infof("Starting wsh with arg: %s, config: %s, heartbeat: %ds", arg, configPath, heartbeatInterval)

	// 检查是否是预定义的端点名称
	if !wshutils.IsURL(arg) {
		// 尝试从配置文件加载端点
		config, err := wshutils.LoadConfig(configPath)
		if err != nil {
			fmt.Printf("Error: Failed to load config: %v\n", err)
			os.Exit(1)
		}

		endpoint, err := wshutils.FindEndpoint(config, arg)
		if err != nil {
			fmt.Printf("Error: Endpoint '%s' not found: %v\n", arg, err)
			printAvailableEndpoints(configPath, config)
			os.Exit(1)
		}

		targetURL = endpoint.URL
		fmt.Printf("Connecting to endpoint '%s' (%s)...\n", endpoint.Name, endpoint.Description)
		logrus.Infof("Using endpoint: %s -> %s", endpoint.Name, endpoint.URL)
	} else {
		targetURL = arg
		logrus.Infof("Using direct URL: %s", targetURL)
	}

	// 创建连接
	conn, err := wshutils.NewConnection(targetURL)
	if err != nil {
		fmt.Printf("Error: Failed to connect: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	// 连接成功后，设置日志重定向到文件
	setupLogging()
	logrus.Info("Connection established, logging redirected to file")

	logrus.Info("Connection established")

	// 切换终端 raw 模式
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		fmt.Printf("Error: Failed to set terminal raw mode: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		// 恢复终端状态
		term.Restore(int(os.Stdin.Fd()), oldState)
		// 重置终端，模仿reset命令的行为
		resetTerminal()

		// 将日志重定向到console
		logrus.SetOutput(os.Stdout)
		logrus.Infof("wsh exited, terminal reset completed")
	}()

	// 记录最后发送消息的时间
	var lastSendTime time.Time
	var lastSendMutex sync.Mutex

	// 更新最后发送时间的函数
	updateLastSendTime := func() {
		lastSendMutex.Lock()
		lastSendTime = time.Now()
		lastSendMutex.Unlock()
		logrus.Debug("Updated last send time")
	}

	// 设置信号处理器
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGWINCH)
	go func() {
		for sig := range sigs {
			switch sig {
			case syscall.SIGINT:
				logrus.Debug("Sending Ctrl+C")
				conn.SendJSON(wshutils.CmdMsg{Type: "cmd", Cmd: string([]byte{3})}) // Ctrl+C
				updateLastSendTime()
			case syscall.SIGWINCH:
				logrus.Debug("Window size changed, sending resize")
				conn.ResizeTerm()
				updateLastSendTime()
			}
		}
	}()

	// 启动终端resize监控
	go func() {
		ticker := time.NewTicker(1 * time.Second) // 每秒检查一次
		defer ticker.Stop()

		var lastCols, lastRows int

		for range ticker.C {
			cols, rows, err := term.GetSize(int(os.Stdout.Fd()))
			if err != nil {
				continue
			}

			// 如果终端大小发生变化，发送resize消息
			if cols != lastCols || rows != lastRows {
				logrus.Debugf("Terminal size changed: %dx%d -> %dx%d", lastCols, lastRows, cols, rows)
				conn.SendJSON(wshutils.ResizeMsg{Type: "resize", Rows: rows, Cols: cols})
				updateLastSendTime()
				lastCols, lastRows = cols, rows
			}
		}
	}()

	// 启动智能心跳
	go func() {
		ticker := time.NewTicker(1 * time.Second) // 每秒检查一次
		defer ticker.Stop()

		for range ticker.C {
			lastSendMutex.Lock()
			timeSinceLastSend := time.Since(lastSendTime)
			lastSendMutex.Unlock()

			// 如果超过设定时间没有发送消息，发送心跳
			if timeSinceLastSend > time.Duration(heartbeatInterval)*time.Second {
				logrus.Debugf("Sending heartbeat (last send: %v ago)", timeSinceLastSend)
				conn.SendJSON(wshutils.HeartbeatMsg{Type: "heartbeat", Data: ""})
				updateLastSendTime()
			}
		}
	}()

	// 接收服务端 raw 数据
	go func() {
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				logrus.WithError(err).Info("Connection closed")
				os.Exit(0)
			}
			os.Stdout.Write(msg)
		}
	}()

	// 启动时先发一次窗口大小
	conn.ResizeTerm()
	updateLastSendTime()

	// 发送必要的环境变量
	conn.SendJSON(wshutils.CmdMsg{Type: "cmd", Cmd: "export TERM=xterm-256color\n"})
	updateLastSendTime()

	logrus.Info("Entering interactive mode")

	// 从 stdin 读输入并发 JSON
	buf := make([]byte, 1024)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil {
			logrus.WithError(err).Error("Input error")
			return
		}

		logrus.Debugf("Sending user input: %d bytes", n)

		if bytes.Equal(buf[:n], []byte{27, 91, 50, 52, 126}) {
			// 预留F12，用来杀连接
			logrus.Info("F12 pressed, closing connection")
			conn.Close()
			break
		}

		conn.SendJSON(wshutils.CmdMsg{Type: "cmd", Cmd: string(buf[:n])})
		updateLastSendTime()
	}
}

func resetTerminal() {
	// 发送reset命令的终端控制序列
	// 这些序列模仿reset命令的行为

	// 1. 清除屏幕并移动光标到左上角
	fmt.Print("\033[2J")

	// 2. 移动光标到第一行第一列
	fmt.Print("\033[H")

	// 3. 重置所有属性（颜色、样式等）
	fmt.Print("\033[0m")

	// 4. 重置光标形状
	fmt.Print("\033[0 q")

	// 5. 显示光标
	fmt.Print("\033[?25h")

	// 6. 重置键盘模式
	fmt.Print("\033[?1l")

	// 7. 重置终端模式
	fmt.Print("\033[?7h")

	// 8. 重置自动换行
	fmt.Print("\033[?25h")

	logrus.Debug("Terminal reset completed")
}

func printAvailableEndpoints(configPath string, config *wshutils.Config) {
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
	if err := rootCmd.Execute(); err != nil {
		fmt.Printf("Error: Command execution failed: %v\n", err)
		os.Exit(1)
	}
}
