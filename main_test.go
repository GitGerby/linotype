package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/log"
	"git.dyson.somuchcrypto.com/gearnsc/linotype/pkg/config"
	"git.dyson.somuchcrypto.com/gearnsc/linotype/pkg/ocr"
	"git.dyson.somuchcrypto.com/gearnsc/linotype/pkg/pgs"
	"git.dyson.somuchcrypto.com/gearnsc/linotype/pkg/subtitle"
)

func buildTestSUP(t *testing.T) []byte {
	var stream []byte

	buildSeg := func(segType byte, data []byte, pts uint32) []byte {
		var header [13]byte
		header[0] = 0x50
		header[1] = 0x47
		binary.BigEndian.PutUint32(header[2:6], pts)
		binary.BigEndian.PutUint32(header[6:10], 0)
		header[10] = segType
		binary.BigEndian.PutUint16(header[11:13], uint16(len(data)))
		return append(header[:], data...)
	}

	// 1. PCS at 1s (90,000 clock ticks)
	pcsData := make([]byte, 19)
	binary.BigEndian.PutUint16(pcsData[0:2], 1920) // Width
	binary.BigEndian.PutUint16(pcsData[2:4], 1080) // Height
	pcsData[4] = 24                              // FrameRate
	binary.BigEndian.PutUint16(pcsData[5:7], 1)     // CompositionNumber
	pcsData[7] = 0x80                            // State: Epoch Start
	pcsData[8] = 0                               // PaletteUpdateFlag
	pcsData[9] = 1                               // PaletteID
	pcsData[10] = 1                              // Num Objects: 1
	// Comp object 0: ObjectID=1, WindowID=0, CroppedFlag=0, X=200, Y=800
	binary.BigEndian.PutUint16(pcsData[11:13], 1)
	pcsData[13] = 0
	pcsData[14] = 0
	binary.BigEndian.PutUint16(pcsData[15:17], 200)
	binary.BigEndian.PutUint16(pcsData[17:19], 800)
	stream = append(stream, buildSeg(pgs.SegmentTypePCS, pcsData, 90000)...)

	// 2. WDS
	wdsData := []byte{1, 0, 0, 0, 0, 0, 1920 / 256, 1920 % 256, 1080 / 256, 1080 % 256}
	stream = append(stream, buildSeg(pgs.SegmentTypeWDS, wdsData, 90000)...)

	// 3. PDS: Palette 1, index 1 = yellow
	pdsData := []byte{1, 0, 1, 210, 16, 146, 255}
	stream = append(stream, buildSeg(pgs.SegmentTypePDS, pdsData, 90000)...)

	// 4. ODS: Object 1, 4x2 pixels
	odsData := make([]byte, 11)
	binary.BigEndian.PutUint16(odsData[0:2], 1)
	odsData[2] = 0
	odsData[3] = 0xC0
	binary.BigEndian.PutUint16(odsData[5:7], 3)
	binary.BigEndian.PutUint16(odsData[7:9], 4) // Width
	binary.BigEndian.PutUint16(odsData[9:11], 2) // Height
	odsData = append(odsData, []byte{0x00, 0x88, 0x01}...) // 8 pixels of color 1
	stream = append(stream, buildSeg(pgs.SegmentTypeODS, odsData, 90000)...)

	// 5. END
	stream = append(stream, buildSeg(pgs.SegmentTypeEND, nil, 90000)...)

	// 6. Clearance set at 3s (270,000 ticks)
	pcsClear := make([]byte, 11)
	binary.BigEndian.PutUint16(pcsClear[0:2], 1920)
	binary.BigEndian.PutUint16(pcsClear[2:4], 1080)
	pcsClear[4] = 24
	binary.BigEndian.PutUint16(pcsClear[5:7], 2)
	pcsClear[7] = 0x00 // Normal
	pcsClear[10] = 0   // 0 objects
	stream = append(stream, buildSeg(pgs.SegmentTypePCS, pcsClear, 270000)...)
	stream = append(stream, buildSeg(pgs.SegmentTypeEND, nil, 270000)...)

	return stream
}

