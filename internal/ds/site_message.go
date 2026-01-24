package ds

import "time"

type SiteMessage struct {
	Id                 int64     `json:"id"`
	Title              string    `json:"title"`
	Content            string    `json:"content"`
	Color              string    `json:"color"`
	PublishedFrontPage bool      `json:"published_front_page"`
	PublishedBinPage   bool      `json:"published_bin_page"`
	CreatedAt          time.Time `json:"created_at"`
	CreatedAtRelative  string    `json:"created_at_relative"`
	UpdatedAt          time.Time `json:"updated_at"`
	UpdatedAtRelative  string    `json:"updated_at_relative"`
}

func (m SiteMessage) IsPublishedFrontPage() bool {
	return m.PublishedFrontPage && (m.Title != "" || m.Content != "")
}

func (m SiteMessage) IsPublishedBinPage() bool {
	return m.PublishedBinPage && (m.Title != "" || m.Content != "")
}

func (m SiteMessage) IsEmpty() bool {
	return m.Title == "" && m.Content == ""
}

func (m SiteMessage) HasContent() bool {
	return m.Title != "" || m.Content != ""
}

func (m SiteMessage) GetAlertClass() string {
	colorMap := map[string]string{
		"blue":   "alert-primary",
		"green":  "alert-success",
		"yellow": "alert-warning",
		"red":    "alert-danger",
		"gray":   "alert-secondary",
		"dark":   "alert-dark",
		"light":  "alert-light",
	}
	if class, ok := colorMap[m.Color]; ok {
		return class
	}
	return "alert-info"
}
