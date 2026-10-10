package extract

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
)

// SubtitleCodec represents the subtitle format
type SubtitleCodec string

const (
	CodecPGS    SubtitleCodec = "hdmv_pgs_subtitle"
	CodecVobSub SubtitleCodec = "dvd_subtitle"
)

// Track represents a subtitle track found in a media file.
type Track struct {
	Index    int           // Absolute stream index in the media file
	RelIndex int           // Relative index among subtitle streams
	Language string        // Language tag (e.g., "eng", "jpn")
	Title    string        // Track title if set
	Codec    SubtitleCodec // Format of the subtitle track
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

type ffprobeVideoOutput struct {
	Streams []struct {
		Index       int    `json:"index"`
		CodecName   string `json:"codec_name"`
		CodecType   string `json:"codec_type"`
		Width       int    `json:"width"`
		Height      int    `json:"height"`
		Disposition struct {
			AttachedPic int `json:"attached_pic"`
		} `json:"disposition"`
	} `json:"streams"`
}

// buildFfprobeVideoCmd returns the exec.Cmd to query video streams using ffprobe.
func buildFfprobeVideoCmd(ctx context.Context, mkvPath string) *exec.Cmd {
	return exec.CommandContext(ctx, "ffprobe", "-v", "quiet", "-print_format", "json", "-show_streams", "-select_streams", "v", mkvPath)
}

// GetVideoDimensions uses ffprobe to find the video dimensions (width, height) of an MKV file.
func GetVideoDimensions(ctx context.Context, mkvPath string) (int, int, error) {
	cmd := buildFfprobeVideoCmd(ctx, mkvPath)
	out, err := cmd.Output()
	if err != nil {
		return 0, 0, fmt.Errorf("ffprobe video query failed: %w", err)
	}

	return parseFfprobeVideoOutput(out)
}

func parseFfprobeVideoOutput(data []byte) (int, int, error) {
	var probe ffprobeVideoOutput
	if err := json.Unmarshal(data, &probe); err != nil {
		return 0, 0, fmt.Errorf("failed to parse ffprobe video JSON: %w", err)
	}

	for _, st := range probe.Streams {
		if st.Width > 0 && st.Height > 0 && st.Disposition.AttachedPic == 0 {
			return st.Width, st.Height, nil
		}
	}

	for _, st := range probe.Streams {
		if st.Width > 0 && st.Height > 0 {
			return st.Width, st.Height, nil
		}
	}

	return 0, 0, fmt.Errorf("no video stream with valid dimensions found")
}

// buildFfprobeCmd returns the exec.Cmd to run ffprobe.
func buildFfprobeCmd(ctx context.Context, mkvPath string) *exec.Cmd {
	return exec.CommandContext(ctx, "ffprobe", "-v", "quiet", "-print_format", "json", "-show_streams", "-select_streams", "s", mkvPath)
}

// ListSubtitleTracks uses ffprobe to find all PGS (hdmv_pgs_subtitle) and VOBSUB (dvd_subtitle) tracks.
func ListSubtitleTracks(ctx context.Context, mediaPath string) ([]Track, error) {
	cmd := buildFfprobeCmd(ctx, mediaPath)
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
		if st.CodecName == "hdmv_pgs_subtitle" || st.CodecName == "dvd_subtitle" {
			tracks = append(tracks, Track{
				Index:    st.Index,
				RelIndex: relIndex,
				Language: st.Tags.Language,
				Title:    st.Tags.Title,
				Codec:    SubtitleCodec(st.CodecName),
			})
		}
		if st.CodecName != "" && (st.CodecName == "hdmv_pgs_subtitle" || st.CodecName == "dvd_subtitle" || st.CodecName == "subrip" || st.CodecName == "ass" || st.CodecName == "mov_text" || st.CodecName == "webvtt") {
			// Actually we just increment relIndex for ANY subtitle stream to match ffmpeg's 0:s:relIndex.
			// Wait, ffmpeg's 0:s:X applies to all subtitle streams.
			// The original code unconditionally incremented relIndex for all streams in probe.Streams.
			// Since we only requested `-select_streams s`, all streams are subtitle streams.
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


// buildExtractDVDCmd returns the exec.Cmd to run ffmpeg for DVD subtitle extraction.
func buildExtractDVDCmd(ctx context.Context, mediaPath string, trackRelIndex int, outputPath string) *exec.Cmd {
	mapArg := fmt.Sprintf("0:s:%d", trackRelIndex)
	return exec.CommandContext(ctx, "ffmpeg", "-y", "-i", mediaPath, "-map", mapArg, "-c", "copy", "-f", "vob", outputPath)
}

// ExtractDVDSubtitle extracts a DVD subtitle track from an MKV or VOB to a .sub (MPEG-PS) file.
func ExtractDVDSubtitle(ctx context.Context, mediaPath string, trackRelIndex int, outputPath string) error {
	cmd := buildExtractDVDCmd(ctx, mediaPath, trackRelIndex, outputPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg DVD extraction failed: %w", err)
	}
	return nil
}

// GetTrackCodecPrivate retrieves the CodecPrivate data of a specific track from an MKV file.
func GetTrackCodecPrivate(ctx context.Context, mediaPath string, trackRelIndex int) ([]byte, error) {
	// ffprobe -v quiet -show_entries stream=codec_private -select_streams s:trackRelIndex -print_format json
	mapArg := fmt.Sprintf("s:%d", trackRelIndex)
	cmd := exec.CommandContext(ctx, "ffprobe", "-v", "quiet", "-show_entries", "stream=codec_private", "-select_streams", mapArg, "-print_format", "json", mediaPath)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe codec_private failed: %w", err)
	}

	var probe struct {
		Streams []struct {
			CodecPrivate string `json:"codec_private"`
		} `json:"streams"`
	}

	if err := json.Unmarshal(out, &probe); err != nil {
		return nil, fmt.Errorf("failed to parse ffprobe codec_private JSON: %w", err)
	}

	if len(probe.Streams) == 0 {
		return nil, nil // Not found or no streams matched
	}

	return []byte(probe.Streams[0].CodecPrivate), nil
}
