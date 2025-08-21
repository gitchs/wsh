# WSH - WebSocket Shell Client

一个简单的WebSocket终端客户端，支持通过配置文件预定义连接端点。

## 功能特性

- 支持通过配置文件预定义WebSocket端点
- 支持通过端点名称快速连接
- 支持直接使用WebSocket URL连接
- 终端大小自适应
- 心跳保活机制

## 使用方法

### 1. 使用预定义端点

```bash
# 连接名为 "dev" 的端点
./wsh dev

# 连接名为 "prod" 的端点
./wsh prod
```

### 2. 使用自定义WebSocket URL

```bash
# 直接连接WebSocket URL
./wsh wss://example.com/shell
```

### 3. 查看可用端点

```bash
# 不带参数运行，显示所有可用端点
./wsh
```

## 配置文件

程序使用 `~/.config/wsh.yaml` 文件来定义端点。配置文件格式如下：

```yaml
endpoints:
  - name: dev
    url: wss://dev.example.com/terminal
    description: "开发环境"
  - name: prod
    url: wss://prod.example.com/ws
    description: "生产环境"
```

### 配置项说明

- `name`: 端点名称，用于命令行参数
- `url`: WebSocket连接地址
- `description`: 端点描述，用于帮助信息显示

### 自定义配置文件

```bash
# 使用自定义配置文件
./wsh -c /path/to/config.yaml dev
```

## 快捷键

- `Ctrl+C`: 发送中断信号
- `F12`: 断开连接并退出

> **注意**: 由于服务端可能不会正常处理退出信号，客户端可能无法正常退出。此时需要使用 `F12` 强制断开连接并退出程序。

## 构建

```bash
go build -o wsh main.go
```

## 依赖

- `github.com/gorilla/websocket` - WebSocket客户端
- `github.com/sirupsen/logrus` - 日志库
- `golang.org/x/term` - 终端控制
- `gopkg.in/yaml.v3` - YAML配置文件解析