func TestProcessSUPEndToEnd(t *testing.T) {
	// Mock OpenAI endpoint
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]string{
						"content": "Hello Linotype Subtitle!",
					},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	tempDir := t.TempDir()
	supPath := filepath.Join(tempDir, "test.sup")
	supBytes := buildTestSUP(t)
	if err := os.WriteFile(supPath, supBytes, 0644); err != nil {
		t.Fatalf("failed to write test.sup: %v", err)
	}

	logger := log.New(bytes.NewBuffer(nil))
	ocrClient := ocr.NewClient(server.URL, "test-key", "gpt-4o")

	// 1. Test ASS output
	assPath := filepath.Join(tempDir, "test.ass")
	cfgASS := &config.Config{
		Format:  "ass",
		Workers: 2,
	}
	cfgASS.OCR.Language = "English"

	err := processSUP(context.Background(), supPath, assPath, cfgASS, ocrClient, "ocr prompt", true, logger, Dimensions{})
	if err != nil {
		t.Fatalf("processSUP ASS failed: %v", err)
	}

	assContent, err := os.ReadFile(assPath)
	if err != nil {
		t.Fatalf("failed to read generated ASS: %v", err)
	}

	assStr := string(assContent)
	if !strings.Contains(assStr, "[Script Info]") {
		t.Errorf("ASS file missing [Script Info]")
	}
	if !strings.Contains(assStr, "Hello Linotype Subtitle!") {
		t.Errorf("ASS file missing OCR text: %s", assStr)
	}
	if !strings.Contains(assStr, "\\pos(200,802)") {
		t.Errorf("ASS file missing \\pos(200,802): %s", assStr)
	}

	// 2. Test WebVTT output
	vttPath := filepath.Join(tempDir, "test.vtt")
	cfgVTT := &config.Config{
		Format:  "webvtt",
		Workers: 2,
	}
	cfgVTT.OCR.Language = "English"

	err = processSUP(context.Background(), supPath, vttPath, cfgVTT, ocrClient, "ocr prompt", true, logger, Dimensions{})
	if err != nil {
		t.Fatalf("processSUP WebVTT failed: %v", err)
	}

	vttContent, err := os.ReadFile(vttPath)
	if err != nil {
		t.Fatalf("failed to read generated WebVTT: %v", err)
	}

	vttStr := string(vttContent)
	if !strings.HasPrefix(vttStr, "WEBVTT") {
		t.Errorf("WebVTT file missing header: %s", vttStr)
	}
	if !strings.Contains(vttStr, "Hello Linotype Subtitle!") {
		t.Errorf("WebVTT file missing OCR text: %s", vttStr)
	}
	if !strings.Contains(vttStr, "--> 00:00:03.000") {
		t.Errorf("WebVTT file missing end time: %s", vttStr)
	}
}

func TestProcessSUP_4KScaling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]string{
						"content": "4K Scaled Subtitle",
					},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	tempDir := t.TempDir()
	supPath := filepath.Join(tempDir, "test.sup")
	supBytes := buildTestSUP(t) // PGS has PCS Width=1920, Height=1080, X=200, Y=800
	if err := os.WriteFile(supPath, supBytes, 0644); err != nil {
		t.Fatalf("failed to write test.sup: %v", err)
	}

	logger := log.New(bytes.NewBuffer(nil))
	ocrClient := ocr.NewClient(server.URL, "test-key", "gpt-4o")

	assPath := filepath.Join(tempDir, "test_4k.ass")
	cfgASS := &config.Config{
		Format:  "ass",
		Workers: 2,
	}
	cfgASS.OCR.Language = "English"

	// Pass 4K video dimensions: 3840 x 2160
	err := processSUP(context.Background(), supPath, assPath, cfgASS, ocrClient, "ocr prompt", true, logger, Dimensions{Width: 3840, Height: 2160})
	if err != nil {
		t.Fatalf("processSUP 4K ASS failed: %v", err)
	}

	assContent, err := os.ReadFile(assPath)
	if err != nil {
		t.Fatalf("failed to read generated ASS: %v", err)
	}

	assStr := string(assContent)
	if !strings.Contains(assStr, "PlayResX: 3840") {
		t.Errorf("ASS file missing PlayResX: 3840, got:\n%s", assStr)
	}
	if !strings.Contains(assStr, "PlayResY: 2160") {
		t.Errorf("ASS file missing PlayResY: 2160, got:\n%s", assStr)
	}
	if !strings.Contains(assStr, "Style: Default,Arial,96") {
		t.Errorf("ASS file missing Style with Fontsize 96, got:\n%s", assStr)
	}
	// Original was X=200, Y=800, W=4, H=2 at 1080p. Scaled 2x to 4K: X=400, Y=1600, W=8, H=4. Anchor for \an1: (400, 1604).
	if !strings.Contains(assStr, "\\pos(400,1604)") {
		t.Errorf("ASS file missing \\pos(400,1604), got:\n%s", assStr)
	}
	if !strings.Contains(assStr, "4K Scaled Subtitle") {
		t.Errorf("ASS file missing OCR text: %s", assStr)
	}
}

