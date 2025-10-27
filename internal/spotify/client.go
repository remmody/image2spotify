package spotify

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Client struct {
	clientID     string
	clientSecret string
	accessToken  string
	tokenExpiry  time.Time
	mu           sync.RWMutex
	httpClient   *http.Client
}

func NewClient(clientID, clientSecret string) *Client {
	return &Client{
		clientID:     clientID,
		clientSecret: clientSecret,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) getAccessToken(ctx context.Context) (string, error) {
	c.mu.RLock()
	if time.Now().Before(c.tokenExpiry) && c.accessToken != "" {
		token := c.accessToken
		c.mu.RUnlock()
		return token, nil
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	if time.Now().Before(c.tokenExpiry) && c.accessToken != "" {
		return c.accessToken, nil
	}

	data := strings.NewReader("grant_type=client_credentials")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://accounts.spotify.com/api/token", data)
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(c.clientID, c.clientSecret)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	c.accessToken = result.AccessToken
	c.tokenExpiry = time.Now().Add(time.Duration(result.ExpiresIn-60) * time.Second)

	return c.accessToken, nil
}

func (c *Client) apiRequest(ctx context.Context, url string) ([]byte, error) {
	token, err := c.getAccessToken(ctx)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

func (c *Client) GetTrack(ctx context.Context, trackID string) (*Track, error) {
	data, err := c.apiRequest(ctx, fmt.Sprintf("https://api.spotify.com/v1/tracks/%s", trackID))
	if err != nil {
		return nil, err
	}

	var track Track
	if err := json.Unmarshal(data, &track); err != nil {
		return nil, err
	}

	return &track, nil
}

func (c *Client) GetAlbumTracks(ctx context.Context, albumID string) ([]Track, error) {
	data, err := c.apiRequest(ctx, fmt.Sprintf("https://api.spotify.com/v1/albums/%s", albumID))
	if err != nil {
		return nil, err
	}

	var album Album
	if err := json.Unmarshal(data, &album); err != nil {
		return nil, err
	}

	tracks := album.Tracks.Items
	for i := range tracks {
		tracks[i].Album.Images = album.Images
		tracks[i].Album.Name = album.Name
	}

	offset := 50
	for offset < album.Tracks.Total {
		url := fmt.Sprintf("https://api.spotify.com/v1/albums/%s/tracks?offset=%d&limit=50", albumID, offset)
		pageData, err := c.apiRequest(ctx, url)
		if err != nil {
			break
		}

		var page struct {
			Items []Track `json:"items"`
		}
		if err := json.Unmarshal(pageData, &page); err != nil {
			break
		}

		for i := range page.Items {
			page.Items[i].Album.Images = album.Images
			page.Items[i].Album.Name = album.Name
		}

		tracks = append(tracks, page.Items...)
		offset += 50
	}

	return tracks, nil
}

func (c *Client) GetPlaylistTracks(ctx context.Context, playlistID string) ([]Track, error) {
	data, err := c.apiRequest(ctx, fmt.Sprintf("https://api.spotify.com/v1/playlists/%s", playlistID))
	if err != nil {
		if strings.Contains(err.Error(), "404") && strings.HasPrefix(playlistID, "37i9dQZF") {
			return nil, fmt.Errorf("editorial playlists are not accessible via API")
		}
		return nil, err
	}

	var playlist Playlist
	if err := json.Unmarshal(data, &playlist); err != nil {
		return nil, err
	}

	var tracks []Track
	for _, item := range playlist.Tracks.Items {
		if item.Track.ID != "" && len(item.Track.Album.Images) > 0 {
			tracks = append(tracks, item.Track)
		}
	}

	offset := 100
	for offset < playlist.Tracks.Total {
		url := fmt.Sprintf("https://api.spotify.com/v1/playlists/%s/tracks?offset=%d&limit=100", playlistID, offset)
		pageData, err := c.apiRequest(ctx, url)
		if err != nil {
			break
		}

		var page struct {
			Items []struct {
				Track Track `json:"track"`
			} `json:"items"`
		}
		if err := json.Unmarshal(pageData, &page); err != nil {
			break
		}

		for _, item := range page.Items {
			if item.Track.ID != "" && len(item.Track.Album.Images) > 0 {
				tracks = append(tracks, item.Track)
			}
		}

		offset += 100
	}

	return tracks, nil
}

func (c *Client) GetTracks(ctx context.Context, url string) ([]Track, string, string, error) {
	urlType := DetectURLType(url)
	sourceID, err := ExtractID(url, urlType)
	if err != nil {
		return nil, "", "", err
	}

	var tracks []Track

	switch urlType {
	case "track":
		track, err := c.GetTrack(ctx, sourceID)
		if err != nil {
			return nil, "", "", err
		}
		tracks = []Track{*track}
	case "album":
		tracks, err = c.GetAlbumTracks(ctx, sourceID)
		if err != nil {
			return nil, "", "", err
		}
	case "playlist":
		tracks, err = c.GetPlaylistTracks(ctx, sourceID)
		if err != nil {
			return nil, "", "", err
		}
	default:
		return nil, "", "", fmt.Errorf("unsupported URL type")
	}

	return tracks, sourceID, urlType, nil
}
