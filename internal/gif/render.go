package gif

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"

	"golang.org/x/image/draw"
)

// RenderHalfBlocks converts an image to a half-block ANSI string.
// Each character cell represents 2 vertical pixels using ▀ with
// foreground (top pixel) and background (bottom pixel) colors.
// Width is the output width in characters. Height is computed from aspect ratio.
func RenderHalfBlocks(img image.Image, width int) string {
	// Resize to target width. Terminal chars are ~2:1 height:width,
	// and half-blocks pack 2 pixels per row, so these cancel out.
	// Just use the raw aspect ratio.
	bounds := img.Bounds()
	aspect := float64(bounds.Dy()) / float64(bounds.Dx())
	// Halve height because terminal chars are ~2x taller than wide
	height := int(float64(width) * aspect * 0.5)
	if height%2 != 0 {
		height++
	}
	if height < 2 {
		height = 2
	}
	// Cap height to prevent absurdly tall renders
	maxHeight := width
	if height > maxHeight {
		height = maxHeight
		if height%2 != 0 {
			height--
		}
	}

	resized := resizeImage(img, width, height)
	dithered := floydSteinberg(resized)

	return renderToString(dithered, width, height)
}

// RenderHalfBlocksClean converts an image to a half-block ANSI string
// without Floyd-Steinberg dithering. Produces sharper, cleaner output
// for photographs since we already use 24-bit true color.
func RenderHalfBlocksClean(img image.Image, width int) string {
	bounds := img.Bounds()
	aspect := float64(bounds.Dy()) / float64(bounds.Dx())
	height := int(float64(width) * aspect * 0.5)
	if height%2 != 0 {
		height++
	}
	if height < 2 {
		height = 2
	}
	maxHeight := width
	if height > maxHeight {
		height = maxHeight
		if height%2 != 0 {
			height--
		}
	}

	resized := resizeImage(img, width, height)
	return renderToString(resized, width, height)
}

// RenderFrames converts a slice of images to half-block ANSI strings.
func RenderFrames(frames []image.Image, width int) []string {
	rendered := make([]string, len(frames))
	for i, frame := range frames {
		rendered[i] = RenderHalfBlocks(frame, width)
	}
	return rendered
}

func resizeImage(img image.Image, width, height int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Over, nil)
	return dst
}

// floydSteinberg applies Floyd-Steinberg error diffusion with serpentine scanning.
// Returns the dithered image (modified in place for performance).
func floydSteinberg(img *image.RGBA) *image.RGBA {
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	// Work with float errors per channel
	errors := make([][]errPixel, h)
	for y := range errors {
		errors[y] = make([]errPixel, w)
	}

	for y := 0; y < h; y++ {
		// Serpentine: alternate scan direction per row
		var xStart, xEnd, xStep int
		if y%2 == 0 {
			xStart, xEnd, xStep = 0, w, 1
		} else {
			xStart, xEnd, xStep = w-1, -1, -1
		}

		for x := xStart; x != xEnd; x += xStep {
			r, g, b, a := img.At(x+bounds.Min.X, y+bounds.Min.Y).RGBA()
			// Add accumulated error
			oldR := clampF(float64(r>>8) + errors[y][x].r)
			oldG := clampF(float64(g>>8) + errors[y][x].g)
			oldB := clampF(float64(b>>8) + errors[y][x].b)

			// Quantize to nearest terminal color (full 24-bit, so just round)
			newR := math.Round(oldR)
			newG := math.Round(oldG)
			newB := math.Round(oldB)

			// Set the quantized pixel
			img.Set(x+bounds.Min.X, y+bounds.Min.Y, color.RGBA{
				R: uint8(newR), G: uint8(newG), B: uint8(newB), A: uint8(a >> 8),
			})

			// Compute error
			errR := oldR - newR
			errG := oldG - newG
			errB := oldB - newB

			// Distribute error to neighbors (serpentine-aware)
			distributeError(errors, x, y, w, h, xStep, errR, errG, errB)
		}
	}

	return img
}

func distributeError(errors [][]errPixel, x, y, w, h, direction int, errR, errG, errB float64) {
	spread := func(dx, dy int, weight float64) {
		nx := x + dx*direction
		ny := y + dy
		if nx >= 0 && nx < w && ny >= 0 && ny < h {
			errors[ny][nx].r += errR * weight
			errors[ny][nx].g += errG * weight
			errors[ny][nx].b += errB * weight
		}
	}

	// Floyd-Steinberg weights
	spread(1, 0, 7.0/16.0)  // right (or left if reversed)
	spread(-1, 1, 3.0/16.0) // below-opposite
	spread(0, 1, 5.0/16.0)  // below
	spread(1, 1, 1.0/16.0)  // below-same
}

type errPixel struct{ r, g, b float64 }

func clampF(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}

func renderToString(img *image.RGBA, width, height int) string {
	bounds := img.Bounds()
	var b strings.Builder
	// Pre-allocate: ~30 bytes per cell (ANSI codes) * width * rows
	b.Grow(width * (height / 2) * 30)

	for y := 0; y < height; y += 2 {
		if y > 0 {
			b.WriteString("\033[0m\n")
		}
		for x := 0; x < width; x++ {
			top := img.At(x+bounds.Min.X, y+bounds.Min.Y)
			bottom := img.At(x+bounds.Min.X, y+1+bounds.Min.Y)

			tr, tg, tb, ta := top.RGBA()
			br, bg, bb, ba := bottom.RGBA()

			tr8, tg8, tb8 := uint8(tr>>8), uint8(tg>>8), uint8(tb>>8)
			br8, bg8, bb8 := uint8(br>>8), uint8(bg>>8), uint8(bb>>8)

			if ta>>8 < 128 && ba>>8 < 128 {
				b.WriteRune(' ')
			} else if ta>>8 < 128 {
				fmt.Fprintf(&b, "\033[38;2;%d;%d;%dm▄", br8, bg8, bb8)
			} else if ba>>8 < 128 {
				fmt.Fprintf(&b, "\033[38;2;%d;%d;%dm▀", tr8, tg8, tb8)
			} else {
				fmt.Fprintf(&b, "\033[38;2;%d;%d;%dm\033[48;2;%d;%d;%dm▀",
					tr8, tg8, tb8, br8, bg8, bb8)
			}
		}
	}
	b.WriteString("\033[0m")
	return b.String()
}
