# Botsu [![Go](https://github.com/xoltia/botsu/actions/workflows/go.yml/badge.svg)](https://github.com/xoltia/botsu/actions/workflows/go.yml) [![Go Report Card](https://goreportcard.com/badge/github.com/xoltia/botsu)](https://goreportcard.com/report/github.com/xoltia/botsu)

Botsu is a Discord bot for logging time spent on immersion activities in Japanese.
It is the successor to [AnotherImmersionBot](https://github.com/UTD-JLA/another-immersion-bot),
which itself is an inspiration of TheMoeWay's [ImmersionBot](https://github.com/TheMoeWay/immersion-bot).

## Requirements

- PostgreSQL 15
- Go 1.21 (for running from source)
- Docker (for running from Docker image)

## Install
To install the latest release from the terminal, use:
```sh
curl -L https://github.com/xoltia/botsu/releases/latest/download/botsu-linux-amd64 -o ~/.local/bin/botsu
chmod a+rx ~/.local/bin/botsu
```
replacing `~/.local/bin` with your preferred install location. 

To install from source, use:
```
go install github.com/xoltia/botsu/cmd/botsu@latest
```
which will install to your `GOBIN` (`~/go/bin` by default).

## Basic Setup

1. Create a Discord bot account and invite it to your server.
2. Create a new PostgreSQL database, for example `botsu`.
3. Create a `config.toml` file in your working directory with the following contents:
```toml
token = "your bot token"
# This requires the privileged members intent, which can be enabled in the Discord developer portal.
use_members_intent = true

[database]
user = "your database user"
password = "your database password"
host = "your database host"
port = 5432
database = "botsu"
```
4. Run `botsu` from the working directory.
