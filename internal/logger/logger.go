package logger

import (
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func Init(level string, debug bool) {
	// Console writer with colors
	consoleWriter := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: "15:04:05",
		NoColor:    false,
	}

	var writers []io.Writer
	writers = append(writers, consoleWriter)

	// File writer for production
	if !debug {
		logFile, err := os.OpenFile(
			"bot.log",
			os.O_APPEND|os.O_CREATE|os.O_WRONLY,
			0644,
		)
		if err == nil {
			writers = append(writers, logFile)
		}
	}

	multi := zerolog.MultiLevelWriter(writers...)

	logger := zerolog.New(multi).With().Timestamp().Logger()

	// Set global logger
	log.Logger = logger

	// Parse and set log level
	parsedLevel, err := zerolog.ParseLevel(level)
	if err != nil {
		parsedLevel = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(parsedLevel)

	// Development mode settings
	if debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
		log.Logger = log.With().Caller().Logger()
	}

	zerolog.TimeFieldFormat = time.RFC3339
}

func Get() *zerolog.Logger {
	return &log.Logger
}
