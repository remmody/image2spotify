# 🎵 Spotify Cover Downloader Bot

High-performance Telegram bot for downloading Spotify cover artwork with advanced features like worker pools, inline mode, and auto-playlist integration.

[![Go Version](https://img.shields.io/badge/Go-1.25.1-00ADD8?style=flat&logo=go)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Telegram](https://img.shields.io/badge/Telegram-Bot-26A5E4?style=flat&logo=telegram)](https://telegram.org/)

## ✨ Features

- 🎨 **High-Quality Images** - Download cover art in 640x640 resolution
- 🚀 **Parallel Processing** - 100 concurrent workers for fast downloads
- 📦 **Batch Support** - Handle entire playlists (no limits)
- ⚡ **Real-Time Streaming** - Images sent as they download
- 🔄 **FloodWait Protection** - 20 worker bots for anti-flood bypass
- 🎯 **Inline Mode** - Quick access via `@botname spotify_url`
- 📊 **Auto-Playlist** - Automatically add processed tracks to your Spotify playlist
- 🔧 **Smart Retry** - Automatic retry with exponential backoff
- 📝 **Structured Logging** - Zerolog for production-ready logs
- 🐳 **Docker Ready** - One-command deployment

## 📋 Table of Contents

- [Requirements](#requirements)
- [Installation](#installation)
  - [Docker (Recommended)](#docker-recommended)
  - [Local Setup](#local-setup)
- [Configuration](#configuration)
- [Usage](#usage)
- [Advanced Features](#advanced-features)
- [Project Structure](#project-structure)
- [Development](#development)
- [Troubleshooting](#troubleshooting)
- [License](#license)

## 🔧 Requirements

- **Go** 1.25.1 or higher
- **Docker** (optional, for containerized deployment)
- **Telegram Bot Token** from [@BotFather](https://t.me/BotFather)
- **Spotify API Credentials** from [Spotify Developer Dashboard](https://developer.spotify.com/dashboard)

## 📦 Installation

### Docker (Recommended)

1. **Clone the repository**

```

git clone https://github.com/remmody/image2spotify.git
cd image2spotify

```

2. **Configure environment**

```

cp .env.example .env
nano .env  \# Edit with your credentials

```

3. **Build and run**

```

docker compose up -d

```

4. **View logs**

```

docker compose logs -f

```

### Local Setup

1. **Clone and install dependencies**

```

git clone https://github.com/remmody/image2spotify.git
cd image2spotify
go mod download

```

2. **Configure environment**

```

cp .env.example .env
nano .env  \# Add your credentials

```

3. **Setup auto-playlist (optional)**

```


# First-time authorization for Spotify playlist management

go run cmd/auth/main.go

```

Follow the browser prompt to authorize. The refresh token will be saved automatically.

4. **Run the bot**

```

go run cmd/bot/main.go

```

## ⚙️ Configuration

### Basic Setup

Edit `.env` with your credentials:

```


# Telegram Bot

TELEGRAM_BOT_TOKEN=your_bot_token_from_botfather

# Spotify API

SPOTIFY_CLIENT_ID=your_spotify_client_id
SPOTIFY_CLIENT_SECRET=your_spotify_client_secret

# Log Channel (for caching covers)

LOG_CHANNEL_ID=-1001234567890

```

### Advanced Configuration

```


# Worker Pool Settings

WORKER_POOL_SIZE=100              \# Parallel download workers
MAX_CONCURRENT_DOWNLOADS=50       \# Concurrent HTTP connections
IMAGE_DOWNLOAD_TIMEOUT_SEC=15     \# Timeout per image
PROCESS_TIMEOUT_MIN=30            \# Total processing timeout

# Telegram Limits

MAX_ALBUM_SIZE=10                 \# Photos per album
MAX_FILE_SIZE_MB=20               \# Max file size
MAX_MESSAGES_PER_SECOND=15        \# Rate limit

# Worker Bots (Anti-FloodWait)

WORKER_BOT_TOKENS=token1,token2,token3,...  \# Up to 20 tokens

# Auto-Playlist Feature

ENABLE_AUTO_PLAYLIST=true
AUTO_PLAYLIST_ID=your_playlist_id
SPOTIFY_REFRESH_TOKEN=auto_generated

# Inline Mode

INLINE_CACHE_TIME=300
MAX_INLINE_RESULTS=50

# Logging

DEBUG=false
LOG_LEVEL=info                    \# debug, info, warn, error

```

### Worker Bots Setup

To bypass Telegram FloodWait (429 errors), create multiple bot tokens:

1. Create 5-20 bots via [@BotFather](https://t.me/BotFather) using `/newbot`
2. Add all bots as **administrators** to your log channel with "Post messages" permission
3. Add tokens to `WORKER_BOT_TOKENS` (comma-separated)

**Benefits:**

- Single bot: ~20 uploads/minute
- 20 worker bots: ~400 uploads/minute

## 🚀 Usage

### Basic Commands

- `/start` or `/help` - Show welcome message
- Send any Spotify link - Get cover images

### Supported Link Types

```

✅ Track: https://open.spotify.com/track/3n3Ppam7vgaVa1iaRUc9Lp
✅ Album: https://open.spotify.com/album/6JWc4iAiJ9FjjkqcbRdMPc
✅ Playlist: https://open.spotify.com/playlist/37i9dQZF1DXcBWIGoYBM5M

```

### Inline Mode

Use the bot in any chat without opening DM:

```

@your_bot_username https://open.spotify.com/track/...

```

Select the cover image from results (max 50) to send it instantly.

## 🎯 Advanced Features

### Auto-Playlist Integration

Automatically add all processed tracks to your Spotify playlist:

**1. Create a playlist on Spotify**

Go to [open.spotify.com](https://open.spotify.com/) and create a new playlist.

**2. Get Playlist ID**

From playlist URL: `https://open.spotify.com/playlist/2RlUagSNQ8eJ3P9hJQ4nxX`

The ID is: `2RlUagSNQ8eJ3P9hJQ4nxX`

**3. Configure `.env`:**

```

ENABLE_AUTO_PLAYLIST=true
AUTO_PLAYLIST_ID=2RlUagSNQ8eJ3P9hJQ4nxX

```

**4. Authorize once:**

```

go run cmd/auth/main.go

```

**Important:** In Spotify Dashboard, set Redirect URI to `http://127.0.0.1:8888/callback` (NOT `localhost`)

**5. Tracks auto-added!**

Every track/album/playlist you process will be added to your Spotify playlist automatically. Duplicates are automatically filtered.

### Real-Time Streaming

Images are sent **as they download**, not after full completion:

- ✅ Instant feedback to users
- ✅ Better UX for large playlists (300+ tracks)
- ✅ Progress updates every 3 seconds
- ✅ No memory spikes from buffering

### FloodWait Protection System

The bot uses multiple worker bots to distribute upload load:

- **Round-robin balancing** across all workers
- **Automatic failover** when a bot hits rate limit
- **Dynamic backoff** with parsed retry delays
- **Health tracking** - skips bots with 3+ consecutive failures

**Architecture:**

```

User Request → Download Images → Worker Bot Pool → Log Channel (FileID cache) → User
↓
20 bots rotate
(Bot 1, Bot 2, ... Bot 20)

```

## 📁 Project Structure

```

image2spotify/
├── cmd/
│   ├── bot/
│   │   └── main.go              \# Bot entry point
│   └── auth/
│       └── main.go              \# OAuth authorization tool
├── internal/
│   ├── config/
│   │   └── config.go            \# Configuration management
│   ├── logger/
│   │   └── logger.go            \# Zerolog setup
│   ├── processor/
│   │   ├── processor.go         \# Main processing logic
│   │   └── worker_pool.go       \# Worker pool implementation
│   ├── spotify/
│   │   ├── auth.go              \# OAuth helper
│   │   ├── client.go            \# Spotify API client
│   │   ├── downloader.go        \# Image downloader
│   │   ├── playlist_manager.go  \# Playlist operations
│   │   ├── types.go             \# Data structures
│   │   └── utils.go             \# Utility functions
│   └── telegram/
│       ├── bot.go               \# Bot initialization
│       ├── handlers.go          \# Message handlers
│       └── sender.go            \# Image sender with worker pool
├── .env.example                 \# Environment template
├── .gitignore
├── .spotify_token_cache.json    \# Auto-generated (gitignored)
├── docker-compose.yml
├── Dockerfile
├── go.mod
├── go.sum
├── LICENSE
└── README.md

```

## 🛠️ Development

### Running Tests

```

go test ./...

```

### Build Binary

```

go build -o bot cmd/bot/main.go
./bot

```

### Docker Build

```

docker build -t image2spotify .
docker run --env-file .env image2spotify

```

### Code Standards

- **Go Version:** 1.25.1
- **Logging:** Zerolog (structured JSON logs)
- **Error Handling:** Wrapped errors with context
- **Concurrency:** Context-aware with timeouts
- **Rate Limiting:** Per-bot mutex + global coordination

## 🐛 Troubleshooting

### Common Issues

**1. "INVALID_CLIENT: Invalid redirect URI"**

**Solution:** In [Spotify Dashboard](https://developer.spotify.com/dashboard):

- Add Redirect URI: `http://127.0.0.1:8888/callback`
- **NOT** `http://localhost:8888/callback` ❌
- Click **Save** at the bottom

**2. "telegram: retry after X (429)" - FloodWait errors**

**Solution:**

- Add more worker bot tokens to `WORKER_BOT_TOKENS` in `.env`
- Ensure all worker bots are **administrators** in log channel
- Reduce `MAX_MESSAGES_PER_SECOND` to 10-15

**3. "Failed to get tracks: 401 Unauthorized"**

**Solution:**

- Spotify credentials expired - run `go run cmd/auth/main.go` again
- Check `SPOTIFY_CLIENT_ID` and `SPOTIFY_CLIENT_SECRET` are correct

**4. Bot not responding**

**Check logs:**

```


# Docker

docker compose logs -f

# Local

tail -f bot.log

```

**Common causes:**

- Invalid `TELEGRAM_BOT_TOKEN`
- Bot not started (`/start` command)
- Firewall blocking Telegram API

**5. Images not uploading to channel**

**Ensure:**

- All worker bots are added as **administrators** to log channel
- Log channel ID starts with `-100` (e.g., `-1001234567890`)
- Bots have "Post messages" permission

**6. "file must be non-empty (400)"**

This was solved by switching from album uploads to individual photo uploads. If you see this, ensure you're running the latest version.

**7. Inline mode not working**

**Enable inline mode:**

1. Go to [@BotFather](https://t.me/BotFather)
2. Select your bot → `/setinline`
3. Set placeholder text: "Paste Spotify link..."

### Debug Mode

Enable detailed logging:

```

DEBUG=true
LOG_LEVEL=debug

```

Then check logs for detailed execution traces.

## 📊 Performance

**System Requirements:**

- **CPU:** 1 core minimum (I/O bound)
- **RAM:** 50-100MB under load
- **Network:** Stable connection for Telegram + Spotify APIs

**Throughput:**

- **Processing:** ~100 tracks/minute
- **Upload (1 bot):** ~20 images/minute
- **Upload (20 bots):** ~400 images/minute
- **Download:** Limited by Spotify CDN (~200ms/image)

**Tested Scale:**

- ✅ Single playlist: 392 tracks → ~4 minutes
- ✅ Multiple users: 10 concurrent requests
- ✅ Uptime: 72+ hours continuous operation

## 🤝 Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing`)
5. Open a Pull Request

**Areas for contribution:**

- Additional music services (Apple Music, Deezer, etc.)
- Web UI for configuration
- Statistics/analytics dashboard
- Multi-language support

## 📝 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- [Spotify Web API](https://developer.spotify.com/documentation/web-api) - Music metadata
- [Telebot v4](https://github.com/tucnak/telebot) - Telegram Bot framework
- [Zerolog](https://github.com/rs/zerolog) - Structured logging

## 📧 Support

- **Issues:** [GitHub Issues](https://github.com/remmody/image2spotify/issues)
- **Discussions:** [GitHub Discussions](https://github.com/remmody/image2spotify/discussions)

---

**Made with ❤️ and Go 1.25.1**

_Star ⭐ this repo if you find it useful!_
