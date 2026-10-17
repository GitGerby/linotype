package vobsub

import (
	"image/color"
	"testing"
)

func TestDefaultPalette(t *testing.T) {
	p := DefaultPalette()
	if p == nil {
		t.Fatal("expected non-nil default palette")
	}

	// Index 0 should be transparent
	if p[0].A != 0 {
		t.Errorf("expected p[0] alpha 0 (transparent), got %d", p[0].A)
	}

	// Index 1 (pattern/text body) MUST be high-contrast white, NOT black
	if p[1] != (color.RGBA{255, 255, 255, 255}) {
		t.Errorf("expected p[1] to be white text {255, 255, 255, 255}, got %+v", p[1])
	}

	// Index 2 (emphasis 1/outline) MUST be black
	if p[2] != (color.RGBA{0, 0, 0, 255}) {
		t.Errorf("expected p[2] to be black outline {0, 0, 0, 255}, got %+v", p[2])
	}

	// Index 3 (shadow) MUST be grey
	if p[3] != (color.RGBA{128, 128, 128, 255}) {
		t.Errorf("expected p[3] to be grey shadow {128, 128, 128, 255}, got %+v", p[3])
	}

	// Remaining indices must not be all-black to prevent accidental black text
	hasBrightFallback := false
	for i := 4; i < 16; i++ {
		if p[i].R > 200 && p[i].G > 200 {
			hasBrightFallback = true
			break
		}
	}
	if !hasBrightFallback {
		t.Errorf("expected high-contrast fallbacks in p[4..15]")
	}

	if !p.HasContrast() {
		t.Errorf("expected HasContrast() to be true for DefaultPalette")
	}
}

func TestParsePalette_AllBlackFallback(t *testing.T) {
	// A dummy or corrupt all-black palette must fall back to DefaultPalette (white text)
	allBlackLine := "palette: 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000"
	p, err := ParsePalette(allBlackLine)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil palette")
	}

	// p[1] should be white fallback, preventing transparent background + black text
	if p[1] != (color.RGBA{255, 255, 255, 255}) {
		t.Errorf("expected fallback to white text for all-black palette, got %+v", p[1])
	}
	if !p.HasContrast() {
		t.Errorf("expected palette after fallback to have contrast")
	}
}

func TestParsePalette_HexPrefixes(t *testing.T) {
	line := "palette: 0x000000, 0x7f7f7f, 0xe0e0e0, 0x000000, 0x000000, 0x000000, 0x000000, 0x000000, 0x000000, 0x000000, 0x000000, 0x000000, 0x000000, 0x000000, 0x000000, 0x000000"
	p, err := ParsePalette(line)
	if err != nil {
		t.Fatalf("unexpected error with 0x prefixes: %v", err)
	}
	if p[1].R != 0x7f {
		t.Errorf("expected 0x7f for R, got %x", p[1].R)
	}
}

func TestParsePalette_HashPrefixes(t *testing.T) {
	line := "palette: #000000, #7f7f7f, #e0e0e0, #000000, #000000, #000000, #000000, #000000, #000000, #000000, #000000, #000000, #000000, #000000, #000000, #000000"
	p, err := ParsePalette(line)
	if err != nil {
		t.Fatalf("unexpected error with # prefixes: %v", err)
	}
	if p[1].R != 0x7f {
		t.Errorf("expected 0x7f for R, got %x", p[1].R)
	}
}

func TestParsePalette_InlineCommentAndTrailingComma(t *testing.T) {
	line := "palette: 000000, 7f7f7f, e0e0e0, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, # DVD original palette"
	p, err := ParsePalette(line)
	if err != nil {
		t.Fatalf("unexpected error with inline comment and trailing comma: %v", err)
	}
	if p[1].R != 0x7f {
		t.Errorf("expected 0x7f for R, got %x", p[1].R)
	}
}

func TestParsePalette_FourColors(t *testing.T) {
	line := "palette: 000000, ffffff, 000000, 808080"
	p, err := ParsePalette(line)
	if err != nil {
		t.Fatalf("unexpected error for 4-color palette: %v", err)
	}
	if p[1] != (color.RGBA{255, 255, 255, 255}) {
		t.Errorf("expected white for p[1], got %+v", p[1])
	}
	if p[2] != (color.RGBA{0, 0, 0, 255}) {
		t.Errorf("expected black for p[2], got %+v", p[2])
	}
}

func TestParsePalette_SpaceDelimited(t *testing.T) {
	line := "palette: 000000 7f7f7f e0e0e0 000000 000000 000000 000000 000000 000000 000000 000000 000000 000000 000000 000000 000000"
	p, err := ParsePalette(line)
	if err != nil {
		t.Fatalf("unexpected error for space-delimited palette: %v", err)
	}
	if p[1].R != 0x7f {
		t.Errorf("expected 0x7f for R, got %x", p[1].R)
	}
}

