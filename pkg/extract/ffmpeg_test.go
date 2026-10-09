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

