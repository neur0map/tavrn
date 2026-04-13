package reddit

import "time"

type Post struct {
	ID          string
	Title       string
	Author      string
	Subreddit   string
	Score       int
	NumComments int
	CreatedUTC  time.Time
	URL         string
	Permalink   string
	Selftext    string
	IsSelf      bool
	Thumbnail   string
	PostHint    string
	Domain      string
	IsVideo     bool
	PreviewURL  string
	PreviewW    int
	PreviewH    int
	HasImage    bool
}

type Comment struct {
	Author   string
	Body     string
	Score    int
	Created  time.Time
	Depth    int
	Children []Comment
}
