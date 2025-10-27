package telegram

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"image2spotify/internal/spotify"

	"github.com/rs/zerolog/log"
	tele "gopkg.in/telebot.v4"
)

type BotWorker struct {
	bot          *tele.Bot
	lastSendTime time.Time
	mu           sync.Mutex
	failures     int32
}

type Sender struct {
	primaryBot      *tele.Bot
	workerBots      []*BotWorker
	currentWorker   int32
	maxFileSizeMB   int
	messageInterval time.Duration
	logChannelID    int64
	globalMu        sync.Mutex
}

func NewSender(primaryBot *tele.Bot, workerBotTokens []string, maxAlbumSize, maxFileSizeMB, maxMessagesPerSecond int, logChannelID int64) *Sender {
	s := &Sender{
		primaryBot:      primaryBot,
		workerBots:      make([]*BotWorker, 0, len(workerBotTokens)),
		maxFileSizeMB:   maxFileSizeMB,
		messageInterval: time.Second / time.Duration(maxMessagesPerSecond),
		logChannelID:    logChannelID,
	}

	// Initialize worker bots
	for i, token := range workerBotTokens {
		if token == "" {
			continue
		}

		pref := tele.Settings{
			Token:  token,
			Poller: &tele.LongPoller{Timeout: 10 * time.Second},
		}

		bot, err := tele.NewBot(pref)
		if err != nil {
			log.Error().Err(err).Int("worker_index", i).Msg("Failed to create worker bot")
			continue
		}

		s.workerBots = append(s.workerBots, &BotWorker{
			bot: bot,
		})
		log.Info().Int("worker_id", i).Msg("Worker bot initialized")
	}

	if len(s.workerBots) == 0 {
		log.Warn().Msg("No worker bots available, using primary bot only")
	} else {
		log.Info().Int("worker_count", len(s.workerBots)).Msg("Worker bots pool ready")
	}

	return s
}

// getNextWorker returns the next available worker bot using round-robin
func (s *Sender) getNextWorker() *BotWorker {
	if len(s.workerBots) == 0 {
		return nil
	}

	// Round-robin с учётом failures
	attempts := len(s.workerBots) * 2
	for i := 0; i < attempts; i++ {
		idx := int(atomic.AddInt32(&s.currentWorker, 1)) % len(s.workerBots)
		worker := s.workerBots[idx]
		
		// Пропускаем воркеров с большим количеством ошибок
		if atomic.LoadInt32(&worker.failures) < 3 {
			return worker
		}
	}

	// Если все воркеры с ошибками, сбрасываем счётчики
	for _, worker := range s.workerBots {
		atomic.StoreInt32(&worker.failures, 0)
	}

	return s.workerBots[0]
}

