# Spotify Cover Downloader Bot

Telegram bot for downloading high-quality Spotify cover images.

## Features

- ✅ Download covers from tracks, albums, and playlists
- ✅ High-quality images (640x640)
- ✅ Parallel processing with worker pool
- ✅ Automatic retry on failures
- ✅ Respects Telegram and Spotify API limits
- ✅ Docker support

## Setup

### 1. Clone repository

\`\`\`bash
git clone repo
cd spotify-cover-bot
\`\`\`

### 2. Configure environment

\`\`\`bash
cp .env.example .env
\`\`\`

Edit `.env` and add your credentials:

- `TELEGRAM_BOT_TOKEN`: Get from @BotFather
- `SPOTIFY_CLIENT_ID` and `SPOTIFY_CLIENT_SECRET`: Get from [Spotify Developer Dashboard](https://developer.spotify.com/dashboard)

### 3. Run with Docker

\`\`\`bash
docker compose up -d
\`\`\`

### 4. Run locally

\`\`\`bash
go mod download
go run cmd/bot/main.go
\`\`\`

## Usage

1. Start bot: `/start`
2. Send any Spotify link
3. Receive cover images

## Configuration

See `.env.example` for all available options.

## License

MIT
\`\`\`

---

Теперь у вас **полноценный Telegram бот** с:

- ✅ Поддержкой треков/альбомов/плейлистов
- ✅ Параллельной загрузкой (100 воркеров)
- ✅ Соблюдением лимитов Telegram (10 фото в альбоме, 20MB на файл, 25 сообщений/сек)
- ✅ Retry механизмом
- ✅ Docker поддержкой
- ✅ Graceful shutdown
- ✅ `.gitignore` для защиты ключей

Запускайте! 🚀
