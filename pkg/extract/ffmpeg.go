package extract

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
)

// Track represents a PGS subtitle track found in an MKV file.
type Track struct {
	Index    int    // Absolute stream index in the MKV
	RelIndex int    // Relative index among subtitle streams
	Language string // Language tag (e.g., "eng", "jpn")
	Title    string // Track title if set
}

type ffprobeOutput struct {
	Streams []struct {
		Index     int `json:"index"`
		CodecName string `json:"codec_name"`
		Tags      struct {
			Language string `json:"language"`
			Title    string `json:"title"`
		} `json:"tags"`
	} `json:"streams"`
}

// buildFfprobeCmd returns the exec.Cmd to run ffprobe.
func buildFfprobeCmd(ctx context.Context, mkvPath string) *exec.Cmd {
	return exec.CommandContext(ctx, "ffprobe", "-v", "quiet", "-print_format", "json", "-show_streams", "-select_streams", "s", mkvPath)
}

// ListPGSTracks uses ffprobe to find all PGS (hdmv_pgs_subtitle) tracks in an MKV.
func ListPGSTracks(ctx context.Context, mkvPath string) ([]Track, error) {
	cmd := buildFfprobeCmd(ctx, mkvPath)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w", err)
	}

	return parseFfprobeOutput(out)
}

func parseFfprobeOutput(data []byte) ([]Track, error) {
	var probe ffprobeOutput
	if err := json.Unmarshal(data, &probe); err != nil {
		return nil, fmt.Errorf("failed to parse ffprobe JSON: %w", err)
	}

	var tracks []Track
	relIndex := 0
	for _, st := range probe.Streams {
		if st.CodecName == "hdmv_pgs_subtitle" {
			tracks = append(tracks, Track{
				Index:    st.Index,
				RelIndex: relIndex,
				Language: st.Tags.Language,
				Title:    st.Tags.Title,
			})
		}
		relIndex++
	}

	return tracks, nil
}

// buildFfmpegCmd returns the exec.Cmd to run ffmpeg.
func buildFfmpegCmd(ctx context.Context, mkvPath string, trackRelIndex int, outputPath string) *exec.Cmd {
	mapArg := fmt.Sprintf("0:s:%d", trackRelIndex)
	return exec.CommandContext(ctx, "ffmpeg", "-y", "-i", mkvPath, "-map", mapArg, "-c", "copy", outputPath)
}

// ExtractPGS extracts a PGS subtitle track from an MKV to a .sup file.
// trackRelIndex is the relative subtitle stream index (0-based).
func ExtractPGS(ctx context.Context, mkvPath string, trackRelIndex int, outputPath string) error {
	cmd := buildFfmpegCmd(ctx, mkvPath, trackRelIndex, outputPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg extraction failed: %w", err)
	}
	return nil
}

