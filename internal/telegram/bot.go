package telegram

import (
	"image2spotify/internal/config"
	"image2spotify/internal/processor"
	"time"

	"github.com/rs/zerolog/log"
	tele "gopkg.in/telebot.v4"
)

type Bot struct {
	bot       *tele.Bot
	processor *processor.Processor
	handlers  *Handlers
	sender    *Sender
}

func NewBot(cfg *config.Config, proc *processor.Processor) (*Bot, error) {
	pref := tele.Settings{
		Token:  cfg.TelegramBotToken,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	bot, err := tele.NewBot(pref)
	if err != nil {
		return nil, err
	}

	sender := NewSender(
		bot,
		cfg.WorkerBotTokens, // Передаём токены worker ботов
		cfg.MaxAlbumSize,
		cfg.MaxFileSizeMB,
		cfg.MaxMessagesPerSecond,
		cfg.LogChannelID,
	)
	handlers := NewHandlers(bot, proc, sender)

	b := &Bot{
		bot:       bot,
		processor: proc,
		handlers:  handlers,
		sender:    sender,
	}

	b.setupHandlers()

	return b, nil
}

func (b *Bot) setupHandlers() {
	b.bot.Handle("/start", b.handlers.HandleStart)
	b.bot.Handle("/help", b.handlers.HandleStart)
	b.bot.Handle(tele.OnText, b.handlers.HandleMessage)
	b.bot.Handle(tele.OnQuery, b.handlers.HandleInlineQuery)
}

func (b *Bot) Start() {
	log.Info().Msg("Bot started")
	b.bot.Start()
}

func (b *Bot) Stop() {
	log.Info().Msg("Shutting down bot")
	b.bot.Stop()
	b.sender.Shutdown()
	b.processor.Shutdown()
}
