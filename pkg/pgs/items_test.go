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

