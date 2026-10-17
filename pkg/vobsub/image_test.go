package vobsub

import (
	"image/color"
	"testing"
)

func TestRender_HighContrastEnforcedOnAllBlack(t *testing.T) {
	// Pixels: 2x2. Pixel 0 is background (0), Pixels 1, 2, 3 are text (1).
	pixels := []byte{0, 1, 1, 1}
	colors := [4]uint8{0, 1, 2, 3}
	alphas := [4]uint8{0, 255, 255, 255}

	// Broken all-black palette
	var blackPalette Palette
	for i := range blackPalette {
		blackPalette[i] = color.RGBA{0, 0, 0, 255}
	}

	img, domColor, err := Render(pixels, 2, 2, colors, alphas, &blackPalette)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	// Background pixel (0,0) must remain transparent
	bg := img.RGBAAt(0, 0)
	if bg.A != 0 {
		t.Errorf("expected background pixel to be transparent, got %+v", bg)
	}

	// Text pixel (1,0) must be elevated to bright white for OCR recognition
	textPixel := img.RGBAAt(1, 0)
	if textPixel.R < 200 || textPixel.G < 200 || textPixel.B < 200 {
		t.Errorf("expected text pixel to be bright white, got %+v", textPixel)
	}

	// Dominant color must be white
	if domColor.R < 200 || domColor.G < 200 || domColor.B < 200 {
		t.Errorf("expected dominant color to be white, got %+v", domColor)
	}
}

func TestRender_InvertedOutlineSwapped(t *testing.T) {
	// Pixel 0 is bg (0), Pixel 1 is text (1), Pixels 2, 3 are outline (2)
	pixels := []byte{0, 1, 2, 2}
	colors := [4]uint8{0, 1, 2, 3}
	alphas := [4]uint8{0, 255, 255, 255}

	// Inverted: text (1) is black, outline (2) is white
	var invPalette Palette
	invPalette[0] = color.RGBA{0, 0, 0, 0}
	invPalette[1] = color.RGBA{0, 0, 0, 255}       // text is black
	invPalette[2] = color.RGBA{255, 255, 255, 255} // outline is white

	img, domColor, err := Render(pixels, 2, 2, colors, alphas, &invPalette)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	// Text pixel (1,0) must have been swapped to bright white
	textPixel := img.RGBAAt(1, 0)
	if textPixel.R < 200 || textPixel.G < 200 || textPixel.B < 200 {
		t.Errorf("expected text pixel (slot 1) to be swapped to white, got %+v", textPixel)
	}

	// Outline pixel (0,1) should be black
	outlinePixel := img.RGBAAt(0, 1)
	if outlinePixel.R > 50 || outlinePixel.G > 50 || outlinePixel.B > 50 {
		t.Errorf("expected outline pixel (slot 2) to be swapped to black, got %+v", outlinePixel)
	}

	if domColor.R < 200 || domColor.G < 200 || domColor.B < 200 {
		t.Errorf("expected dominant color to be white, got %+v", domColor)
	}
}

func TestRender_BrightForegroundPreserved(t *testing.T) {
	// Yellow subtitle text with black outline
	pixels := []byte{0, 1, 2, 2}
	colors := [4]uint8{0, 1, 2, 3}
	alphas := [4]uint8{0, 255, 255, 255}

	var yellowPalette Palette
	yellowPalette[0] = color.RGBA{0, 0, 0, 0}
	yellowPalette[1] = color.RGBA{255, 255, 0, 255} // Yellow
	yellowPalette[2] = color.RGBA{0, 0, 0, 255}     // Black outline

	img, domColor, err := Render(pixels, 2, 2, colors, alphas, &yellowPalette)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	textPixel := img.RGBAAt(1, 0)
	// Yellow color should be preserved
	if textPixel.R < 200 || textPixel.G < 200 || textPixel.B > 50 {
		t.Errorf("expected yellow text pixel preserved, got %+v", textPixel)
	}

	if domColor.R < 200 || domColor.G < 200 || domColor.B > 50 {
		t.Errorf("expected yellow dominant color preserved, got %+v", domColor)
	}
}

