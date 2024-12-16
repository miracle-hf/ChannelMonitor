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
- [x] Support multiple database types, including MySQL, SQLite, PostgreSQL, and SQL Server
- [x] Concurrent testing

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
# If using an SQLite database, mount the database file
# docker run -d --name ChannelMonitor -v ./config.json:/app/config.json -v /path/to/database.db:/app/database.db dulljz/channel-monitor
```

### Docker Compose

```yaml
version: '3'
services:
  channel-monitor:
    image: dulljz/channel-monitor
    volumes:
      - ./config.json:/app/config.json
      # If using an SQLite database, mount the database file
      # - /path/to/database.db:/app/database.db
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
  "oneapi_type": "oneapi",
  "exclude_channel": [5],
  "exclude_model": ["advanced-voice"],
  "models": ["gpt-3.5-turbo", "gpt-4"],
  "force_models": false,
  "time_period": "1h",
  "db_type": "mysql",
  "db_dsn": "YOUR_DB_DSN",
  "base_url": "http://localhost:3000",
  "system_token": ""
}
```

Configuration explanation:
- oneapi_type: Type of OneAPI, including oneapi, newapi, onehub (reserved field, currently has no effect)
- exclude_channel: IDs of channels to exclude from monitoring
- exclude_model: IDs of models to exclude from monitoring
- models: List of models, used only when unable to retrieve models from the channel (/v1/models)
- force_models: If true, only the above models will be tested, and channel models will not be fetched. Default is false
- time_period: Interval for testing model availability, recommended not less than 30 minutes, accepts time formats s, m, h
- db_type: Database type, including mysql, sqlite, postgres, sqlserver
- db_dsn: Database DSN string, the format varies by database type. Examples below
- base_url: The base URL for OneAPI/NewAPI/OneHub. If using host mode, you can directly use http://localhost:3000. Currently, only OneHub requires this field.
- system_token: System token, currently only required for OneHub.

### MySQL

```json
{
  "db_type": "mysql",
  "db_dsn": "username:password@tcp(host:port)/dbname"
}
```

### SQLite

```json
{
  "db_type": "sqlite",
  "db_dsn": "/path/to/database.db"
}
```

### PostgreSQL

```json
{
  "db_type": "postgres",
  "db_dsn": "host=host port=port user=username password=password dbname=dbname sslmode=disable"
}
```

### SQL Server

```json
{
  "db_type": "sqlserver",
  "db_dsn": "sqlserver://username:password@host:port?database=dbname"
}
```