package jukebox

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

type testConn struct {
	buf    bytes.Buffer
	mu     sync.Mutex
	closed bool
}

func (c *testConn) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return 0, io.ErrClosedPipe
	}
	return c.buf.Write(p)
}

func (c *testConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return nil
}

func (c *testConn) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buf.Len()
}

func TestStreamerAddRemoveConn(t *testing.T) {
	s := NewStreamer()
	conn := &testConn{}

	s.AddConn(conn)
	if s.ConnCount() != 1 {
		t.Errorf("expected 1 conn, got %d", s.ConnCount())
	}

	s.RemoveConn(conn)
	if s.ConnCount() != 0 {
		t.Errorf("expected 0 conns, got %d", s.ConnCount())
	}
}

func TestStreamerBroadcastsToConns(t *testing.T) {
	mp3Data := bytes.Repeat([]byte{0xFF, 0xFB, 0x90, 0x00}, 500)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/mpeg")
		w.Write(mp3Data)
	}))
	defer ts.Close()

	s := NewStreamer()

	conn1 := &testConn{}
	conn2 := &testConn{}
	s.AddConn(conn1)
	s.AddConn(conn2)

	track := Track{ID: "1", Title: "Test", Artist: "Artist", Duration: 10, URL: ts.URL}
	s.StreamTrack(track)

	time.Sleep(500 * time.Millisecond)
	s.Stop()

	if conn1.Len() == 0 {
		t.Error("conn1 received no data")
	}
	if conn2.Len() == 0 {
		t.Error("conn2 received no data")
	}
}
