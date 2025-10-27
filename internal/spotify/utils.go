package spotify

import (
	"fmt"
	"regexp"
	"strings"
)

func CleanURL(rawURL string) string {
	if idx := strings.Index(rawURL, "?"); idx != -1 {
		rawURL = rawURL[:idx]
	}
	return rawURL
}

func ExtractID(rawURL, typ string) (string, error) {
	cleanedURL := CleanURL(rawURL)
	pattern := fmt.Sprintf(`%s/([a-zA-Z0-9]+)`, typ)
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(cleanedURL)
	if len(matches) < 2 {
		return "", fmt.Errorf("cannot extract %s ID", typ)
	}
	return matches[1], nil
}

func DetectURLType(rawURL string) string {
	cleanedURL := CleanURL(rawURL)
	if strings.Contains(cleanedURL, "/track/") {
		return "track"
	} else if strings.Contains(cleanedURL, "/album/") {
		return "album"
	} else if strings.Contains(cleanedURL, "/playlist/") {
		return "playlist"
	}
	return "unknown"
}

func FindSpotifyURL(text string) string {
	re := regexp.MustCompile(`https?://open\.spotify\.com/(track|album|playlist)/[a-zA-Z0-9]+`)
	return re.FindString(text)
}
