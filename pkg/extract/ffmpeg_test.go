package extract

import (
	"context"
	"testing"
)

func TestParseFfprobeOutput(t *testing.T) {
	data := []byte(`{
		"streams": [
			{
				"index": 2,
				"codec_name": "subrip",
				"tags": { "language": "eng" }
			},
			{
				"index": 3,
				"codec_name": "hdmv_pgs_subtitle",
				"tags": { "language": "jpn", "title": "Japanese PGS" }
			}
		]
	}`)

	tracks, err := parseFfprobeOutput(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tracks) != 1 {
		t.Fatalf("expected 1 track, got %d", len(tracks))
	}

	if tracks[0].Index != 3 {
		t.Errorf("expected absolute index 3, got %d", tracks[0].Index)
	}
	if tracks[0].RelIndex != 1 {
		t.Errorf("expected relative index 1, got %d", tracks[0].RelIndex)
	}
	if tracks[0].Language != "jpn" {
		t.Errorf("expected language jpn, got %s", tracks[0].Language)
	}
}

func TestBuildFfprobeCmd(t *testing.T) {
	cmd := buildFfprobeCmd(context.Background(), "test.mkv")
	expectedArgs := []string{"ffprobe", "-v", "quiet", "-print_format", "json", "-show_streams", "-select_streams", "s", "test.mkv"}
	if len(cmd.Args) != len(expectedArgs) {
		t.Fatalf("args length mismatch")
	}
	for i, arg := range expectedArgs {
		if cmd.Args[i] != arg {
			t.Errorf("arg %d: expected %s, got %s", i, arg, cmd.Args[i])
		}
	}
}

func TestBuildFfmpegCmd(t *testing.T) {
	cmd := buildFfmpegCmd(context.Background(), "test.mkv", 2, "out.sup")
	expectedArgs := []string{"ffmpeg", "-y", "-i", "test.mkv", "-map", "0:s:2", "-c", "copy", "out.sup"}
	if len(cmd.Args) != len(expectedArgs) {
		t.Fatalf("args length mismatch")
	}
	for i, arg := range expectedArgs {
		if cmd.Args[i] != arg {
			t.Errorf("arg %d: expected %s, got %s", i, arg, cmd.Args[i])
		}
	}
}

func TestBuildFfprobeVideoCmd(t *testing.T) {
	cmd := buildFfprobeVideoCmd(context.Background(), "test.mkv")
	expectedArgs := []string{"ffprobe", "-v", "quiet", "-print_format", "json", "-show_streams", "-select_streams", "v", "test.mkv"}
	if len(cmd.Args) != len(expectedArgs) {
		t.Fatalf("args length mismatch")
	}
	for i, arg := range expectedArgs {
		if cmd.Args[i] != arg {
			t.Errorf("arg %d: expected %s, got %s", i, arg, cmd.Args[i])
		}
	}
}

func TestParseFfprobeVideoOutput(t *testing.T) {
	// Case 1: 4K video stream
	data4K := []byte(`{
		"streams": [
			{
				"index": 0,
				"codec_name": "hevc",
				"codec_type": "video",
				"width": 3840,
				"height": 2160,
				"disposition": {
					"attached_pic": 0
				}
			}
		]
	}`)
	w, h, err := parseFfprobeVideoOutput(data4K)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w != 3840 || h != 2160 {
		t.Errorf("expected 3840x2160, got %dx%d", w, h)
	}

	// Case 2: Attached cover pic ignored in favor of main video
	dataAttached := []byte(`{
		"streams": [
			{
				"index": 0,
				"codec_name": "mjpeg",
				"codec_type": "video",
				"width": 600,
				"height": 800,
				"disposition": {
					"attached_pic": 1
				}
			},
			{
				"index": 1,
				"codec_name": "hevc",
				"codec_type": "video",
				"width": 3840,
				"height": 2160,
				"disposition": {
					"attached_pic": 0
				}
			}
		]
	}`)
	w, h, err = parseFfprobeVideoOutput(dataAttached)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w != 3840 || h != 2160 {
		t.Errorf("expected 3840x2160, got %dx%d", w, h)
	}

	// Case 3: Empty streams
	dataEmpty := []byte(`{"streams": []}`)
	_, _, err = parseFfprobeVideoOutput(dataEmpty)
	if err == nil {
		t.Errorf("expected error for empty streams, got nil")
	}
}

