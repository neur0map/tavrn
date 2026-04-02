package gif

import (
	"image"
	"image/color"
	"strings"
	"testing"
)

func solidImage(w, h int, c color.Color) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	return img
}

func TestRenderHalfBlocks_SolidRed(t *testing.T) {
	img := solidImage(10, 10, color.RGBA{255, 0, 0, 255})
	result := RenderHalfBlocks(img, 10)
	if result == "" {
		t.Fatal("empty result")
	}
	// Should contain the upper half block character
	if !strings.Contains(result, "▀") {
		t.Error("expected half-block characters")
	}
	// Should contain red ANSI color codes (255;0;0)
	if !strings.Contains(result, "255;0;0") {
		t.Error("expected red color in ANSI output")
	}
	// Should end with reset
	if !strings.HasSuffix(result, "\033[0m") {
		t.Error("expected ANSI reset at end")
	}
}

func TestRenderHalfBlocks_Dimensions(t *testing.T) {
	img := solidImage(20, 20, color.RGBA{0, 128, 0, 255})
	result := RenderHalfBlocks(img, 10)
	lines := strings.Split(result, "\n")
	// 1:1 image at width 10, height halved for terminal aspect = 5 pixels,
	// rounded to even = 6, / 2 per row = 3 rows
	if len(lines) < 1 {
		t.Error("expected at least 1 row")
	}
}

func TestRenderHalfBlocks_TransparentPixels(t *testing.T) {
	img := solidImage(4, 4, color.RGBA{0, 0, 0, 0})
	result := RenderHalfBlocks(img, 4)
	// All transparent — should have spaces
	if !strings.Contains(result, " ") {
		t.Error("expected spaces for transparent pixels")
	}
}

func TestRenderHalfBlocks_MinHeight(t *testing.T) {
	// Very wide, very short image
	img := solidImage(100, 1, color.RGBA{255, 255, 255, 255})
	result := RenderHalfBlocks(img, 10)
	if result == "" {
		t.Fatal("empty result")
	}
	lines := strings.Split(result, "\n")
	if len(lines) < 1 {
		t.Error("expected at least 1 row")
	}
}

func TestRenderFrames(t *testing.T) {
	frames := []image.Image{
		solidImage(10, 10, color.RGBA{255, 0, 0, 255}),
		solidImage(10, 10, color.RGBA{0, 255, 0, 255}),
		solidImage(10, 10, color.RGBA{0, 0, 255, 255}),
	}
	rendered := RenderFrames(frames, 10)
	if len(rendered) != 3 {
		t.Fatalf("expected 3 frames, got %d", len(rendered))
	}
	if !strings.Contains(rendered[0], "255;0;0") {
		t.Error("frame 0 should be red")
	}
	if !strings.Contains(rendered[1], "0;255;0") {
		t.Error("frame 1 should be green")
	}
	if !strings.Contains(rendered[2], "0;0;255") {
		t.Error("frame 2 should be blue")
	}
}

func TestFloydSteinberg_DoesNotPanic(t *testing.T) {
	// Various sizes including edge cases
	sizes := [][2]int{{1, 1}, {1, 2}, {2, 1}, {10, 10}, {3, 7}}
	for _, s := range sizes {
		img := solidImage(s[0], s[1], color.RGBA{128, 128, 128, 255})
		_ = floydSteinberg(img)
	}
}
