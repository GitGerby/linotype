package vobsub

import (
	"bytes"
	"testing"
)

func TestParsePSStream(t *testing.T) {
	data := []byte{
		0x00, 0x00, 0x01, 0xBA,
		0x44, 0x00, 0x04, 0x00, 0x04, 0x01, 0x01, 0x89, 0xC3, 0xF8, // 10 bytes (MPEG-2 PS)
		0x00, 0x00, 0x01, 0xBD,
		0x00, 0x0C, // PES length 12
		0x80, 0x80, 0x05, 0x21, 0x00, 0x01, 0x00, 0x01, // PES header + PTS
		0x20,             // Substream ID
		0x00, 0x03, 0xFF, // SPU bytes
	}

	spus, err := ParsePSStream(bytes.NewReader(data), 0x20)
	if err != nil {
		t.Fatalf("ParsePSStream failed: %v", err)
	}
	if len(spus) != 1 {
		t.Fatalf("expected 1 SPU, got %d", len(spus))
	}
	if len(spus[0].Data) != 3 {
		t.Errorf("expected 3 bytes in SPU, got %d", len(spus[0].Data))
	}
}
