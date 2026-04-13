package reddit

import (
	"testing"
	"time"
)

func TestFetchSubreddit(t *testing.T) {
	c := NewClient()
	posts, err := c.FetchSubreddit("golang", 5)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(posts) == 0 {
		t.Fatal("expected posts, got none")
	}
	if posts[0].Title == "" {
		t.Error("first post has empty title")
	}
	if posts[0].Subreddit == "" {
		t.Error("first post has empty subreddit")
	}
}

func TestFetchMerged(t *testing.T) {
	c := NewClient()
	posts, err := c.FetchMerged([]string{"golang", "programming"}, 5)
	if err != nil {
		t.Fatalf("fetch merged: %v", err)
	}
	if len(posts) == 0 {
		t.Fatal("expected posts, got none")
	}
	subs := make(map[string]bool)
	for _, p := range posts {
		subs[p.Subreddit] = true
	}
	if len(subs) < 2 {
		t.Logf("warning: only got posts from %d subreddit(s), may be rate limited", len(subs))
	}
}

func TestCaching(t *testing.T) {
	c := NewClient()
	c.cacheTTL = 1 * time.Hour

	_, err := c.FetchMerged([]string{"golang"}, 5)
	if err != nil {
		t.Fatalf("first fetch: %v", err)
	}

	posts, err := c.Posts()
	if err != nil {
		t.Fatalf("cached fetch: %v", err)
	}
	if len(posts) == 0 {
		t.Fatal("cache returned no posts")
	}
}

func TestFetchComments(t *testing.T) {
	c := NewClient()
	posts, err := c.FetchSubreddit("golang", 1)
	if err != nil || len(posts) == 0 {
		t.Skip("could not fetch a post to test comments")
	}
	comments, err := c.FetchComments(posts[0].Subreddit, posts[0].ID, 10)
	if err != nil {
		t.Fatalf("fetch comments: %v", err)
	}
	t.Logf("got %d top-level comments", len(comments))
}
