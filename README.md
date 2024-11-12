# Channel Monitor

[Chinese 中文](README-zh.md) | English 英文

## Introduction

Channel Monitor is a tool designed for monitoring OneAPI/NewAPI channels. It directly reads the channels data table and tests the availability of each channel's models at regular intervals. Based on whether the requests are successful, it updates the available models and writes them to the data table, thus achieving automated monitoring of the channels. This ensures high availability of the overall OneAPI/NewAPI and minimizes the number of error returns.

## Features

- [x] Directly read and write to the OneAPI/NewAPI database
- [x] Test the availability of each model in the channels
- [x] Automatically fetch available models from upstream
- [x] Automatically update the available models in the database for each channel
- [x] Support exclusion of channels and models from monitoring
- [x] Support configurable intervals
- [ ] TODO: Multi-threaded concurrent testing

## Installation

### Binary

Download the latest version of the binary file from the [Releases](https://github.com/DullJZ/ChannelMonitor/releases) page. After configuring `config.json` in the same directory, you can run it. It is recommended to use tools like `screen` or `nohup` to run it in the background.

```bash
mkdir ChannelMonitor && cd ChannelMonitor
wget https://github.com/DullJZ/ChannelMonitor/releases/download/v0.1.0/ChannelMonitor_linux_amd64
chmod +x ChannelMonitor_linux_amd64
# Download and modify the configuration file
wget https://raw.githubusercontent.com/DullJZ/ChannelMonitor/refs/heads/main/config_example.json -O config.json
nano config.json
screen -S ChannelMonitor
./ChannelMonitor_linux_amd64
```

### Docker

```bash
docker pull dulljz/channel-monitor
# Download and modify the configuration file
wget https://raw.githubusercontent.com/DullJZ/ChannelMonitor/refs/heads/main/config_example.json -O config.json
nano config.json
# If using the host's database, you can simply use the host mode,
# and use localhost:3306 as the database address
docker run -d --name ChannelMonitor -v ./config.json:/app/config.json --net host dulljz/channel-monitor
```

### Docker Compose

```yaml
version: '3'
services:
  channel-monitor:
    image: dulljz/channel-monitor
    volumes:
      - ./config.json:/app/config.json
    # If using the host's database, you can simply use the host mode,
    # and use localhost:3306 as the database address
    network_mode: host
```

```bash
# Download and modify the configuration file
wget https://raw.githubusercontent.com/DullJZ/ChannelMonitor/refs/heads/main/config_example.json -O config.json
nano config.json
docker-compose up -d
```

## Configuration

The configuration file is `config.json` located in the same directory, with the following format:

```json
{
    // Exclude channel IDs from monitoring
    "exclude_channel": [5],
    // Exclude model IDs from monitoring
    "exclude_model": ["advanced-voice"],
    // List of models, used only when the channel's models cannot be retrieved (/v1/models)
    "models": ["gpt-3.5-turbo", "gpt-3.5-turbo"],
    // Time interval for model availability testing, recommended to be no less than 30 minutes; accepted time formats are s, m, h
    "time_period": "1h",
    // Database DSN string, format: user:password@tcp(host:port)/database
    "db_dsn": "YOUR_DB_DSN"
}
