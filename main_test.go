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

	err := processSUP(context.Background(), supPath, assPath, cfgASS, ocrClient, "ocr prompt", true, logger)
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
	if !strings.Contains(assStr, "\\pos(200,800)") {
		t.Errorf("ASS file missing \\pos(200,800): %s", assStr)
	}

	// 2. Test WebVTT output
	vttPath := filepath.Join(tempDir, "test.vtt")
	cfgVTT := &config.Config{
		Format:  "webvtt",
		Workers: 2,
	}
	cfgVTT.OCR.Language = "English"

	err = processSUP(context.Background(), supPath, vttPath, cfgVTT, ocrClient, "ocr prompt", true, logger)
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
