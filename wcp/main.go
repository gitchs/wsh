package main

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/gitchs/wsh/wshutils"
)

const (
	// 每256字节发送1条消息
	chunkSize = 256
	// 结束标记
	endMarker = "__EOF"
)

func printUsage(configPath string, config *wshutils.Config) {
	fmt.Println("Usage:")
	fmt.Println("  wcp <endpoint-name> <local-file>                    - Copy file to remote endpoint")
	fmt.Println("  wcp <websocket-url> <local-file>                    - Copy file to custom WebSocket URL")
	fmt.Println("  wcp -c <config-file> <endpoint-name> <local-file>   - Use custom config file")
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
	var localFile string
	var arg string

	// 解析命令行参数
	switch len(os.Args) {
	case 1:
		// 没有参数，显示帮助
		configPath = wshutils.GetDefaultConfigPath()
		config, _ := wshutils.LoadConfig(configPath)
		printUsage(configPath, config)
		os.Exit(1)
	case 3:
		// 两个参数：<endpoint-name/url> <local-file>
		arg = os.Args[1]
		localFile = os.Args[2]
		configPath = wshutils.GetDefaultConfigPath()
	case 4:
		// 四个参数：-c <config-file> <endpoint-name> <local-file>
		if os.Args[1] != "-c" {
			fmt.Println("Error: Invalid arguments")
			fmt.Println("Usage: wcp -c <config-file> <endpoint-name> <local-file>")
			os.Exit(1)
		}
		configPath = os.Args[2]
		arg = os.Args[3]
		localFile = os.Args[4]
	default:
		fmt.Println("Error: Invalid number of arguments")
		fmt.Println("Usage:")
		fmt.Println("  wcp <endpoint-name> <local-file>")
		fmt.Println("  wcp <websocket-url> <local-file>")
		fmt.Println("  wcp -c <config-file> <endpoint-name> <local-file>")
		os.Exit(1)
	}

	// 检查本地文件是否存在
	if _, err := os.Stat(localFile); os.IsNotExist(err) {
		log.Fatalf("Local file '%s' does not exist", localFile)
	}

	// 检查是否是预定义的端点名称
	if !wshutils.IsURL(arg) {
		// 尝试从配置文件加载端点
		config, err := wshutils.LoadConfig(configPath)
		if err != nil {
			log.Fatal("failed to load config:", err)
		}

		endpoint, err := wshutils.FindEndpoint(config, arg)
		if err != nil {
			fmt.Printf("Error: %v\n\n", err)
			printUsage(configPath, config)
			os.Exit(1)
		}

		targetURL = endpoint.URL
		fmt.Printf("Copying to endpoint '%s' (%s)...\n", endpoint.Name, endpoint.Description)
	} else {
		targetURL = arg
	}

	// 创建连接
	conn, err := wshutils.NewConnection(targetURL)
	if err != nil {
		log.Fatal("Failed to connect:", err)
	}
	defer conn.Close()

	// 执行文件传输
	if err := transferFile(conn, localFile); err != nil {
		log.Fatal("File transfer failed:", err)
	}

	fmt.Printf("File '%s' successfully transferred\n", localFile)

	// 等待接收响应消息
	fmt.Println("Waiting for response...")
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			fmt.Printf("Connection closed: %v\n", err)
			break
		}
		fmt.Printf("Received: %s", string(msg))
	}
}

// transferFile 执行文件传输
func transferFile(conn *wshutils.Connection, localFile string) error {
	fileName := filepath.Base(localFile)

	// 1. 发送握手消息
	handshakeMsg := fmt.Sprintf("cat <<'__EOF' |base64 --decode |gunzip > %s\n", fileName)
	fmt.Println("Sending handshake message...")
	fmt.Printf("Handshake: %s", handshakeMsg)
	if err := conn.SendJSON(wshutils.CmdMsg{Type: "cmd", Cmd: handshakeMsg}); err != nil {
		return fmt.Errorf("failed to send handshake: %v", err)
	}

	// 2. 读取文件并编码
	fmt.Println("Encoding file...")
	encodedData, err := encodeFile(localFile)
	if err != nil {
		return fmt.Errorf("failed to encode file: %v", err)
	}

	// 3. 分块发送编码后的数据
	fmt.Println("Sending file data...")
	if err := sendEncodedData(conn, encodedData); err != nil {
		return fmt.Errorf("failed to send file data: %v", err)
	}

	// 4. 发送结束标记
	fmt.Println("Sending end marker...")
	fmt.Printf("End marker: %s\n", endMarker)
	if err := conn.SendJSON(wshutils.CmdMsg{Type: "cmd", Cmd: endMarker + "\n"}); err != nil {
		return fmt.Errorf("failed to send end marker: %v", err)
	}

	return nil
}

// encodeFile 编码文件
func encodeFile(localFile string) (string, error) {
	// 打开源文件
	sourceFile, err := os.Open(localFile)
	if err != nil {
		return "", fmt.Errorf("failed to open source file: %v", err)
	}
	defer sourceFile.Close()

	// 创建gzip压缩buffer
	var gzipBuffer bytes.Buffer
	gw := gzip.NewWriter(&gzipBuffer)

	// 复制文件内容到gzip压缩器
	if _, err := io.Copy(gw, sourceFile); err != nil {
		return "", fmt.Errorf("failed to compress file: %v", err)
	}

	// 关闭gzip writer
	if err := gw.Close(); err != nil {
		return "", fmt.Errorf("failed to close gzip writer: %v", err)
	}

	// 获取gzip压缩数据
	gzipData := gzipBuffer.Bytes()

	// 创建base64编码器
	var base64Buffer bytes.Buffer
	encoder := base64.NewEncoder(base64.StdEncoding, &base64Buffer)

	// 写入gzip数据到base64编码器
	if _, err := encoder.Write(gzipData); err != nil {
		return "", fmt.Errorf("failed to encode to base64: %v", err)
	}

	// 关闭编码器
	if err := encoder.Close(); err != nil {
		return "", fmt.Errorf("failed to close base64 encoder: %v", err)
	}

	return base64Buffer.String(), nil
}

// sendEncodedData 分块发送编码后的数据
func sendEncodedData(conn *wshutils.Connection, encodedData string) error {
	data := []byte(encodedData)
	totalChunks := (len(data) + chunkSize - 1) / chunkSize

	fmt.Printf("Total data size: %d bytes, will send in %d chunks\n", len(data), totalChunks)

	for i := 0; i < totalChunks; i++ {
		start := i * chunkSize
		end := start + chunkSize
		if end > len(data) {
			end = len(data)
		}

		chunk := data[start:end]
		chunkStr := string(chunk)
		fmt.Printf("Sending chunk %d/%d: %s\n", i+1, totalChunks, chunkStr)

		// 发送数据块
		if err := conn.SendJSON(wshutils.CmdMsg{Type: "cmd", Cmd: chunkStr + "\n"}); err != nil {
			return fmt.Errorf("failed to send chunk %d/%d: %v", i+1, totalChunks, err)
		}

		fmt.Printf("\rProgress: %d/%d chunks sent (chunk size: %d bytes)", i+1, totalChunks, len(chunk))
	}
	fmt.Println() // 换行

	return nil
}