func TestRender_BrightBackgroundDarkText(t *testing.T) {
	// Opaque white background box with black text
	pixels := []byte{0, 1, 1, 1}
	colors := [4]uint8{0, 1, 2, 3}
	alphas := [4]uint8{255, 255, 255, 255}

	var boxPalette Palette
	boxPalette[0] = color.RGBA{255, 255, 255, 255} // White background
	boxPalette[1] = color.RGBA{0, 0, 0, 255}       // Black text

	img, domColor, err := Render(pixels, 2, 2, colors, alphas, &boxPalette)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	bgPixel := img.RGBAAt(0, 0)
	if bgPixel.R < 200 {
		t.Errorf("expected white background, got %+v", bgPixel)
	}

	textPixel := img.RGBAAt(1, 0)
	if textPixel.R > 50 {
		t.Errorf("expected black text on white background, got %+v", textPixel)
	}

	_ = domColor
}

func TestRender_NilPalette(t *testing.T) {
	pixels := []byte{0, 1, 1, 1}
	colors := [4]uint8{0, 1, 2, 3}
	alphas := [4]uint8{0, 255, 255, 255}

	img, domColor, err := Render(pixels, 2, 2, colors, alphas, nil)
	if err != nil {
		t.Fatalf("Render with nil palette failed: %v", err)
	}

	textPixel := img.RGBAAt(1, 0)
	if textPixel.R < 200 || textPixel.G < 200 || textPixel.B < 200 {
		t.Errorf("expected white text from nil palette fallback, got %+v", textPixel)
	}
	if domColor.R < 200 {
		t.Errorf("expected white dominant color, got %+v", domColor)
	}
}

func TestRender_Slot2DominantAllBlack(t *testing.T) {
	// Pixel 0 is bg (0), Pixels 1, 2, 3 are text in slot 2 (e.g. emphasis 1 used as main text, slot 1 empty)
	pixels := []byte{0, 2, 2, 2}
	colors := [4]uint8{0, 1, 2, 3}
	alphas := [4]uint8{0, 255, 255, 255}

	var blackPalette Palette
	for i := range blackPalette {
		blackPalette[i] = color.RGBA{0, 0, 0, 255}
	}

	img, domColor, err := Render(pixels, 2, 2, colors, alphas, &blackPalette)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	// Background pixel (0,0) must remain transparent
	bg := img.RGBAAt(0, 0)
	if bg.A != 0 {
		t.Errorf("expected background pixel to be transparent, got %+v", bg)
	}

	// Slot 2 text pixel (1,0) must be elevated to bright white for OCR recognition
	textPixel := img.RGBAAt(1, 0)
	if textPixel.R < 200 || textPixel.G < 200 || textPixel.B < 200 {
		t.Errorf("expected slot 2 text pixel to be bright white, got %+v", textPixel)
	}

	// Dominant color must be white
	if domColor.R < 200 || domColor.G < 200 || domColor.B < 200 {
		t.Errorf("expected dominant color to be white, got %+v", domColor)
	}
}

func TestRender_LowAlphaTextElevated(t *testing.T) {
	pixels := []byte{0, 1, 1, 1}
	colors := [4]uint8{0, 1, 2, 3}
	alphas := [4]uint8{0, 34, 255, 255} // text has very low alpha 34

	var pal Palette
	pal[0] = color.RGBA{0, 0, 0, 0}
	pal[1] = color.RGBA{255, 255, 255, 255} // white text
	pal[2] = color.RGBA{0, 0, 0, 255}
	pal[3] = color.RGBA{128, 128, 128, 255}

	img, _, err := Render(pixels, 2, 2, colors, alphas, &pal)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	textPixel := img.RGBAAt(1, 0)
	if textPixel.A < 200 {
		t.Errorf("expected text pixel alpha elevated to opaque (>=200), got %d", textPixel.A)
	}
}

func TestRender_TransparentBrightSlotNotCountedAsVisible(t *testing.T) {
	pixels := []byte{0, 1, 2, 2}
	colors := [4]uint8{0, 1, 2, 3}
	alphas := [4]uint8{0, 255, 10, 255} // slot 2 has near-zero alpha 10

	var pal Palette
	pal[0] = color.RGBA{0, 0, 0, 0}
	pal[1] = color.RGBA{0, 0, 0, 255}       // text is black
	pal[2] = color.RGBA{255, 255, 255, 255} // outline is white, but alpha is 10 (invisible!)
	pal[3] = color.RGBA{0, 0, 0, 255}

	img, _, err := Render(pixels, 2, 2, colors, alphas, &pal)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	// Slot 1 must be elevated to white because slot 2 is transparent and cannot provide contrast
	textPixel := img.RGBAAt(1, 0)
	if textPixel.R < 200 || textPixel.G < 200 || textPixel.B < 200 {
		t.Errorf("expected text pixel to be elevated to white, got %+v", textPixel)
	}
}
