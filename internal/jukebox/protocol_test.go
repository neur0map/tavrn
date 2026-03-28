package jukebox

import (
	"bytes"
	"io"
	"testing"
)

func TestProtocolEncodeDecodeHeader(t *testing.T) {
	track := Track{
		ID:       "123",
		Title:    "Test Song",
		Artist:   "Test Artist",
		Duration: 180,
	}
	var buf bytes.Buffer
	if err := EncodeTrackHeader(&buf, track); err != nil {
		t.Fatalf("encode error: %v", err)
	}

	decoded, err := DecodeTrackHeader(&buf)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if decoded.Title != "Test Song" {
		t.Errorf("expected 'Test Song', got '%s'", decoded.Title)
	}
	if decoded.Artist != "Test Artist" {
		t.Errorf("expected 'Test Artist', got '%s'", decoded.Artist)
	}
	if decoded.Duration != 180 {
		t.Errorf("expected duration 180, got %d", decoded.Duration)
	}
}

func TestProtocolFullFrame(t *testing.T) {
	track := Track{
		ID:       "456",
		Title:    "Another Song",
		Artist:   "Another Artist",
		Duration: 240,
	}

	audioData := []byte("fake mp3 data that could contain any bytes including 0x00")

	// Encode full frame: header + audio length + audio
	var buf bytes.Buffer
	EncodeTrackHeader(&buf, track)
	EncodeAudioLength(&buf, uint32(len(audioData)))
	buf.Write(audioData)

	// Decode
	decoded, err := DecodeTrackHeader(&buf)
	if err != nil {
		t.Fatalf("decode header: %v", err)
	}
	if decoded.ID != "456" {
		t.Errorf("expected ID '456', got '%s'", decoded.ID)
	}

	audioLen, err := DecodeAudioLength(&buf)
	if err != nil {
		t.Fatalf("decode audio length: %v", err)
	}
	if audioLen != uint32(len(audioData)) {
		t.Errorf("expected audio length %d, got %d", len(audioData), audioLen)
	}

	// Read exactly audioLen bytes
	readBack := make([]byte, audioLen)
	_, err = io.ReadFull(&buf, readBack)
	if err != nil {
		t.Fatalf("read audio: %v", err)
	}
	if string(readBack) != string(audioData) {
		t.Errorf("audio data mismatch")
	}

	// Buffer should be empty
	if buf.Len() != 0 {
		t.Errorf("expected empty buffer, got %d bytes remaining", buf.Len())
	}
}

func TestProtocolMultipleFrames(t *testing.T) {
	track1 := Track{ID: "1", Title: "First"}
	track2 := Track{ID: "2", Title: "Second"}
	audio1 := bytes.Repeat([]byte{0xFF, 0x00, 0x01}, 100) // contains 0x00 bytes
	audio2 := []byte("second track audio")

	var buf bytes.Buffer

	// Write two frames
	EncodeTrackHeader(&buf, track1)
	EncodeAudioLength(&buf, uint32(len(audio1)))
	buf.Write(audio1)

	EncodeTrackHeader(&buf, track2)
	EncodeAudioLength(&buf, uint32(len(audio2)))
	buf.Write(audio2)

	// Read first frame
	h1, _ := DecodeTrackHeader(&buf)
	l1, _ := DecodeAudioLength(&buf)
	d1 := make([]byte, l1)
	io.ReadFull(&buf, d1)

	if h1.Title != "First" {
		t.Errorf("expected 'First', got '%s'", h1.Title)
	}
	if !bytes.Equal(d1, audio1) {
		t.Error("first audio data mismatch")
	}

	// Read second frame
	h2, _ := DecodeTrackHeader(&buf)
	l2, _ := DecodeAudioLength(&buf)
	d2 := make([]byte, l2)
	io.ReadFull(&buf, d2)

	if h2.Title != "Second" {
		t.Errorf("expected 'Second', got '%s'", h2.Title)
	}
	if !bytes.Equal(d2, audio2) {
		t.Error("second audio data mismatch")
	}
}