func TestDimensions(t *testing.T) {
	d := Dimensions{Width: 1920, Height: 1080}
	if d.Width != 1920 || d.Height != 1080 {
		t.Errorf("expected 1920x1080, got %dx%d", d.Width, d.Height)
	}

	if !(Dimensions{}).IsZero() {
		t.Errorf("expected zero Dimensions{} to be IsZero()")
	}
	if !(Dimensions{Width: 0, Height: 1080}).IsZero() {
		t.Errorf("expected 0 width to be IsZero()")
	}
	if !(Dimensions{Width: 1920, Height: 0}).IsZero() {
		t.Errorf("expected 0 height to be IsZero()")
	}
	if !(Dimensions{Width: -1, Height: 1080}).IsZero() {
		t.Errorf("expected negative width to be IsZero()")
	}
	if !(Dimensions{Width: 1920, Height: -1}).IsZero() {
		t.Errorf("expected negative height to be IsZero()")
	}
	if !(Dimensions{Width: -1, Height: -1}).IsZero() {
		t.Errorf("expected negative width and height to be IsZero()")
	}
	if d.IsZero() {
		t.Errorf("expected 1920x1080 to not be IsZero()")
	}
}

func TestProcessSUP_Downscaling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]string{
						"content": "720p Subtitle",
					},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	tempDir := t.TempDir()
	supPath := filepath.Join(tempDir, "test.sup")
	supBytes := buildTestSUP(t) // PGS has PCS Width=1920, Height=1080, X=200, Y=800
	if err := os.WriteFile(supPath, supBytes, 0644); err != nil {
		t.Fatalf("failed to write test.sup: %v", err)
	}

	logger := log.New(bytes.NewBuffer(nil))
	ocrClient := ocr.NewClient(server.URL, "test-key", "gpt-4o")

	assPath := filepath.Join(tempDir, "test_720p.ass")
	cfgASS := &config.Config{
		Format:  "ass",
		Workers: 2,
	}
	cfgASS.OCR.Language = "English"

	// Scale 1080p down to 1280x720 (scale = 2/3)
	// X=200 -> 200 * (1280/1920) = 133
	// Y=800 -> 800 * (720/1080) = 533
	err := processSUP(context.Background(), supPath, assPath, cfgASS, ocrClient, "ocr prompt", true, logger, Dimensions{Width: 1280, Height: 720})
	if err != nil {
		t.Fatalf("processSUP 720p failed: %v", err)
	}

	assContent, err := os.ReadFile(assPath)
	if err != nil {
		t.Fatalf("failed to read generated ASS: %v", err)
	}

	assStr := string(assContent)
	if !strings.Contains(assStr, "PlayResX: 1280") {
		t.Errorf("ASS file missing PlayResX: 1280, got:\n%s", assStr)
	}
	if !strings.Contains(assStr, "PlayResY: 720") {
		t.Errorf("ASS file missing PlayResY: 720, got:\n%s", assStr)
	}
	if !strings.Contains(assStr, "\\pos(133,534)") {
		t.Errorf("ASS file missing \\pos(133,534), got:\n%s", assStr)
	}
}