func TestParseFfprobeOutput_SubtitleDimensions(t *testing.T) {
	data := []byte(`{
		"streams": [
			{
				"index": 2,
				"codec_name": "dvd_subtitle",
				"width": 640,
				"height": 480,
				"tags": { "language": "eng" }
			}
		]
	}`)

	tracks, err := parseFfprobeOutput(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tracks) != 1 {
		t.Fatalf("expected 1 track, got %d", len(tracks))
	}

	if tracks[0].Width != 640 || tracks[0].Height != 480 {
		t.Errorf("expected 640x480, got %dx%d", tracks[0].Width, tracks[0].Height)
	}
}

func TestParseFfprobeOutput_Extradata(t *testing.T) {
	data := []byte(`{
		"streams": [
			{
				"index": 2,
				"codec_name": "dvd_subtitle",
				"width": 640,
				"height": 480,
				"extradata": "73697a653a2036343078343830",
				"tags": { "language": "eng" }
			}
		]
	}`)

	tracks, err := parseFfprobeOutput(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tracks) != 1 {
		t.Fatalf("expected 1 track, got %d", len(tracks))
	}
	if string(tracks[0].Extradata) != "size: 640x480" {
		t.Errorf("expected Extradata 'size: 640x480', got %q", string(tracks[0].Extradata))
	}
}

func TestBuildCodecPrivateCmd(t *testing.T) {
	cmd := buildCodecPrivateCmd(context.Background(), "video.mkv", 1)
	expectedArgs := []string{"ffprobe", "-v", "quiet", "-show_entries", "stream=codec_private,extradata", "-select_streams", "s:1", "-print_format", "json", "video.mkv"}
	if len(cmd.Args) != len(expectedArgs) {
		t.Fatalf("args length mismatch: expected %d, got %d", len(expectedArgs), len(cmd.Args))
	}
	for i, arg := range expectedArgs {
		if cmd.Args[i] != arg {
			t.Errorf("arg %d: expected %s, got %s", i, arg, cmd.Args[i])
		}
	}
}

func TestParseCodecPrivateOutput(t *testing.T) {
	// Case 1: extradata with hex
	jsonExtradata := []byte(`{
		"streams": [
			{
				"extradata": "73697a653a2036343078343830"
			}
		]
	}`)
	out, err := parseCodecPrivateOutput(jsonExtradata)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(out) != "size: 640x480" {
		t.Errorf("expected 'size: 640x480', got %q", string(out))
	}

	// Case 2: codec_private with hex and 0x prefix
	json0x := []byte(`{
		"streams": [
			{
				"codec_private": "0x73697a653a2036343078343830"
			}
		]
	}`)
	out, err = parseCodecPrivateOutput(json0x)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(out) != "size: 640x480" {
		t.Errorf("expected 'size: 640x480', got %q", string(out))
	}

	// Case 3: plain text
	jsonPlain := []byte(`{
		"streams": [
			{
				"codec_private": "size: 640x480\npalette: 000000"
			}
		]
	}`)
	out, err = parseCodecPrivateOutput(jsonPlain)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(out) != "size: 640x480\npalette: 000000" {
		t.Errorf("expected plain text, got %q", string(out))
	}

	// Case 4: empty streams
	jsonEmpty := []byte(`{"streams": []}`)
	out, err = parseCodecPrivateOutput(jsonEmpty)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != nil {
		t.Errorf("expected nil for empty streams, got %v", out)
	}

	// Case 5: empty field in stream
	jsonEmptyField := []byte(`{"streams": [{}]}`)
	out, err = parseCodecPrivateOutput(jsonEmptyField)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != nil {
		t.Errorf("expected nil for empty field, got %v", out)
	}
}


