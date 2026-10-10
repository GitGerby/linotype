package vobsub

import (
	"encoding/binary"
	"testing"
)

func TestParseSPU(t *testing.T) {
	spuData := make([]byte, 40)
	binary.BigEndian.PutUint16(spuData[0:2], 34)
	binary.BigEndian.PutUint16(spuData[2:4], 10)
	
	// RLE at 4 and 7
	spuData[4] = 0xAA // length 1, color 2, repeated? actually 10(len=0)->0 (fill), so it's wait.
	// let's use valid RLE: length 2, color 1. 0000 10 01 -> 0x09
	// or just length 0, color 1 (fill with 1). 0000 0000 0000 01 -> 0x00 01? wait, 14 bits?
	// just 0x00 0x00 is fill with 0. Let's use 0x00, 0x00
	spuData[4] = 0x00
	spuData[5] = 0x00
	spuData[7] = 0x00
	spuData[8] = 0x00
	
	// DCSQT at 10
	binary.BigEndian.PutUint16(spuData[10:12], 0)
	binary.BigEndian.PutUint16(spuData[12:14], 34)
	
	idx := 14
	spuData[idx] = 0x01; idx++
	spuData[idx] = 0x03; spuData[idx+1] = 0x12; spuData[idx+2] = 0x34; idx += 3
	spuData[idx] = 0x04; spuData[idx+1] = 0x56; spuData[idx+2] = 0x78; idx += 3
	spuData[idx] = 0x05; spuData[idx+1] = 0x00; spuData[idx+2] = 0x00; spuData[idx+3] = 0x01; spuData[idx+4] = 0x00; spuData[idx+5] = 0x00; spuData[idx+6] = 0x01; idx += 7
	spuData[idx] = 0x06; binary.BigEndian.PutUint16(spuData[idx+1:], 4); binary.BigEndian.PutUint16(spuData[idx+3:], 7); idx += 5
	spuData[idx] = 0xFF
	
	spu, err := ParseSPU(spuData[:34])
	if err != nil {
		t.Fatalf("ParseSPU failed: %v", err)
	}
	if spu.Width != 2 || spu.Height != 2 {
		t.Errorf("expected 2x2, got %dx%d", spu.Width, spu.Height)
	}
	
	if spu.Colors[0] != 4 {
		t.Errorf("expected color 0 to be 4, got %d", spu.Colors[0])
	}
}

func TestRender_DominantColorOutlineIgnored(t *testing.T) {
	// 4 pixels:
	// pixel 0: color 2 (outline)
	// pixel 1: color 2 (outline)
	// pixel 2: color 2 (outline)
	// pixel 3: color 1 (pattern/text)
	// Outline has 3 pixels, pattern text has 1 pixel.
	pixels := []byte{2, 2, 2, 1}
	colors := [4]uint8{0, 2, 1, 3} // colors[1] = palette[2] (white), colors[2] = palette[1] (black)
	alphas := [4]uint8{0, 255, 255, 255}
	palette := DefaultPalette() // palette[1] is black, palette[2] is white

	_, domColor, err := Render(pixels, 2, 2, colors, alphas, palette)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	// Dominant color must be white text (palette[2]), not black outline (palette[1])
	if domColor.R < 200 || domColor.G < 200 || domColor.B < 200 {
		t.Errorf("expected white text dominant color, got %+v", domColor)
	}
}

