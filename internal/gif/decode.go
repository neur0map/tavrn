package gif

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"image/gif"
)

// DecodedGIF holds the extracted frames and their delays.
type DecodedGIF struct {
	Frames []image.Image
	Delays []int // delay per frame in milliseconds
}

// Decode extracts all frames from GIF data, compositing them properly
// to handle disposal methods and transparency.
func Decode(data []byte) (*DecodedGIF, error) {
	g, err := gif.DecodeAll(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode gif: %w", err)
	}

	if len(g.Image) == 0 {
		return nil, fmt.Errorf("gif has no frames")
	}

	width := g.Config.Width
	height := g.Config.Height
	if width == 0 || height == 0 {
		bounds := g.Image[0].Bounds()
		width = bounds.Dx()
		height = bounds.Dy()
	}

	canvas := image.NewRGBA(image.Rect(0, 0, width, height))
	frames := make([]image.Image, len(g.Image))
	delays := make([]int, len(g.Image))

	for i, frame := range g.Image {
		// Handle disposal from previous frame
		if i > 0 {
			switch g.Disposal[i-1] {
			case gif.DisposalBackground:
				// Clear the previous frame's area to background
				prev := g.Image[i-1].Bounds()
				for y := prev.Min.Y; y < prev.Max.Y; y++ {
					for x := prev.Min.X; x < prev.Max.X; x++ {
						canvas.Set(x, y, image.Transparent)
					}
				}
			case gif.DisposalPrevious:
				// Restore to state before previous frame — we'd need to track this
				// For simplicity, treat same as background
				prev := g.Image[i-1].Bounds()
				for y := prev.Min.Y; y < prev.Max.Y; y++ {
					for x := prev.Min.X; x < prev.Max.X; x++ {
						canvas.Set(x, y, image.Transparent)
					}
				}
			}
			// DisposalNone (0) or no disposal: leave canvas as-is
		}

		// Draw current frame onto canvas
		draw.Draw(canvas, frame.Bounds(), frame, frame.Bounds().Min, draw.Over)

		// Snapshot the canvas for this frame
		snapshot := image.NewRGBA(canvas.Bounds())
		draw.Draw(snapshot, snapshot.Bounds(), canvas, canvas.Bounds().Min, draw.Src)
		frames[i] = snapshot

		// Convert delay: GIF delay is in 100ths of a second → milliseconds
		delay := g.Delay[i] * 10
		if delay < 20 {
			delay = 100 // browsers default ~10fps for 0-delay frames
		}
		delays[i] = delay
	}

	return &DecodedGIF{Frames: frames, Delays: delays}, nil
}