func TestProcessSUP_ZeroAndNegativeDimensions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]string{
						"content": "Unscaled Subtitle",
					},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	tempDir := t.TempDir()
	supPath := filepath.Join(tempDir, "test.sup")
	supBytes := buildTestSUP(t)
	if err := os.WriteFile(supPath, supBytes, 0644); err != nil {
		t.Fatalf("failed to write test.sup: %v", err)
	}

	logger := log.New(bytes.NewBuffer(nil))
	ocrClient := ocr.NewClient(server.URL, "test-key", "gpt-4o")

	for _, dim := range []Dimensions{
		{},
		{Width: 0, Height: 0},
		{Width: -100, Height: -100},
		{Width: 1920, Height: 0},
		{Width: 0, Height: 1080},
		{Width: -1920, Height: 1080},
		{Width: 1920, Height: -1080},
	} {
		assPath := filepath.Join(tempDir, "test_unscaled.ass")
		cfgASS := &config.Config{
			Format:  "ass",
			Workers: 2,
		}
		cfgASS.OCR.Language = "English"

		err := processSUP(context.Background(), supPath, assPath, cfgASS, ocrClient, "ocr prompt", true, logger, dim)
		if err != nil {
			t.Fatalf("processSUP with dim %+v failed: %v", dim, err)
		}

		assContent, err := os.ReadFile(assPath)
		if err != nil {
			t.Fatalf("failed to read generated ASS: %v", err)
		}

		assStr := string(assContent)
		if !strings.Contains(assStr, "PlayResX: 1920") {
			t.Errorf("dim %+v: ASS file should keep original PlayResX: 1920", dim)
		}
		if !strings.Contains(assStr, "\\pos(200,802)") {
			t.Errorf("dim %+v: ASS file should keep original unscaled \\pos(200,802)", dim)
		}
	}
}

func TestProcessSubtitleItems_NonstandardVobSubResolution(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]string{
						"content": "Bottom Subtitle Text",
					},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	tempDir := t.TempDir()
	logger := log.New(bytes.NewBuffer(nil))
	ocrClient := ocr.NewClient(server.URL, "test-key", "gpt-4o")

	// DVD 640x480 item placed near bottom (Y=400 in 480 height = 83.3%)
	items := []subtitle.Item{
		{
			Index:       1,
			StartTime:   1000 * 1000 * 1000,
			EndTime:     2000 * 1000 * 1000,
			X:           50,
			Y:           400,
			Width:       200,
			Height:      40,
			VideoWidth:  640,
			VideoHeight: 480,
			PNGData:     []byte{1, 2, 3},
		},
	}

	targetDim := Dimensions{Width: 1440, Height: 960}

	// 1. ASS output verification
	assPath := filepath.Join(tempDir, "vobsub_scaled.ass")
	cfgASS := &config.Config{
		Format:  "ass",
		Workers: 1,
	}
	cfgASS.OCR.Language = "English"

	err := processSubtitleItems(context.Background(), items, assPath, cfgASS, ocrClient, "prompt", true, logger, targetDim)
	if err != nil {
		t.Fatalf("processSubtitleItems ASS failed: %v", err)
	}

	assBytes, err := os.ReadFile(assPath)
	if err != nil {
		t.Fatalf("failed to read ASS file: %v", err)
	}
	assStr := string(assBytes)

	if !strings.Contains(assStr, "PlayResX: 1440") {
		t.Errorf("ASS file missing PlayResX: 1440, got:\n%s", assStr)
	}
	if !strings.Contains(assStr, "PlayResY: 960") {
		t.Errorf("ASS file missing PlayResY: 960, got:\n%s", assStr)
	}
	// Scaled Y = 400 * (960/480) = 800, Height = 40 * 2 = 80.
	// centerY = 800 + 40 = 840 >= 640 (2/3 of 960), so it must be row 1 (bottom alignment: \an1, \an2, or \an3).
	// It must NOT be vertically centered (an4, an5, an6).
	if strings.Contains(assStr, "\\an4") || strings.Contains(assStr, "\\an5") || strings.Contains(assStr, "\\an6") {
		t.Errorf("ASS subtitle incorrectly centered vertically:\n%s", assStr)
	}
	// For bottom row, anchor Y is y + height = 800 + 80 = 880.
	if !strings.Contains(assStr, ",880)") {
		t.Errorf("ASS subtitle anchor Y should be 880 at the bottom, got:\n%s", assStr)
	}

	// 2. WebVTT output verification
	vttPath := filepath.Join(tempDir, "vobsub_scaled.vtt")
	cfgVTT := &config.Config{
		Format:  "webvtt",
		Workers: 1,
	}
	cfgVTT.OCR.Language = "English"

	err = processSubtitleItems(context.Background(), items, vttPath, cfgVTT, ocrClient, "prompt", true, logger, targetDim)
	if err != nil {
		t.Fatalf("processSubtitleItems WebVTT failed: %v", err)
	}

	vttBytes, err := os.ReadFile(vttPath)
	if err != nil {
		t.Fatalf("failed to read WebVTT file: %v", err)
	}
	vttStr := string(vttBytes)

	// Scaled Y = 800 in 960 => 800*100/960 = 83%.
	// If unscaled, Y=400 in 960 => 400*100/960 = 41% (which was the bug: vertically centered).
	if strings.Contains(vttStr, "line:41%") || strings.Contains(vttStr, "line:42%") {
		t.Errorf("WebVTT subtitle incorrectly centered vertically (~41%%):\n%s", vttStr)
	}
	if !strings.Contains(vttStr, "line:83%") {
		t.Errorf("WebVTT subtitle missing line:83%%, got:\n%s", vttStr)
	}
}