// StreamImage отправляет одно изображение сразу в канал и пользователю
func (s *Sender) StreamImage(chatID int64, username string, img *spotify.ImageData, index, total int) error {
	maxFileSize := int64(s.maxFileSizeMB * 1024 * 1024)
	if int64(len(img.Data)) > maxFileSize {
		log.Debug().Str("track_id", img.TrackID).Int("size", len(img.Data)).Msg("Image exceeds size limit")
		return nil
	}

	if len(img.Data) == 0 {
		log.Debug().Str("track_id", img.TrackID).Msg("Empty image data")
		return nil
	}

	var fileID string
	maxRetries := 3

	// 1. Отправляем в лог-канал через worker bots (если доступны)
	if s.logChannelID != 0 {
		logChannel := &tele.Chat{ID: s.logChannelID}

		for retry := 0; retry < maxRetries; retry++ {
			worker := s.getNextWorker()
			var bot *tele.Bot
			
			if worker != nil {
				bot = worker.bot
				
				// Rate limiting для worker
				worker.mu.Lock()
				elapsed := time.Since(worker.lastSendTime)
				if elapsed < s.messageInterval {
					time.Sleep(s.messageInterval - elapsed)
				}
				worker.lastSendTime = time.Now()
				worker.mu.Unlock()
			} else {
				// Fallback to primary bot
				bot = s.primaryBot
				
				s.globalMu.Lock()
				time.Sleep(s.messageInterval)
				s.globalMu.Unlock()
			}

			// Отправляем БЕЗ caption
			photo := &tele.Photo{
				File: tele.FromReader(bytes.NewReader(img.Data)),
			}

			sent, err := bot.Send(logChannel, photo)
			if err == nil {
				if sent.Photo != nil && sent.Photo.FileID != "" {
					fileID = sent.Photo.FileID
					
					// Сбрасываем счётчик ошибок при успехе
					if worker != nil {
						atomic.StoreInt32(&worker.failures, 0)
					}
					
					log.Debug().
						Str("track_id", img.TrackID).
						Int("index", index).
						Str("file_id", fileID).
						Msg("Uploaded to log channel")
				}
				break
			}

			// Увеличиваем счётчик ошибок
			if worker != nil {
				atomic.AddInt32(&worker.failures, 1)
			}

			// Обработка FloodWait
			if strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "retry after") {
				waitTime := s.parseRetryAfter(err.Error())
				if waitTime == 0 {
					waitTime = time.Duration(retry+1) * 3 * time.Second
				}
				
				log.Debug().
					Err(err).
					Int("retry", retry+1).
					Dur("wait_time", waitTime).
					Msg("FloodWait on log channel, switching worker")
				
				// При FloodWait сразу переключаемся на другого воркера
				time.Sleep(1 * time.Second)
				continue
			}

			log.Error().Err(err).Int("retry", retry+1).Msg("Failed to send to log channel")
			time.Sleep(time.Duration(retry+1) * time.Second)
		}
	}

	// 2. Отправляем пользователю (через FileID если есть, иначе загружаем заново)
	recipient := &tele.User{ID: chatID}

	for retry := 0; retry < maxRetries; retry++ {
		var sent *tele.Message
		var err error

		// Rate limiting для primary bot
		s.globalMu.Lock()
		time.Sleep(s.messageInterval)
		s.globalMu.Unlock()

		if fileID != "" {
			// Отправляем через FileID (быстро)
			photo := &tele.Photo{
				File: tele.File{FileID: fileID},
			}
			if index%10 == 1 {
				photo.Caption = fmt.Sprintf("%d/%d", index, total)
			}
			sent, err = s.primaryBot.Send(recipient, photo)
		} else {
			// Загружаем заново если нет FileID
			photo := &tele.Photo{
				File: tele.FromReader(bytes.NewReader(img.Data)),
			}
			if index%10 == 1 {
				photo.Caption = fmt.Sprintf("%d/%d", index, total)
			}
			sent, err = s.primaryBot.Send(recipient, photo)
		}

		if err == nil {
			log.Debug().
				Int64("chat_id", chatID).
				Str("track_id", img.TrackID).
				Int("index", index).
				Msg("Sent to user")
			return nil
		}

		// Обработка FloodWait
		if strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "retry after") {
			waitTime := s.parseRetryAfter(err.Error())
			if waitTime == 0 {
				waitTime = time.Duration(retry+1) * 3 * time.Second
			}
			log.Warn().
				Err(err).
				Int("retry", retry+1).
				Dur("wait_time", waitTime).
				Msg("FloodWait on user send")
			time.Sleep(waitTime)

			// При FloodWait пробуем через FileID если есть
			if fileID == "" && sent != nil && sent.Photo != nil {
				fileID = sent.Photo.FileID
			}
			continue
		}

		log.Error().Err(err).Int("retry", retry+1).Msg("Failed to send to user")
		time.Sleep(time.Duration(retry+1) * time.Second)
	}

	return fmt.Errorf("failed to send image after %d retries", maxRetries)
}

func (s *Sender) parseRetryAfter(errMsg string) time.Duration {
	if idx := strings.Index(errMsg, "retry after "); idx != -1 {
		substr := errMsg[idx+12:]
		var seconds int
		if _, err := fmt.Sscanf(substr, "%d", &seconds); err == nil {
			return time.Duration(seconds) * time.Second
		}
	}
	return 0
}

func (s *Sender) SendText(chatID int64, text string) error {
	recipient := &tele.User{ID: chatID}
	_, err := s.primaryBot.Send(recipient, text)
	if err != nil {
		log.Error().Err(err).Int64("chat_id", chatID).Msg("Failed to send text")
	}
	return err
}

func (s *Sender) SendFinalMessage(chatID int64, username string, total int) {
	recipient := &tele.User{ID: chatID}
	msg := fmt.Sprintf("✅ Successfully sent %d covers!", total)
	s.primaryBot.Send(recipient, msg)
}

func (s *Sender) Shutdown() {
	for i, worker := range s.workerBots {
		worker.bot.Stop()
		log.Info().Int("worker_id", i).Msg("Worker bot stopped")
	}
}
