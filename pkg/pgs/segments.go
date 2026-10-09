package pgs

// Segment type constants
const (
	SegmentTypePDS byte = 0x14 // Palette Definition Segment
	SegmentTypeODS byte = 0x15 // Object Definition Segment
	SegmentTypePCS byte = 0x16 // Presentation Composition Segment
	SegmentTypeWDS byte = 0x17 // Window Definition Segment
	SegmentTypeEND byte = 0x80 // End of Display Set
)

// CompositionState indicates the type of display update
type CompositionState byte

const (
	CompositionNormal           CompositionState = 0x00
	CompositionAcquisitionPoint CompositionState = 0x40
	CompositionEpochStart       CompositionState = 0x80
)

// PCS - Presentation Composition Segment
type PCS struct {
	Width             uint16
	Height            uint16
	FrameRate         byte
	CompositionNumber uint16
	State             CompositionState
	PaletteUpdateFlag bool
	PaletteID         byte
	Objects           []CompositionObject
}

// CompositionObject represents an object in the PCS
type CompositionObject struct {
	ObjectID    uint16
	WindowID    byte
	CroppedFlag bool
	X           uint16
	Y           uint16
	// Crop fields (only if CroppedFlag)
	CropX      uint16
	CropY      uint16
	CropWidth  uint16
	CropHeight uint16
}

// Window - Window Definition Segment
type Window struct {
	ID     byte
	X      uint16
	Y      uint16
	Width  uint16
	Height uint16
}

// PaletteEntry represents a single YCbCr+Alpha color
type PaletteEntry struct {
	Y, Cb, Cr, Alpha byte
}

// Palette - Palette Definition Segment
type Palette struct {
	ID      byte
	Version byte
	Entries [256]PaletteEntry
}

// ObjectData - Object Definition Segment
type ObjectData struct {
	ID           uint16
	Version      byte
	SequenceFlag byte // 0x40=last, 0x80=first, 0xC0=first and last
	Width        uint16
	Height       uint16
	Data         []byte // RLE-encoded bitmap data
}

