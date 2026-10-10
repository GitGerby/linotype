package subtitle

import (
	"image/color"
	"time"
)

// Item represents an extracted subtitle image with timing and geometry,
// ready for OCR processing.
type Item struct {
	Index       int
	StartTime   time.Duration
	EndTime     time.Duration
	PNGData     []byte
	X           int
	Y           int
	Width       int
	Height      int
	VideoWidth  int
	VideoHeight int
	Color       color.RGBA
}
