package main

import (
	"os"
	"os/signal"
	"syscall"

	"image2spotify/internal/config"
	"image2spotify/internal/logger"
	"image2spotify/internal/processor"
	"image2spotify/internal/spotify"
	"image2spotify/internal/telegram"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog/log"
)

func main() {
	_ = godotenv.Load()

	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		log.Fatal().Err(err).Msg("Configuration error")
	}

	logger.Init(cfg.LogLevel, cfg.Debug)
	log.Info().Msg("Starting Spotify Cover Bot")

	spotifyClient := spotify.NewClient(cfg.SpotifyClientID, cfg.SpotifyClientSecret)

	// Автоматическая инициализация auth для playlist
	var playlistManager *spotify.PlaylistManager
	if cfg.EnableAutoPlaylist && cfg.AutoPlaylistID != "" {
		refreshToken, err := spotify.InitializeAuth(cfg.SpotifyClientID, cfg.SpotifyClientSecret)
		if err != nil {
			log.Warn().Err(err).Msg("Auto-playlist disabled: no refresh token")
			cfg.EnableAutoPlaylist = false
		} else {
			playlistManager = spotify.NewPlaylistManager(
				cfg.SpotifyClientID,
				cfg.SpotifyClientSecret,
				refreshToken,
			)
			log.Info().Str("playlist_id", cfg.AutoPlaylistID).Msg("Auto-playlist enabled")
		}
	}

	proc := processor.NewProcessor(
		spotifyClient,
		playlistManager,
		cfg.AutoPlaylistID,
		cfg.EnableAutoPlaylist,
		cfg.WorkerPoolSize,
		cfg.ImageDownloadTimeout,
		cfg.ProcessTimeout,
	)

	bot, err := telegram.NewBot(cfg, proc)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create bot")
	}

	log.Info().
		Int("workers", cfg.WorkerPoolSize).
		Int("max_concurrent", cfg.MaxConcurrentDownloads).
		Dur("image_timeout", cfg.ImageDownloadTimeout).
		Dur("process_timeout", cfg.ProcessTimeout).
		Int("max_album_size", cfg.MaxAlbumSize).
		Int("max_file_size_mb", cfg.MaxFileSizeMB).
		Bool("debug", cfg.Debug).
		Bool("auto_playlist", cfg.EnableAutoPlaylist).
		Msg("Configuration loaded")

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigs
		log.Info().Msg("Received shutdown signal")
		bot.Stop()
		os.Exit(0)
	}()

	bot.Start()
}
