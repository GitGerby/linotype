package vobsub

import (
	"bufio"
	"io"
	"strconv"
	"strings"
)

// IdxInfo represents parsed information from an .idx file.
type IdxInfo struct {
	Palette *Palette
	Tracks  []TrackInfo
}

// TrackInfo represents a subtitle track found in an .idx file.
type TrackInfo struct {
	Index    int
	Language string
	Cues     []Cue
}

// Cue represents a timestamp and file position.
type Cue struct {
	Timestamp string
	FilePos   int64
}

// ParseIdx parses an .idx text file.
func ParseIdx(r io.Reader) (*IdxInfo, error) {
	scanner := bufio.NewScanner(r)
	info := &IdxInfo{
		Palette: DefaultPalette(),
	}

	var currentTrack *TrackInfo

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "palette:") {
			p, err := ParsePalette(line)
			if err != nil {
				return nil, err
			}
			info.Palette = p
		} else if strings.HasPrefix(line, "id:") {
			// id: en, index: 0
			parts := strings.Split(line, ",")
			if len(parts) >= 2 {
				lang := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(parts[0]), "id:"))
				idxStr := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(parts[1]), "index:"))
				idx, err := strconv.Atoi(idxStr)
				if err == nil {
					info.Tracks = append(info.Tracks, TrackInfo{
						Index:    idx,
						Language: lang,
					})
					currentTrack = &info.Tracks[len(info.Tracks)-1]
				}
			}
		} else if strings.HasPrefix(line, "timestamp:") {
			// timestamp: 00:01:23:456, filepos: 000001000
			if currentTrack == nil {
				continue
			}
			parts := strings.Split(line, ",")
			if len(parts) >= 2 {
				ts := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(parts[0]), "timestamp:"))
				fpStr := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(parts[1]), "filepos:"))
				
				// filepos is hex
				fp, err := strconv.ParseInt(fpStr, 16, 64)
				if err == nil {
					currentTrack.Cues = append(currentTrack.Cues, Cue{
						Timestamp: ts,
						FilePos:   fp,
					})
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return info, nil
}

