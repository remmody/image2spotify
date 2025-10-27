package telegram

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"image2spotify/internal/processor"
	"image2spotify/internal/spotify"

	"github.com/rs/zerolog/log"
	tele "gopkg.in/telebot.v4"
)

// Кеш для inline результатов
type inlineCache struct {
	mu    sync.RWMutex
	cache map[string]cacheEntry
}

type cacheEntry struct {
	images    []*spotify.ImageData
	expiresAt time.Time
}

var (
	inlineCacheInstance = &inlineCache{
		cache: make(map[string]cacheEntry),
	}
	cacheDuration = 5 * time.Minute
)

func (ic *inlineCache) Get(key string) ([]*spotify.ImageData, bool) {
	ic.mu.RLock()
	defer ic.mu.RUnlock()
	entry, exists := ic.cache[key]
	if !exists || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return entry.images, true
}

func (ic *inlineCache) Set(key string, images []*spotify.ImageData) {
	ic.mu.Lock()
	defer ic.mu.Unlock()
	ic.cache[key] = cacheEntry{
		images:    images,
		expiresAt: time.Now().Add(cacheDuration),
	}
}

type Handlers struct {
	processor *processor.Processor
	sender    *Sender
	bot       *tele.Bot
}

func NewHandlers(bot *tele.Bot, proc *processor.Processor, sender *Sender) *Handlers {
	return &Handlers{
		bot:       bot,
		processor: proc,
		sender:    sender,
	}
}

func (h *Handlers) HandleStart(c tele.Context) error {
	helpText := `🎵 *Spotify Cover Downloader Bot*

Send me any Spotify link and I'll download the covers for you\!

*Supported links:*
• Track: ` + "`https://open.spotify.com/track/...`" + `
• Album: ` + "`https://open.spotify.com/album/...`" + `
• Playlist: ` + "`https://open.spotify.com/playlist/...`" + `

*Features:*
✅ High\-quality images \(640x640\)
✅ Full playlist support \(no limits\)
✅ Inline mode support \(max 50 results\)
✅ Fast parallel processing

*Inline Mode:*
Type ` + "`@botusername spotify_url`" + ` in any chat to get covers instantly\!

Just send me a link and I'll do the rest\! 🚀`

	return c.Send(helpText, &tele.SendOptions{ParseMode: tele.ModeMarkdownV2})
}

