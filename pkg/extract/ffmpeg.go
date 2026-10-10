package extract

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// SubtitleCodec represents the subtitle format
type SubtitleCodec string

const (
	CodecPGS    SubtitleCodec = "hdmv_pgs_subtitle"
	CodecVobSub SubtitleCodec = "dvd_subtitle"
)

// Track represents a subtitle track found in a media file.
type Track struct {
	Index     int           // Absolute stream index in the media file
	RelIndex  int           // Relative index among subtitle streams
	Language  string        // Language tag (e.g., "eng", "jpn")
	Title     string        // Track title if set
	Codec     SubtitleCodec // Format of the subtitle track
	Width     int           // Subtitle track width if available
	Height    int           // Subtitle track height if available
	Extradata []byte        // Subtitle track extradata/codec_private if available
}

type ffprobeOutput struct {
	Streams []struct {
		Index        int    `json:"index"`
		CodecName    string `json:"codec_name"`
		Width        int    `json:"width"`
		Height       int    `json:"height"`
		CodecPrivate string `json:"codec_private"`
		Extradata    string `json:"extradata"`
		Tags         struct {
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
		var extra []byte
		val := st.CodecPrivate
		if val == "" {
			val = st.Extradata
		}
		if val != "" {
			clean := strings.TrimPrefix(strings.TrimSpace(val), "0x")
			clean = strings.TrimPrefix(clean, "0X")
			if dec, err := hex.DecodeString(clean); err == nil && len(dec) > 0 {
				extra = dec
			} else {
				extra = []byte(val)
			}
		}

		if st.CodecName == "hdmv_pgs_subtitle" || st.CodecName == "dvd_subtitle" {
			tracks = append(tracks, Track{
				Index:     st.Index,
				RelIndex:  relIndex,
				Language:  st.Tags.Language,
				Title:     st.Tags.Title,
				Codec:     SubtitleCodec(st.CodecName),
				Width:     st.Width,
				Height:    st.Height,
				Extradata: extra,
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

// buildCodecPrivateCmd returns the exec.Cmd to query track codec_private / extradata using ffprobe.
func buildCodecPrivateCmd(ctx context.Context, mediaPath string, trackRelIndex int) *exec.Cmd {
	mapArg := fmt.Sprintf("s:%d", trackRelIndex)
	return exec.CommandContext(ctx, "ffprobe", "-v", "quiet", "-show_entries", "stream=codec_private,extradata", "-select_streams", mapArg, "-print_format", "json", mediaPath)
}

// parseCodecPrivateOutput parses ffprobe JSON output for stream codec_private or extradata.
func parseCodecPrivateOutput(data []byte) ([]byte, error) {
	var probe struct {
		Streams []struct {
			CodecPrivate string `json:"codec_private"`
			Extradata    string `json:"extradata"`
		} `json:"streams"`
	}

	if err := json.Unmarshal(data, &probe); err != nil {
		return nil, fmt.Errorf("failed to parse ffprobe codec_private JSON: %w", err)
	}

	if len(probe.Streams) == 0 {
		return nil, nil // Not found or no streams matched
	}

	val := probe.Streams[0].CodecPrivate
	if val == "" {
		val = probe.Streams[0].Extradata
	}
	if val == "" {
		return nil, nil
	}

	clean := strings.TrimPrefix(strings.TrimSpace(val), "0x")
	clean = strings.TrimPrefix(clean, "0X")
	if decoded, err := hex.DecodeString(clean); err == nil && len(decoded) > 0 {
		return decoded, nil
	}
	return []byte(val), nil
}

// GetTrackCodecPrivate retrieves the CodecPrivate / extradata of a specific track from a media file.
func GetTrackCodecPrivate(ctx context.Context, mediaPath string, trackRelIndex int) ([]byte, error) {
	cmd := buildCodecPrivateCmd(ctx, mediaPath, trackRelIndex)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe codec_private failed: %w", err)
	}
	return parseCodecPrivateOutput(out)
}
