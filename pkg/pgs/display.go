package pgs

import "time"

// DisplaySet represents a complete subtitle display event.
// It groups all segments between a PCS and an END segment.
type DisplaySet struct {
	PTS     time.Duration // Presentation timestamp
	DTS     time.Duration // Decoding timestamp
	PCS     PCS
	Windows []Window
	Palette Palette
	Objects []ObjectData
}