func TestProcessSubtitleItems_PALVobSubScaling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]string{
						"content": "PAL Bottom Subtitle",
					},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	tempDir := t.TempDir()
	logger := log.New(bytes.NewBuffer(nil))
	ocrClient := ocr.NewClient(server.URL, "test-key", "gpt-4o")

	// PAL 720x576 item placed near bottom (Y=490 in 576 height)
	items := []subtitle.Item{
		{
			Index:       1,
			StartTime:   1000 * 1000 * 1000,
			EndTime:     2000 * 1000 * 1000,
			X:           60,
			Y:           490,
			Width:       200,
			Height:      40,
			VideoWidth:  720,
			VideoHeight: 576,
			PNGData:     []byte{1, 2, 3},
		},
	}

	targetDim := Dimensions{Width: 1440, Height: 960}

	// 1. ASS output verification
	assPath := filepath.Join(tempDir, "pal_scaled.ass")
	cfgASS := &config.Config{
		Format:  "ass",
		Workers: 1,
	}
	cfgASS.OCR.Language = "English"

	err := processSubtitleItems(context.Background(), items, assPath, cfgASS, ocrClient, "prompt", true, logger, targetDim)
	if err != nil {
		t.Fatalf("processSubtitleItems PAL ASS failed: %v", err)
	}

	assBytes, err := os.ReadFile(assPath)
	if err != nil {
		t.Fatalf("failed to read ASS file: %v", err)
	}
	assStr := string(assBytes)

	if !strings.Contains(assStr, "PlayResX: 1440") || !strings.Contains(assStr, "PlayResY: 960") {
		t.Errorf("ASS file missing 1440x960 PlayRes:\n%s", assStr)
	}
	// Must not be vertically centered (\an4, \an5, \an6)
	if strings.Contains(assStr, "\\an4") || strings.Contains(assStr, "\\an5") || strings.Contains(assStr, "\\an6") {
		t.Errorf("PAL ASS subtitle incorrectly centered vertically:\n%s", assStr)
	}

	// 2. WebVTT output verification
	vttPath := filepath.Join(tempDir, "pal_scaled.vtt")
	cfgVTT := &config.Config{
		Format:  "webvtt",
		Workers: 1,
	}
	cfgVTT.OCR.Language = "English"

	err = processSubtitleItems(context.Background(), items, vttPath, cfgVTT, ocrClient, "prompt", true, logger, targetDim)
	if err != nil {
		t.Fatalf("processSubtitleItems PAL WebVTT failed: %v", err)
	}

	vttBytes, err := os.ReadFile(vttPath)
	if err != nil {
		t.Fatalf("failed to read WebVTT file: %v", err)
	}
	vttStr := string(vttBytes)

	// Y = round(490 * 960 / 576) = 817. linePct = 817 * 100 / 960 = 85%.
	if !strings.Contains(vttStr, "line:85%") {
		t.Errorf("PAL WebVTT subtitle missing line:85%%, got:\n%s", vttStr)
	}
}

