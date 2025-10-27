package spotify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

type PlaylistManager struct {
	clientID     string
	clientSecret string
	refreshToken string
	accessToken  string
	tokenExpiry  time.Time
	mu           sync.RWMutex
	httpClient   *http.Client
}

func NewPlaylistManager(clientID, clientSecret, refreshToken string) *PlaylistManager {
	return &PlaylistManager{
		clientID:     clientID,
		clientSecret: clientSecret,
		refreshToken: refreshToken,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (pm *PlaylistManager) getAccessToken() (string, error) {
	pm.mu.RLock()
	if time.Now().Before(pm.tokenExpiry) && pm.accessToken != "" {
		token := pm.accessToken
		pm.mu.RUnlock()
		return token, nil
	}
	pm.mu.RUnlock()

	// Используем helper для получения токена
	token, err := GetAccessToken(pm.clientID, pm.clientSecret, pm.refreshToken)
	if err != nil {
		return "", err
	}

	pm.mu.Lock()
	pm.accessToken = token
	pm.tokenExpiry = time.Now().Add(55 * time.Minute) // 55 минут (с запасом)
	pm.mu.Unlock()

	return token, nil
}

func (pm *PlaylistManager) AddTracksToPlaylist(ctx context.Context, playlistID string, trackURIs []string) error {
	if len(trackURIs) == 0 {
		return nil
	}

	token, err := pm.getAccessToken()
	if err != nil {
		return fmt.Errorf("failed to get access token: %w", err)
	}

	batchSize := 100
	for i := 0; i < len(trackURIs); i += batchSize {
		end := i + batchSize
		if end > len(trackURIs) {
			end = len(trackURIs)
		}

		batch := trackURIs[i:end]

		body := map[string]interface{}{
			"uris":     batch,
			"position": 0,
		}

		bodyJSON, err := json.Marshal(body)
		if err != nil {
			return err
		}

		apiURL := fmt.Sprintf("https://api.spotify.com/v1/playlists/%s/tracks", playlistID)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(bodyJSON))
		if err != nil {
			return err
		}

		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")

		resp, err := pm.httpClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("failed to add tracks: %d: %s", resp.StatusCode, string(body))
		}

		log.Info().
			Int("batch_start", i).
			Int("batch_end", end).
			Int("total", len(trackURIs)).
			Msg("Added tracks to playlist")

		if end < len(trackURIs) {
			time.Sleep(500 * time.Millisecond)
		}
	}

	return nil
}

func (pm *PlaylistManager) CheckIfTracksExist(ctx context.Context, playlistID string, trackIDs []string) (map[string]bool, error) {
	token, err := pm.getAccessToken()
	if err != nil {
		return nil, err
	}

	exists := make(map[string]bool)
	offset := 0
	limit := 100

	for {
		apiURL := fmt.Sprintf("https://api.spotify.com/v1/playlists/%s/tracks?offset=%d&limit=%d&fields=items(track(id))",
			playlistID, offset, limit)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
		if err != nil {
			return nil, err
		}

		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := pm.httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("failed to get playlist tracks: %d: %s", resp.StatusCode, string(body))
		}

		var result struct {
			Items []struct {
				Track struct {
					ID string `json:"id"`
				} `json:"track"`
			} `json:"items"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, err
		}

		for _, item := range result.Items {
			exists[item.Track.ID] = true
		}

		if len(result.Items) < limit {
			break
		}

		offset += limit
	}

	return exists, nil
}
