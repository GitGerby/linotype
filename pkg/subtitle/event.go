package subtitle

import (
	"image/color"
	"time"
)

// Event represents a single subtitle display event with OCR'd text
// and positioning information derived from the PGS stream.
type Event struct {
	// Index is the sequential subtitle number (1-based).
	Index int

	// StartTime is when the subtitle should appear.
	StartTime time.Duration

	// EndTime is when the subtitle should disappear.
	EndTime time.Duration

	// Text is the OCR'd text content, potentially multi-line.
	Text string

	// X and Y are the subtitle position in pixels, relative to the video frame.
	X int
	Y int

	// Width and Height are the dimensions of the original subtitle bitmap.
	Width int
	Height int

	// VideoWidth and VideoHeight are the dimensions of the video frame,
	// used as the reference for percentage-based positioning.
	VideoWidth  int
	VideoHeight int

	// Color is the dominant text color extracted from the PGS palette.
	Color color.RGBA
}

