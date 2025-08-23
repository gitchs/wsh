wcp 是一个小文件传输命令

**设计目标**: wcp专为小文件传输设计，默认限制文件大小为32KB。对于大文件传输，建议使用其他工具如scp、rsync等。

通过跟wsh相同的连接方式，连接上目标。
目标是一个shell，可以执行cat, base64, gunzip 等命令

**文件大小限制**:
- 默认最大文件大小: 32KB
- 超过限制时需要使用 `--force` 参数强制传输
- 大文件传输会显示警告信息

wcp工作流程是
1. 通过wsh相同的机制，连上目标
2. 检查文件大小，超过32KB且未使用--force时拒绝传输
3. 开始传输，发送下面的握手消息
```bash
cat <<'__EOF' |base64 --decode |gunzip > filename
```
4. 开启文件传输的时候，模拟这个命令 `gzip filename | base64`
5. 编码后的文件，每256字节发送1条消息（最后一条消息可以少于256字节）
6. 文件编码发送完成后，发送终止__EOF，然后等待响应后退出。
7. 重构wsh，将公共代码剥离出来，放到wshutils目录

**使用示例**:
```bash
# 传输小文件（<32KB）
wcp endpoint-name config.txt

# 强制传输大文件（>32KB）
wcp --force endpoint-name large-file.txt

# 使用自定义配置文件
wcp -c /path/to/config.yaml endpoint-name file.txt
```