func TestProcessSubtitleItems_PALFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]string{
						"content": "PAL Fallback Subtitle",
					},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	tempDir := t.TempDir()
	logger := log.New(bytes.NewBuffer(nil))
	ocrClient := ocr.NewClient(server.URL, "test-key", "gpt-4o")

	// Subtitle with VideoWidth=0, VideoHeight=0 and Y=500, H=40 (exceeds 480, fits in 576)
	items := []subtitle.Item{
		{
			Index:       1,
			StartTime:   1000 * 1000 * 1000,
			EndTime:     2000 * 1000 * 1000,
			X:           50,
			Y:           500,
			Width:       200,
			Height:      40,
			VideoWidth:  0,
			VideoHeight: 0,
			PNGData:     []byte{1, 2, 3},
		},
	}

	// 1. ASS output verification
	assPath := filepath.Join(tempDir, "pal_fallback.ass")
	cfgASS := &config.Config{
		Format:  "ass",
		Workers: 1,
	}
	cfgASS.OCR.Language = "English"

	err := processSubtitleItems(context.Background(), items, assPath, cfgASS, ocrClient, "prompt", true, logger, Dimensions{})
	if err != nil {
		t.Fatalf("processSubtitleItems PAL fallback ASS failed: %v", err)
	}

	assBytes, err := os.ReadFile(assPath)
	if err != nil {
		t.Fatalf("failed to read ASS file: %v", err)
	}
	assStr := string(assBytes)

	// Should fallback to 720x576 (PAL), not 1920x1080
	if !strings.Contains(assStr, "PlayResX: 720") || !strings.Contains(assStr, "PlayResY: 576") {
		t.Errorf("ASS file expected 720x576 PlayRes, got:\n%s", assStr)
	}
	if strings.Contains(assStr, "PlayResX: 1920") || strings.Contains(assStr, "PlayResY: 1080") {
		t.Errorf("ASS file incorrectly fell back to 1080p for PAL bounds:\n%s", assStr)
	}

	// 2. WebVTT output verification
	vttPath := filepath.Join(tempDir, "pal_fallback.vtt")
	cfgVTT := &config.Config{
		Format:  "webvtt",
		Workers: 1,
	}
	cfgVTT.OCR.Language = "English"

	err = processSubtitleItems(context.Background(), items, vttPath, cfgVTT, ocrClient, "prompt", true, logger, Dimensions{})
	if err != nil {
		t.Fatalf("processSubtitleItems PAL fallback WebVTT failed: %v", err)
	}

	vttBytes, err := os.ReadFile(vttPath)
	if err != nil {
		t.Fatalf("failed to read WebVTT file: %v", err)
	}
	vttStr := string(vttBytes)

	// Y=500 in 576 => 500*100/576 = 86%. If 1080, 500*100/1080 = 46% (centered)
	if strings.Contains(vttStr, "line:46%") {
		t.Errorf("WebVTT subtitle incorrectly centered using 1080p fallback:\n%s", vttStr)
	}
	if !strings.Contains(vttStr, "line:86%") {
		t.Errorf("WebVTT subtitle missing line:86%%, got:\n%s", vttStr)
	}
}


