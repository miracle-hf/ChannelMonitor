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
