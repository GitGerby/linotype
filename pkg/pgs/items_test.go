package pgs

import (
	"testing"
	"time"
)

func TestExtractSubtitleItems(t *testing.T) {
	// Create two sets: one appearance at 1s, one clearance at 3s
	pcs1 := PCS{
		Width:  1920,
		Height: 1080,
		Objects: []CompositionObject{
			{ObjectID: 1, X: 100, Y: 200},
		},
	}
	var pal Palette
	pal.Entries[1] = PaletteEntry{Y: 81, Cb: 90, Cr: 240, Alpha: 255} // Red
	obj1 := ObjectData{
		ID:     1,
		Width:  2,
		Height: 2,
		Data:   []byte{0x00, 0x84, 0x01}, // 4 pixels of color 1
	}

	set1 := DisplaySet{
		PTS:     time.Second,
		PCS:     pcs1,
		Palette: pal,
		Objects: []ObjectData{obj1},
	}

	// Clearance set at 3s
	pcs2 := PCS{
		Width:   1920,
		Height:  1080,
		Objects: nil,
	}
	set2 := DisplaySet{
		PTS: 3 * time.Second,
		PCS: pcs2,
	}

	items, err := ExtractSubtitleItems([]DisplaySet{set1, set2})
	if err != nil {
		t.Fatalf("ExtractSubtitleItems failed: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}

	item := items[0]
	if item.Index != 1 {
		t.Errorf("expected Index 1, got %d", item.Index)
	}
	if item.StartTime != time.Second {
		t.Errorf("expected StartTime 1s, got %v", item.StartTime)
	}
	if item.EndTime != 3*time.Second {
		t.Errorf("expected EndTime 3s, got %v", item.EndTime)
	}
	if item.X != 100 || item.Y != 200 {
		t.Errorf("expected (100, 200), got (%d, %d)", item.X, item.Y)
	}
	if len(item.PNGData) == 0 {
		t.Errorf("expected non-empty PNG data")
	}
	if item.Color.R < 250 || item.Color.A != 255 {
		t.Errorf("expected red color, got %+v", item.Color)
	}
}

func TestRenderDisplaySet_TransparentPadding(t *testing.T) {
	// A 10x10 object placed at (100, 200) where only (4..5, 4..5) has visible pixels
	var pal Palette
	pal.Entries[1] = PaletteEntry{Y: 81, Cb: 90, Cr: 240, Alpha: 255} // Visible
	pal.Entries[0] = PaletteEntry{Alpha: 0}                           // Transparent

	// Rows 0..3: empty lines (padded with 0 to 10 pixels each)
	// Row 4: 4 zeroes (0x00, 0x04), 2 ones (0x01, 0x01), EOL (0x00, 0x00) -> padded to 10
	// Row 5: 4 zeroes (0x00, 0x04), 2 ones (0x01, 0x01), EOL (0x00, 0x00) -> padded to 10
	// Rows 6..9: empty lines
	var rleData []byte
	for row := 0; row < 4; row++ {
		rleData = append(rleData, 0x00, 0x00)
	}
	for row := 4; row < 6; row++ {
		rleData = append(rleData, 0x00, 0x04, 0x01, 0x01, 0x00, 0x00)
	}
	for row := 6; row < 10; row++ {
		rleData = append(rleData, 0x00, 0x00)
	}

	set := DisplaySet{
		PCS: PCS{
			Width:  1920,
			Height: 1080,
			Objects: []CompositionObject{
				{ObjectID: 1, X: 100, Y: 200},
			},
		},
		Palette: pal,
		Objects: []ObjectData{
			{ID: 1, Width: 10, Height: 10, Data: rleData},
		},
	}

	pngData, col, x, y, w, h, err := RenderDisplaySet(&set)
	if err != nil {
		t.Fatalf("RenderDisplaySet failed: %v", err)
	}

	if col.A != 255 {
		t.Errorf("expected visible color, got %+v", col)
	}
	// Bounding box must be tightened to the visible 2x2 area: X=104, Y=200, W=2, H=2
	if x != 104 || y != 200 {
		t.Errorf("expected tight origin (104, 200), got (%d, %d)", x, y)
	}
	if w != 2 || h != 2 {
		t.Errorf("expected tight dimensions (2, 2), got (%d, %d)", w, h)
	}
	if len(pngData) == 0 {
		t.Errorf("expected non-empty PNG data")
	}
}

func TestRenderDisplaySet_CroppedFlag(t *testing.T) {
	// A 10x10 object where comp has CroppedFlag: true
	var pal Palette
	pal.Entries[1] = PaletteEntry{Y: 81, Cb: 90, Cr: 240, Alpha: 255}

	// 100 pixels of color 1 (0x00, 0xC0, 100, 0x01)
	rleData := []byte{0x00, 0x80 | 0x40, 0x00, 100, 0x01}

	set := DisplaySet{
		PCS: PCS{
			Width:  1920,
			Height: 1080,
			Objects: []CompositionObject{
				{
					ObjectID:    1,
					X:           50,
					Y:           80,
					CroppedFlag: true,
					CropX:       2,
					CropY:       3,
					CropWidth:   4,
					CropHeight:  5,
				},
			},
		},
		Palette: pal,
		Objects: []ObjectData{
			{ID: 1, Width: 10, Height: 10, Data: rleData},
		},
	}

	_, _, x, y, w, h, err := RenderDisplaySet(&set)
	if err != nil {
		t.Fatalf("RenderDisplaySet failed: %v", err)
	}

	if x != 50 || y != 80 {
		t.Errorf("expected origin (50, 80), got (%d, %d)", x, y)
	}
	if w != 4 || h != 5 {
		t.Errorf("expected cropped dimensions (4, 5), got (%d, %d)", w, h)
	}
}

func TestRenderDisplaySet_MultipleObjectsUnordered(t *testing.T) {
	var pal Palette
	pal.Entries[1] = PaletteEntry{Y: 81, Cb: 90, Cr: 240, Alpha: 255}

	// Two objects: Obj 1 at (300, 400), Obj 2 at (100, 200)
	objData := []byte{0x00, 0x84, 0x01} // 4 pixels of color 1 (2x2)

	set := DisplaySet{
		PCS: PCS{
			Width:  1920,
			Height: 1080,
			Objects: []CompositionObject{
				{ObjectID: 1, X: 300, Y: 400},
				{ObjectID: 2, X: 100, Y: 200},
			},
		},
		Palette: pal,
		Objects: []ObjectData{
			{ID: 1, Width: 2, Height: 2, Data: objData},
			{ID: 2, Width: 2, Height: 2, Data: objData},
		},
	}

	_, _, x, y, w, h, err := RenderDisplaySet(&set)
	if err != nil {
		t.Fatalf("RenderDisplaySet failed: %v", err)
	}

	// minX must be 100, minY must be 200, maxX must be 302, maxY must be 402
	if x != 100 || y != 200 {
		t.Errorf("expected origin (100, 200), got (%d, %d)", x, y)
	}
	if w != 202 || h != 202 {
		t.Errorf("expected dimensions (202, 202), got (%d, %d)", w, h)
	}
}

func TestDominantColor_OutlineIgnored(t *testing.T) {
	var pal Palette
	// Entry 0: transparent
	pal.Entries[0] = PaletteEntry{Alpha: 0}
	// Entry 1: black outline (Y=16, Cb=128, Cr=128, Alpha=255)
	pal.Entries[1] = PaletteEntry{Y: 16, Cb: 128, Cr: 128, Alpha: 255}
	// Entry 2: white text (Y=235, Cb=128, Cr=128, Alpha=255)
	pal.Entries[2] = PaletteEntry{Y: 235, Cb: 128, Cr: 128, Alpha: 255}

	var usageCounts [256]int
	// Outline has 5,000 pixels, white text has 2,000 pixels
	usageCounts[1] = 5000
	usageCounts[2] = 2000

	domColor := DominantColor(&pal, usageCounts)
	// Must pick white text (Y=235), not black outline (Y=16)
	if domColor.R < 200 || domColor.G < 200 || domColor.B < 200 {
		t.Errorf("expected dominant color to be white text, got %+v", domColor)
	}
}

