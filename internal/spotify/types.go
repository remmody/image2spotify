package spotify

type Track struct {
	Album struct {
		Images []struct {
			URL    string `json:"url"`
			Height int    `json:"height"`
			Width  int    `json:"width"`
		} `json:"images"`
		Name string `json:"name"`
	} `json:"album"`
	Name    string `json:"name"`
	Artists []struct {
		Name string `json:"name"`
	} `json:"artists"`
	ID string `json:"id"`
}

type Album struct {
	Images []struct {
		URL    string `json:"url"`
		Height int    `json:"height"`
		Width  int    `json:"width"`
	} `json:"images"`
	Name   string `json:"name"`
	Tracks struct {
		Items []Track `json:"items"`
		Next  string  `json:"next"`
		Total int     `json:"total"`
	} `json:"tracks"`
}

type Playlist struct {
	Images []struct {
		URL string `json:"url"`
	} `json:"images"`
	Name   string `json:"name"`
	Tracks struct {
		Items []struct {
			Track Track `json:"track"`
		} `json:"items"`
		Next  string `json:"next"`
		Total int    `json:"total"`
	} `json:"tracks"`
}

type ImageData struct {
	Data     []byte
	Filename string
	URL      string
	TrackID  string
}
