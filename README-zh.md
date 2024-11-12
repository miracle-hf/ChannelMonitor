# Channel Monitor 渠道监控

Chinese 中文 | [English 英文](README.md)

## 介绍

Channel Monitor 是一个用于监控OneAPI/NewAPI渠道的工具，它直接读取channels数据表，每间隔一段时间测试每个渠道的模型可用性，根据请求是否成功更新可用模型，写入到数据表中，以此来实现渠道的自动监控，保证整体OneAPI/NewAPI的高可用，尽可能减少错误返回次数。

## 特性

- [x] 直接读写OneAPI/NewAPI的数据库
- [x] 测试渠道的每个模型可用性
- [x] 自动向上游获取可用模型
- [x] 自动更新数据库中的每个渠道可用模型
- [x] 支持排除不予监控的渠道和模型
- [x] 支持间隔时间配置
- [ ] TODO: 多线程并发测试


## 安装

### 二进制

从[Releases](https://github.com/DullJZ/ChannelMonitor/releases)页面下载最新版本的二进制文件，在同一目录下配置`config.json`后运行即可。建议使用`screen`或`nohup`等工具后台运行。

```bash
mkdir ChannelMonitor && cd ChannelMonitor
wget https://github.com/DullJZ/ChannelMonitor/releases/download/v0.1.0/ChannelMonitor_linux_amd64
chmod +x ChannelMonitor_linux_amd64
# 下载并修改配置文件
wget https://raw.githubusercontent.com/DullJZ/ChannelMonitor/refs/heads/main/config_example.json -O config.json
nano config.json
screen -S ChannelMonitor
./ChannelMonitor_linux_amd64
```

### Docker

```bash
docker pull dulljz/channel-monitor
# 下载并修改配置文件
wget https://raw.githubusercontent.com/DullJZ/ChannelMonitor/refs/heads/main/config_example.json -O config.json
nano config.json
# 如果使用宿主机的数据库，可以简单使用host模式，
# 并使用localhost:3306作为数据库地址
docker run -d --name ChannelMonitor -v ./config.json:/app/config.json -net host dulljz/channel-monitor
```

### Docker Compose

```yaml
version: '3'
services:
  channel-monitor:
    image: dulljz/channel-monitor
    volumes:
      - ./config.json:/app/config.json
    # 如果使用宿主机的数据库，可以简单使用host模式，
    # 并使用localhost:3306作为数据库地址
    network_mode: host
```

```bash
# 下载并修改配置文件
wget https://raw.githubusercontent.com/DullJZ/ChannelMonitor/refs/heads/main/config_example.json -O config.json
nano config.json
docker-compose up -d
```

## 配置

配置文件使用同级目录下的`config.json`文件，格式如下：

```json
{
    // 排除不予监控的渠道ID
    "exclude_channel": [5],
    // 排除不予监控的模型ID
    "exclude_model": ["advanced-voice"],
    // 模型列表，仅当获取不到渠道的模型(/v1/models)时使用
    "models": ["gpt-3.5-turbo", "gpt-3.5-turbo"],
    // 模型可用性测试的时间间隔，建议不小于30分钟，接收的时间格式为s、m、h
    "time_period": "1h",
    // 数据库DSN字符串，格式为：user:password@tcp(host:port)/database
    "db_dsn": "YOUR_DB_DSN"
}
```