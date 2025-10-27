package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	// Telegram (Primary bot)
	TelegramBotToken string
	
	// Worker bots (for uploading to channel)
	WorkerBotTokens []string
	LogChannelID    int64

	// Spotify
	SpotifyClientID     string
	SpotifyClientSecret string

	// Workers
	WorkerPoolSize         int
	MaxConcurrentDownloads int
	ImageDownloadTimeout   time.Duration
	ProcessTimeout         time.Duration

	// Telegram Limits
	MaxAlbumSize          int
	MaxFileSizeMB         int
	MaxMessagesPerSecond  int

	// Inline Mode
	InlineCacheTime  int
	MaxInlineResults int

	// Debug
	Debug    bool
	LogLevel string

	AutoPlaylistID          string // ID плейлиста для автозаполнения
	SpotifyRefreshToken     string // Refresh token для OAuth
	EnableAutoPlaylist      bool   // Включить автоплейлист
}

func Load() *Config {
	cfg := &Config{
		TelegramBotToken:       os.Getenv("TELEGRAM_BOT_TOKEN"),
		LogChannelID:           getEnvInt64OrDefault("LOG_CHANNEL_ID", -1003065136240),
		SpotifyClientID:        os.Getenv("SPOTIFY_CLIENT_ID"),
		SpotifyClientSecret:    os.Getenv("SPOTIFY_CLIENT_SECRET"),
		WorkerPoolSize:         getEnvIntOrDefault("WORKER_POOL_SIZE", 100),
		MaxConcurrentDownloads: getEnvIntOrDefault("MAX_CONCURRENT_DOWNLOADS", 50),
		ImageDownloadTimeout:   time.Duration(getEnvIntOrDefault("IMAGE_DOWNLOAD_TIMEOUT_SEC", 15)) * time.Second,
		ProcessTimeout:         time.Duration(getEnvIntOrDefault("PROCESS_TIMEOUT_MIN", 30)) * time.Minute,
		MaxAlbumSize:           getEnvIntOrDefault("MAX_ALBUM_SIZE", 10),
		MaxFileSizeMB:          getEnvIntOrDefault("MAX_FILE_SIZE_MB", 20),
		MaxMessagesPerSecond:   getEnvIntOrDefault("MAX_MESSAGES_PER_SECOND", 15),
		InlineCacheTime:        getEnvIntOrDefault("INLINE_CACHE_TIME", 300),
		MaxInlineResults:       getEnvIntOrDefault("MAX_INLINE_RESULTS", 50),
		Debug:                  getEnvBoolOrDefault("DEBUG", false),
		LogLevel:               getEnvOrDefault("LOG_LEVEL", "info"),

		AutoPlaylistID:      os.Getenv("AUTO_PLAYLIST_ID"),
		SpotifyRefreshToken: os.Getenv("SPOTIFY_REFRESH_TOKEN"),
		EnableAutoPlaylist:  getEnvBoolOrDefault("ENABLE_AUTO_PLAYLIST", false),
	}

	// Load worker bot tokens
	workerTokensStr := os.Getenv("WORKER_BOT_TOKENS")
	if workerTokensStr != "" {
		cfg.WorkerBotTokens = strings.Split(workerTokensStr, ",")
		for i, token := range cfg.WorkerBotTokens {
			cfg.WorkerBotTokens[i] = strings.TrimSpace(token)
		}
	}

	return cfg
}

func (c *Config) Validate() error {
	if c.TelegramBotToken == "" {
		return fmt.Errorf("TELEGRAM_BOT_TOKEN is required")
	}
	if c.SpotifyClientID == "" || c.SpotifyClientSecret == "" {
		return fmt.Errorf("SPOTIFY_CLIENT_ID and SPOTIFY_CLIENT_SECRET are required")
	}
	return nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvInt64OrDefault(key string, defaultValue int64) int64 {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.ParseInt(value, 10, 64); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvBoolOrDefault(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolVal, err := strconv.ParseBool(value); err == nil {
			return boolVal
		}
	}
	return defaultValue
}
