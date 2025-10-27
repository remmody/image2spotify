package spotify

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	tokenCacheFile = ".spotify_token_cache.json"
)

type TokenCache struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

func InitializeAuth(clientID, clientSecret string) (string, error) {
	// Пытаемся загрузить из кэша
	cache, err := loadTokenCache()
	if err == nil && cache.RefreshToken != "" {
		log.Info().Msg("Found cached refresh token")
		return cache.RefreshToken, nil
	}

	// Проверяем переменную окружения
	if refreshToken := os.Getenv("SPOTIFY_REFRESH_TOKEN"); refreshToken != "" {
		log.Info().Msg("Using refresh token from environment")

		cache := &TokenCache{
			RefreshToken: refreshToken,
		}
		saveTokenCache(cache)

		return refreshToken, nil
	}

	log.Warn().Msg("No refresh token found. Auto-playlist feature will be disabled.")
	log.Warn().Msg("To enable it, run: go run cmd/auth/main.go")

	return "", fmt.Errorf("no refresh token available")
}

func loadTokenCache() (*TokenCache, error) {
	data, err := os.ReadFile(tokenCacheFile)
	if err != nil {
		return nil, err
	}

	var cache TokenCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, err
	}

	return &cache, nil
}

func saveTokenCache(cache *TokenCache) error {
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(tokenCacheFile, data, 0600)
}

func GetAccessToken(clientID, clientSecret, refreshToken string) (string, error) {
	cache, err := loadTokenCache()
	if err == nil && cache.AccessToken != "" && time.Now().Before(cache.ExpiresAt) {
		log.Debug().Msg("Using cached access token")
		return cache.AccessToken, nil
	}

	log.Info().Msg("Refreshing access token")

	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", refreshToken)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		"https://accounts.spotify.com/api/token",
		strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(clientID, clientSecret)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to refresh token: %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	cache = &TokenCache{
		AccessToken:  result.AccessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(result.ExpiresIn-60) * time.Second),
	}
	saveTokenCache(cache)

	log.Info().Msg("Access token refreshed successfully")
	return result.AccessToken, nil
}
