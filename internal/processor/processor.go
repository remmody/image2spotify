package processor

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"image2spotify/internal/spotify"

	"github.com/rs/zerolog/log"
)

type Processor struct {
	spotifyClient      *spotify.Client
	playlistManager    *spotify.PlaylistManager
	autoPlaylistID     string
	enableAutoPlaylist bool
	workerPool         *WorkerPool
	timeout            time.Duration
}


func NewProcessor(
	spotifyClient *spotify.Client,
	playlistManager *spotify.PlaylistManager,
	autoPlaylistID string,
	enableAutoPlaylist bool,
	workers int,
	imageTimeout, processTimeout time.Duration,
) *Processor {
	return &Processor{
		spotifyClient:      spotifyClient,
		playlistManager:    playlistManager,
		autoPlaylistID:     autoPlaylistID,
		enableAutoPlaylist: enableAutoPlaylist,
		workerPool:         NewWorkerPool(workers, imageTimeout),
		timeout:            processTimeout,
	}
}


func (p *Processor) GetSpotifyClient() *spotify.Client {
	return p.spotifyClient
}

// StreamProcessURL обрабатывает URL и вызывает callback для каждого скачанного изображения
func (p *Processor) StreamProcessURL(
	ctx context.Context,
	url string,
	imageCallback func(img *spotify.ImageData, index, total int) error,
	progressCallback func(current, total int),
) error {
	processCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	tracks, sourceID, urlType, err := p.spotifyClient.GetTracks(processCtx, url)
	if err != nil {
		return fmt.Errorf("failed to get tracks: %w", err)
	}

	if len(tracks) == 0 {
		return fmt.Errorf("no tracks found")
	}

	log.Info().
		Str("url", url).
		Str("type", urlType).
		Str("source_id", sourceID).
		Int("track_count", len(tracks)).
		Msg("Processing URL")

	uniqueImages := make(map[string]string)
	for _, track := range tracks {
		if len(track.Album.Images) == 0 {
			continue
		}
		imageURL := track.Album.Images[0].URL
		trackID := track.ID
		if trackID == "" {
			trackID = fmt.Sprintf("unknown_%d", len(uniqueImages))
		}
		if _, exists := uniqueImages[imageURL]; !exists {
			uniqueImages[imageURL] = trackID
		}
	}

	total := len(uniqueImages)
	if total == 0 {
		return fmt.Errorf("no images found")
	}

	log.Debug().Int("unique_images", total).Msg("Found unique images")

	resultsChan := make(chan *spotify.ImageData, total)
	var downloadedCount int32
	var successCount int32

	// Submit tasks
	submitted := 0
	for imageURL, trackID := range uniqueImages {
		task := &DownloadTask{
			URL:     imageURL,
			TrackID: trackID,
			Result:  resultsChan,
		}
		if p.workerPool.Submit(task) {
			submitted++
		}
	}

	log.Debug().Int("submitted", submitted).Int("total", total).Msg("Submitted download tasks")

	// Обрабатываем результаты по мере поступления
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	lastReported := int32(0)

	for int(atomic.LoadInt32(&downloadedCount)) < total {
		select {
		case result := <-resultsChan:
			current := atomic.AddInt32(&downloadedCount, 1)

			if len(result.Data) > 0 {
				success := atomic.AddInt32(&successCount, 1)
				
				// КЛЮЧЕВОЙ МОМЕНТ: Вызываем callback сразу для каждого изображения
				if imageCallback != nil {
					if err := imageCallback(result, int(success), total); err != nil {
						log.Error().Err(err).Msg("Image callback failed")
					}
				}
			}

			if progressCallback != nil && (current%10 == 0 || current == int32(total)) {
				progressCallback(int(current), total)
			}

		case <-ticker.C:
			current := atomic.LoadInt32(&downloadedCount)
			success := atomic.LoadInt32(&successCount)
			if current > lastReported {
				log.Debug().
					Int32("downloaded", current).
					Int32("successful", success).
					Int("total", total).
					Msg("Download progress")
				lastReported = current
			}

		case <-processCtx.Done():
			log.Error().
				Int32("downloaded", atomic.LoadInt32(&downloadedCount)).
				Int("total", total).
				Msg("Processing timeout")
			return fmt.Errorf("processing timeout")
		}
	}

	close(resultsChan)

	finalSuccess := int(atomic.LoadInt32(&successCount))
	if finalSuccess == 0 {
		return fmt.Errorf("no images were downloaded successfully")
	}

	log.Info().
		Int("successful", finalSuccess).
		Int("total", total).
		Msg("Download completed")

	return nil
}

func (p *Processor) Shutdown() {
	log.Info().Msg("Shutting down processor")
	p.workerPool.Shutdown()
}
func (p *Processor) IsAutoPlaylistEnabled() bool {
	return p.enableAutoPlaylist && p.autoPlaylistID != "" && p.playlistManager != nil
}
func (p *Processor) AddToAutoPlaylist(ctx context.Context, trackURIs []string) error {
	if !p.IsAutoPlaylistEnabled() {
		return fmt.Errorf("auto-playlist not enabled")
	}

	// Извлекаем track IDs из URIs
	trackIDs := make([]string, 0, len(trackURIs))
	for _, uri := range trackURIs {
		parts := strings.Split(uri, ":")
		if len(parts) == 3 && parts[1] == "track" {
			trackIDs = append(trackIDs, parts[2])
		}
	}

	// Проверяем какие треки уже есть
	existing, err := p.playlistManager.CheckIfTracksExist(ctx, p.autoPlaylistID, trackIDs)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to check existing tracks, adding all")
		existing = make(map[string]bool)
	}

	// Фильтруем новые треки
	newTrackURIs := make([]string, 0)
	for _, uri := range trackURIs {
		parts := strings.Split(uri, ":")
		if len(parts) == 3 {
			trackID := parts[2]
			if !existing[trackID] {
				newTrackURIs = append(newTrackURIs, uri)
			}
		}
	}

	if len(newTrackURIs) == 0 {
		log.Info().Msg("All tracks already in playlist")
		return nil
	}

	log.Info().
		Int("new_tracks", len(newTrackURIs)).
		Int("total_tracks", len(trackURIs)).
		Msg("Adding new tracks to auto-playlist")

	return p.playlistManager.AddTracksToPlaylist(ctx, p.autoPlaylistID, newTrackURIs)
}