# WSH - WebSocket Shell

WSH 是一个基于 WebSocket 的远程 Shell 客户端，允许您通过 WebSocket 连接到远程 Shell 服务器。

## 功能特性

- 🔌 基于 WebSocket 的远程 Shell 连接
- 📝 支持配置文件管理多个连接端点
- 💓 智能心跳机制保持连接稳定
- 🖥️ 自动终端大小调整
- 🎯 支持 F12 快捷键退出连接
- 📊 详细的日志记录

## 系统要求

- Go 1.23.0 或更高版本
- 支持的操作系统：Linux、macOS、Windows

## 安装指南

### 方法一：从源码构建（推荐）

1. **克隆项目**
   ```bash
   git clone https://github.com/gitchs/wsh.git
   cd wsh
   ```

2. **构建项目**
   ```bash
   # 构建所有组件
   make all
   
   # 或者单独构建 wsh
   make wsh
   
   # 或者单独构建 wcp
   make wcp
   ```

3. **安装到系统（可选）**
   ```bash
   # 安装到 /usr/local/bin/
   sudo make install
   
   # 卸载
   sudo make uninstall
   ```

### 方法二：直接使用 Go 命令

```bash
# 克隆项目
git clone https://github.com/gitchs/wsh.git
cd wsh

# 构建 wsh
go build -o wsh/wsh wsh/main.go

# 构建 wcp
go build -o wcp/wcp wcp/main.go
```

## 配置文件设置

WSH 使用 YAML 格式的配置文件来管理连接端点。默认配置文件位置：`~/.config/wsh.yaml`

### 创建配置文件

1. **创建配置目录**
   ```bash
   mkdir -p ~/.config
   ```

2. **创建配置文件**
   ```bash
   cat > ~/.config/wsh.yaml << 'EOF'
   endpoints:
     - name: "server1"
       url: "ws://your-server:8080/ws"
       description: "生产服务器"
     
     - name: "server2"
       url: "wss://your-secure-server:8443/ws"
       description: "安全服务器"
     
     - name: "local"
       url: "ws://localhost:8080/ws"
       description: "本地开发服务器"
   EOF
   ```

### 配置文件格式说明

```yaml
endpoints:
  - name: "端点名称"           # 用于连接时指定的名称
    url: "WebSocket URL"      # WebSocket 连接地址
    description: "描述信息"    # 端点的描述信息
```

## 使用方法

### 基本用法

1. **查看可用端点**
   ```bash
   ./wsh/wsh
   ```

2. **连接到预定义端点**
   ```bash
   ./wsh/wsh server1
   ```

3. **直接连接 WebSocket URL**
   ```bash
   ./wsh/wsh ws://your-server:8080/ws
   ```

### 高级选项

```bash
# 使用自定义配置文件
./wsh/wsh -c /path/to/config.yaml server1

# 设置心跳间隔（秒）
./wsh/wsh --heartbeat-interval 30 server1

# 查看帮助
./wsh/wsh --help
```

### 快捷键操作

- **F12**: 退出连接并关闭程序
- **Ctrl+C**: 发送中断信号到远程 Shell
- **窗口大小调整**: 自动同步终端大小到远程服务器

## 项目结构

```
wsh/
├── wsh/           # 主程序目录
│   └── main.go    # WSH 客户端主程序
├── wcp/           # WCP 程序目录
│   └── main.go    # WCP 程序
├── wshutils/      # 工具库
│   └── connection.go
├── go.mod         # Go 模块文件
├── go.sum         # Go 依赖校验文件
├── Makefile       # 构建脚本
└── README.md      # 项目说明文档
```

## 依赖项

- `github.com/gorilla/websocket` - WebSocket 客户端库
- `github.com/sirupsen/logrus` - 日志库
- `github.com/spf13/cobra` - 命令行界面库
- `golang.org/x/term` - 终端操作库
- `gopkg.in/yaml.v3` - YAML 解析库

## 故障排除

### 常见问题

1. **连接失败**
   - 检查 WebSocket URL 是否正确
   - 确认服务器是否正在运行
   - 检查网络连接和防火墙设置

2. **配置文件错误**
   - 确保 YAML 格式正确
   - 检查文件权限
   - 验证配置文件路径

3. **终端显示异常**
   - 程序会自动重置终端状态
   - 如果仍有问题，可以手动运行 `reset` 命令

### 日志文件

程序运行时会生成日志文件：`/tmp/wsh-{PID}.txt`

## 开发

### 构建开发版本

```bash
# 清理构建文件
make clean

# 重新构建
make all
```

### 运行测试

```bash
go test ./...
```

## 许可证

本项目采用 MIT 许可证。详见 LICENSE 文件。

## 贡献

欢迎提交 Issue 和 Pull Request！

## 更新日志

- 支持配置文件管理多个端点
- 添加智能心跳机制
- 支持终端大小自动调整
- 添加 F12 快捷键退出功能
- 改进日志记录和错误处理