func (h *Handlers) HandleMessage(c tele.Context) error {
	text := c.Text()
	if text == "" {
		return c.Send("Please send me a Spotify link.")
	}

	spotifyURL := spotify.FindSpotifyURL(text)
	if spotifyURL == "" {
		return c.Send("No Spotify link found in your message. Please send a valid Spotify track, album, or playlist link.")
	}

	urlType := spotify.DetectURLType(spotifyURL)
	if urlType == "unknown" {
		return c.Send("Unsupported Spotify link. Please send a track, album, or playlist link.")
	}

	username := c.Sender().Username
	if username == "" {
		username = c.Sender().FirstName
	}

	log.Info().
		Int64("user_id", c.Sender().ID).
		Str("username", username).
		Str("url", spotifyURL).
		Str("type", urlType).
		Msg("Processing user request")

	processingMsg, err := c.Bot().Send(c.Sender(), fmt.Sprintf("⏳ Processing %s...", urlType))
	if err != nil {
		log.Error().Err(err).Msg("Failed to send processing message")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	// Получаем треки для автоплейлиста (если включено)
	var trackURIs []string
	if h.processor.IsAutoPlaylistEnabled() {
		tracks, _, _, err := h.processor.GetSpotifyClient().GetTracks(ctx, spotifyURL)
		if err == nil {
			for _, track := range tracks {
				if track.ID != "" {
					trackURIs = append(trackURIs, fmt.Sprintf("spotify:track:%s", track.ID))
				}
			}
		}
	}

	lastUpdate := time.Now()
	var sentCount int32

	progressCallback := func(current, total int) {
		if time.Since(lastUpdate) >= 3*time.Second {
			updateText := fmt.Sprintf("⏳ Processing: %d/%d downloaded, %d sent",
				current, total, atomic.LoadInt32(&sentCount))
			if processingMsg != nil {
				c.Bot().Edit(processingMsg, updateText)
			}
			lastUpdate = time.Now()
		}
	}

	imageCallback := func(img *spotify.ImageData, index, total int) error {
		err := h.sender.StreamImage(c.Chat().ID, username, img, index, total)
		if err == nil {
			atomic.AddInt32(&sentCount, 1)
		}
		return err
	}

	err = h.processor.StreamProcessURL(ctx, spotifyURL, imageCallback, progressCallback)
	if err != nil {
		log.Error().Err(err).Str("url", spotifyURL).Msg("Failed to process URL")
		errorMsg := fmt.Sprintf("❌ Error: %v", err)
		if processingMsg != nil {
			c.Bot().Edit(processingMsg, errorMsg)
		} else {
			c.Send(errorMsg)
		}
		return nil
	}

	// Добавляем треки в автоплейлист (асинхронно)
	if len(trackURIs) > 0 {
		go func() {
			if err := h.processor.AddToAutoPlaylist(context.Background(), trackURIs); err != nil {
				log.Error().Err(err).Int("track_count", len(trackURIs)).Msg("Failed to add to auto-playlist")
			} else {
				log.Info().Int("track_count", len(trackURIs)).Msg("Added tracks to auto-playlist")
			}
		}()
	}

	finalCount := int(atomic.LoadInt32(&sentCount))

	if processingMsg != nil {
		c.Bot().Delete(processingMsg)
	}

	log.Info().
		Int64("user_id", c.Sender().ID).
		Int("image_count", finalCount).
		Int("tracks_added_to_playlist", len(trackURIs)).
		Msg("Successfully processed request")

	h.sender.SendFinalMessage(c.Chat().ID, username, finalCount)

	return nil
}

func (h *Handlers) HandleInlineQuery(c tele.Context) error {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	query := strings.TrimSpace(c.Query().Text)

	log.Debug().
		Int64("user_id", c.Sender().ID).
		Str("query", query).
		Msg("Inline query received")

	if query == "" {
		article := &tele.ArticleResult{
			Title:       "How to use",
			Description: "Paste a Spotify link to get covers",
			Text:        "Send any Spotify track, album, or playlist link to get high-quality cover images!",
			ThumbURL:    "https://storage.googleapis.com/pr-newsroom-wp/1/2018/11/Spotify_Logo_RGB_Green.png",
		}
		article.SetResultID("help")

		return c.Answer(&tele.QueryResponse{
			Results:   tele.Results{article},
			CacheTime: 60,
		})
	}

	spotifyURL := spotify.FindSpotifyURL(query)
	if spotifyURL == "" {
		article := &tele.ArticleResult{
			Title:       "❌ Invalid link",
			Description: "Please paste a valid Spotify URL",
			Text:        "No valid Spotify link found.",
		}
		article.SetResultID("error_invalid")
		return c.Answer(&tele.QueryResponse{Results: tele.Results{article}, CacheTime: 10})
	}

	urlType := spotify.DetectURLType(spotifyURL)
	if urlType == "unknown" {
		article := &tele.ArticleResult{
			Title:       "❌ Unsupported link",
			Description: "Only tracks, albums, and playlists are supported",
			Text:        "This type of Spotify link is not supported.",
		}
		article.SetResultID("error_unsupported")
		return c.Answer(&tele.QueryResponse{Results: tele.Results{article}, CacheTime: 10})
	}

	cacheKey := spotifyURL
	var images []*spotify.ImageData

	if cachedImages, found := inlineCacheInstance.Get(cacheKey); found {
		images = cachedImages
		log.Debug().Int("cached_count", len(images)).Msg("Using cached inline results")
	} else {
		tracks, _, _, err := h.processor.GetSpotifyClient().GetTracks(ctx, spotifyURL)
		if err != nil {
			log.Error().Err(err).Msg("Failed to get tracks for inline")
			article := &tele.ArticleResult{Title: "❌ Error", Description: err.Error(), Text: "Failed to process"}
			article.SetResultID("error")
			return c.Answer(&tele.QueryResponse{Results: tele.Results{article}, CacheTime: 10})
		}

		uniqueImages := make(map[string]*spotify.ImageData)
		for _, track := range tracks {
			if len(track.Album.Images) == 0 {
				continue
			}
			imageURL := track.Album.Images[0].URL
			if _, exists := uniqueImages[imageURL]; !exists {
				uniqueImages[imageURL] = &spotify.ImageData{
					URL:     imageURL,
					TrackID: track.ID,
				}
			}
		}

		images = make([]*spotify.ImageData, 0, len(uniqueImages))
		for _, img := range uniqueImages {
			images = append(images, img)
		}

		inlineCacheInstance.Set(cacheKey, images)
	}

	if len(images) == 0 {
		article := &tele.ArticleResult{Title: "❌ No images", Description: "No covers found", Text: "Empty"}
		article.SetResultID("no_images")
		return c.Answer(&tele.QueryResponse{Results: tele.Results{article}, CacheTime: 60})
	}

	// ЛИМИТ 50 ТОЛЬКО ДЛЯ INLINE
	maxResults := 50
	if len(images) > maxResults {
		log.Debug().Int("total", len(images)).Int("limited", maxResults).Msg("Limiting inline results to 50")
		images = images[:maxResults]
	}

	results := make(tele.Results, 0, len(images))
	timestamp := time.Now().UnixNano()

	for idx, img := range images {
		photoResult := &tele.PhotoResult{
			URL:      img.URL,
			ThumbURL: img.URL,
		}
		photoResult.SetResultID(fmt.Sprintf("p_%s_%d_%d", img.TrackID, timestamp, idx))

		results = append(results, photoResult)
	}

	log.Info().
		Int64("user_id", c.Sender().ID).
		Str("url", spotifyURL).
		Int("results", len(results)).
		Msg("Inline query processed")

	return c.Answer(&tele.QueryResponse{
		Results:    results,
		CacheTime:  30,
		IsPersonal: true,
	})
}