func TestParsePalette_SemicolonDelimited(t *testing.T) {
	line := "palette: 000000; 7f7f7f; e0e0e0; 000000; 000000; 000000; 000000; 000000; 000000; 000000; 000000; 000000; 000000; 000000; 000000; 000000"
	p, err := ParsePalette(line)
	if err != nil {
		t.Fatalf("unexpected error for semicolon-delimited palette: %v", err)
	}
	if p[1].R != 0x7f {
		t.Errorf("expected 0x7f for R, got %x", p[1].R)
	}
}

func TestParsePalette_ShortHex(t *testing.T) {
	line := "palette: #000, #fff, #000, #888"
	p, err := ParsePalette(line)
	if err != nil {
		t.Fatalf("unexpected error for short hex palette: %v", err)
	}
	if p[1] != (color.RGBA{255, 255, 255, 255}) {
		t.Errorf("expected white for p[1], got %+v", p[1])
	}
}

func TestParsePalette_HashPrefixesWithComment(t *testing.T) {
	line := "palette: #000000, #7f7f7f, #e0e0e0, #000000, #000000, #000000, #000000, #000000, #000000, #000000, #000000, #000000, #000000, #000000, #000000, #000000 # DVD palette with hash"
	p, err := ParsePalette(line)
	if err != nil {
		t.Fatalf("unexpected error with # prefixes and inline comment: %v", err)
	}
	if p[1].R != 0x7f {
		t.Errorf("expected 0x7f for R, got %x", p[1].R)
	}
}

func TestParsePalette_ShortHexWithComment(t *testing.T) {
	line := "palette: #000, #fff, #000, #888 # 4-color short hex palette"
	p, err := ParsePalette(line)
	if err != nil {
		t.Fatalf("unexpected error for short hex palette with comment: %v", err)
	}
	if p[1] != (color.RGBA{255, 255, 255, 255}) {
		t.Errorf("expected white for p[1], got %+v", p[1])
	}
}

func TestParsePalette_NoPrefixDirectColors(t *testing.T) {
	line := "#000000, #7f7f7f, #e0e0e0, #000000"
	p, err := ParsePalette(line)
	if err != nil {
		t.Fatalf("unexpected error for direct colors: %v", err)
	}
	if p[1].R != 0x7f {
		t.Errorf("expected 0x7f for R, got %x", p[1].R)
	}
}

func TestParsePalette_SpaceDelimitedHashPrefixes(t *testing.T) {
	line := "palette: #000000 #7f7f7f #e0e0e0 #000000 #000000 #000000 #000000 #000000 #000000 #000000 #000000 #000000 #000000 #000000 #000000 #000000 # space-delimited"
	p, err := ParsePalette(line)
	if err != nil {
		t.Fatalf("unexpected error for space delimited with hash: %v", err)
	}
	if p[1].R != 0x7f {
		t.Errorf("expected 0x7f for R, got %x", p[1].R)
	}
}

func TestParsePalette_CommentMimickingHex(t *testing.T) {
	line := "palette: #000, #fff, #000, #888 #bad timing"
	p, err := ParsePalette(line)
	if err != nil {
		t.Fatalf("unexpected error parsing palette with #bad timing comment: %v", err)
	}
	if p[1] != (color.RGBA{255, 255, 255, 255}) {
		t.Errorf("expected white for p[1], got %+v", p[1])
	}
}

func TestParsePalette_CommentMimickingHexWithComma(t *testing.T) {
	line := "palette: #000, #fff, #000, #888 #abc, comment"
	p, err := ParsePalette(line)
	if err != nil {
		t.Fatalf("unexpected error parsing palette with #abc, comment: %v", err)
	}
	if p[1] != (color.RGBA{255, 255, 255, 255}) {
		t.Errorf("expected white for p[1], got %+v", p[1])
	}
}

func TestParsePalette_CommentMimickingHexWithSemicolon(t *testing.T) {
	line := "palette: #000, #fff, #000, #888 #fff; note"
	p, err := ParsePalette(line)
	if err != nil {
		t.Fatalf("unexpected error parsing palette with #fff; note: %v", err)
	}
	if p[1] != (color.RGBA{255, 255, 255, 255}) {
		t.Errorf("expected white for p[1], got %+v", p[1])
	}
}

func TestParsePalette_CommentMimickingHexMultipleWords(t *testing.T) {
	line := "palette: #000, #fff, #000, #888 #123 bed and breakfast"
	p, err := ParsePalette(line)
	if err != nil {
		t.Fatalf("unexpected error parsing palette with #123 bed and breakfast comment: %v", err)
	}
	if p[1] != (color.RGBA{255, 255, 255, 255}) {
		t.Errorf("expected white for p[1], got %+v", p[1])
	}
}

func TestParsePalette_CommentAfter16Colors(t *testing.T) {
	line := "palette: #000000, #7f7f7f, #e0e0e0, #000000, #000000, #000000, #000000, #000000, #000000, #000000, #000000, #000000, #000000, #000000, #000000, #000000 #123456"
	p, err := ParsePalette(line)
	if err != nil {
		t.Fatalf("unexpected error parsing 16-color palette with hex comment: %v", err)
	}
	if p[1].R != 0x7f {
		t.Errorf("expected 0x7f for R, got %x", p[1].R)
	}
}
