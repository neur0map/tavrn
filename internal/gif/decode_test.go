package gif

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"testing"
)

func makeTestGIF(frames int, w, h int) []byte {
	g := &gif.GIF{}
	colors := []color.Color{
		color.RGBA{255, 0, 0, 255},
		color.RGBA{0, 255, 0, 255},
		color.RGBA{0, 0, 255, 255},
	}
	palette := append(colors, color.Transparent)

	for i := 0; i < frames; i++ {
		img := image.NewPaletted(image.Rect(0, 0, w, h), palette)
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				img.Set(x, y, colors[i%len(colors)])
			}
		}
		g.Image = append(g.Image, img)
		g.Delay = append(g.Delay, 10) // 100ms
		g.Disposal = append(g.Disposal, gif.DisposalNone)
	}
	g.Config = image.Config{Width: w, Height: h}

	var buf bytes.Buffer
	gif.EncodeAll(&buf, g)
	return buf.Bytes()
}

func TestDecode_BasicGIF(t *testing.T) {
	data := makeTestGIF(3, 10, 10)
	decoded, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode error: %v", err)
	}
	if len(decoded.Frames) != 3 {
		t.Errorf("frames = %d, want 3", len(decoded.Frames))
	}
	if len(decoded.Delays) != 3 {
		t.Errorf("delays = %d, want 3", len(decoded.Delays))
	}
	for i, d := range decoded.Delays {
		if d != 100 {
			t.Errorf("delay[%d] = %d, want 100ms", i, d)
		}
	}
}

func TestDecode_FrameDimensions(t *testing.T) {
	data := makeTestGIF(1, 20, 15)
	decoded, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode error: %v", err)
	}
	bounds := decoded.Frames[0].Bounds()
	if bounds.Dx() != 20 || bounds.Dy() != 15 {
		t.Errorf("frame size = %dx%d, want 20x15", bounds.Dx(), bounds.Dy())
	}
}

func TestDecode_ZeroDelayDefaultsTo100ms(t *testing.T) {
	g := &gif.GIF{
		Config: image.Config{Width: 2, Height: 2},
	}
	palette := []color.Color{color.White, color.Black}
	img := image.NewPaletted(image.Rect(0, 0, 2, 2), palette)
	g.Image = append(g.Image, img)
	g.Delay = append(g.Delay, 0) // 0 delay
	g.Disposal = append(g.Disposal, gif.DisposalNone)

	var buf bytes.Buffer
	gif.EncodeAll(&buf, g)

	decoded, err := Decode(buf.Bytes())
	if err != nil {
		t.Fatalf("Decode error: %v", err)
	}
	if decoded.Delays[0] != 100 {
		t.Errorf("zero delay should default to 100ms, got %d", decoded.Delays[0])
	}
}

func TestDecode_InvalidData(t *testing.T) {
	_, err := Decode([]byte("not a gif"))
	if err == nil {
		t.Error("expected error for invalid data")
	}
}
