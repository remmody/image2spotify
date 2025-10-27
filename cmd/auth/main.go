package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

const (
	authURL     = "https://accounts.spotify.com/authorize"
	tokenURL    = "https://accounts.spotify.com/api/token"
	redirectURI = "http://127.0.0.1:8888/callback" // ИЗМЕНЕНО: localhost → 127.0.0.1
	scope       = "playlist-modify-public playlist-modify-private"
)

var (
	clientID     string
	clientSecret string
	codeChan     = make(chan string, 1)
)

func main() {
	_ = godotenv.Load()

	clientID = os.Getenv("SPOTIFY_CLIENT_ID")
	clientSecret = os.Getenv("SPOTIFY_CLIENT_SECRET")

	if clientID == "" || clientSecret == "" {
		log.Fatal("Set SPOTIFY_CLIENT_ID and SPOTIFY_CLIENT_SECRET in .env file")
	}

	http.HandleFunc("/callback", handleCallback)
	server := &http.Server{Addr: "127.0.0.1:8888"} // ИЗМЕНЕНО: явно указываем 127.0.0.1

	go func() {
		log.Println("Starting local server on http://127.0.0.1:8888")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	authURLWithParams := fmt.Sprintf("%s?client_id=%s&response_type=code&redirect_uri=%s&scope=%s",
		authURL, clientID, url.QueryEscape(redirectURI), url.QueryEscape(scope))

	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("Spotify Authorization")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println("\n1. Open this URL in your browser:")
	fmt.Println("\n   " + authURLWithParams)
	fmt.Println("\n2. Authorize the application")
	fmt.Println(strings.Repeat("=", 70))

	select {
	case code := <-codeChan:
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(ctx)

		token, err := exchangeCodeForToken(code)
		if err != nil {
			log.Fatal(err)
		}

		// Сохраняем в кэш файл
		cacheData := map[string]interface{}{
			"refresh_token": token,
			"created_at":    time.Now().Format(time.RFC3339),
		}

		data, _ := json.MarshalIndent(cacheData, "", "  ")
		os.WriteFile(".spotify_token_cache.json", data, 0600)

		fmt.Println("\n" + strings.Repeat("=", 70))
		fmt.Println("✅ SUCCESS!")
		fmt.Println(strings.Repeat("=", 70))
		fmt.Println("\nRefresh token saved to: .spotify_token_cache.json")
		fmt.Println("\nYou can also add this to your .env file:")
		fmt.Println("\n   SPOTIFY_REFRESH_TOKEN=" + token)
		fmt.Println("\n" + strings.Repeat("=", 70))
		fmt.Println("\nNow you can start the bot with: go run cmd/bot/main.go")
		fmt.Println(strings.Repeat("=", 70) + "\n")

	case <-time.After(2 * time.Minute):
		log.Fatal("Authorization timeout (2 minutes)")
	}
}

func handleCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "No authorization code", http.StatusBadRequest)
		return
	}

	codeChan <- code

	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `
<!DOCTYPE html>
<html>
<head>
	<title>Authorization Successful</title>
	<style>
		body { font-family: Arial, sans-serif; text-align: center; padding: 50px; background: #1DB954; color: white; }
		h1 { font-size: 3em; margin-bottom: 20px; }
		p { font-size: 1.5em; }
	</style>
</head>
<body>
	<h1>✅ Authorization Successful!</h1>
	<p>You can close this window and return to the terminal.</p>
	<p>The bot is now configured for auto-playlist feature.</p>
</body>
</html>
	`)
}

func exchangeCodeForToken(code string) (string, error) {
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", redirectURI)

	req, err := http.NewRequest(http.MethodPost, tokenURL, strings.NewReader(data.Encode()))
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
		return "", fmt.Errorf("failed to get token: status %d", resp.StatusCode)
	}

	var result struct {
		RefreshToken string `json:"refresh_token"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.RefreshToken, nil
}
