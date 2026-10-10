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
	d := NewDimensions(1920, 1080)
	if d.Width != 1920 || d.Height != 1080 {
		t.Errorf("expected 1920x1080, got %dx%d", d.Width, d.Height)
	}

	if d.String() != "1920x1080" {
		t.Errorf("expected String() '1920x1080', got %q", d.String())
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

	// Verify VideoDimensions, TargetDimensions, and TargetDim aliases and constructors
	var vd VideoDimensions = d
	if vd.Width != 1920 || vd.Height != 1080 {
		t.Errorf("VideoDimensions alias mismatch")
	}

	var td TargetDimensions = d
	if td.Width != 1920 || td.Height != 1080 {
		t.Errorf("TargetDimensions alias mismatch")
	}

	var tdim TargetDim = d
	if tdim.Width != 1920 || tdim.Height != 1080 {
		t.Errorf("TargetDim alias mismatch")
	}

	tdNew := NewTargetDimensions(3840, 2160)
	if tdNew.Width != 3840 || tdNew.Height != 2160 {
		t.Errorf("NewTargetDimensions constructor mismatch")
	}

	tdimNew := NewTargetDim(3840, 2160)
	if tdimNew.Width != 3840 || tdimNew.Height != 2160 {
		t.Errorf("NewTargetDim constructor mismatch")
	}
}

func TestProcessSUP_TypeAliasesAndConstructors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]string{
						"content": "Alias Subtitle",
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

	cfgASS := &config.Config{
		Format:  "ass",
		Workers: 2,
	}
	cfgASS.OCR.Language = "English"

	// Test with VideoDimensions alias
	assPathVD := filepath.Join(tempDir, "test_vd.ass")
	err := processSUP(context.Background(), supPath, assPathVD, cfgASS, ocrClient, "ocr prompt", true, logger, VideoDimensions{Width: 3840, Height: 2160})
	if err != nil {
		t.Fatalf("processSUP VideoDimensions failed: %v", err)
	}

	// Test with TargetDimensions alias
	assPathTD := filepath.Join(tempDir, "test_td.ass")
	err = processSUP(context.Background(), supPath, assPathTD, cfgASS, ocrClient, "ocr prompt", true, logger, TargetDimensions{Width: 3840, Height: 2160})
	if err != nil {
		t.Fatalf("processSUP TargetDimensions failed: %v", err)
	}

	// Test with TargetDim alias
	assPathTDim := filepath.Join(tempDir, "test_tdim.ass")
	err = processSUP(context.Background(), supPath, assPathTDim, cfgASS, ocrClient, "ocr prompt", true, logger, TargetDim{Width: 3840, Height: 2160})
	if err != nil {
		t.Fatalf("processSUP TargetDim failed: %v", err)
	}

	// Test with NewDimensions constructor
	assPathNew := filepath.Join(tempDir, "test_new.ass")
	err = processSUP(context.Background(), supPath, assPathNew, cfgASS, ocrClient, "ocr prompt", true, logger, NewDimensions(3840, 2160))
	if err != nil {
		t.Fatalf("processSUP NewDimensions failed: %v", err)
	}

	for _, path := range []string{assPathVD, assPathTD, assPathTDim, assPathNew} {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read generated ASS: %v", err)
		}
		str := string(content)
		if !strings.Contains(str, "PlayResX: 3840") {
			t.Errorf("%s: missing PlayResX: 3840", path)
		}
		if !strings.Contains(str, "\\pos(400,1604)") {
			t.Errorf("%s: missing scaled \\pos(400,1604)", path)
		}
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

