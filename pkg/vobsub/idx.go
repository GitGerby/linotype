package vobsub

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"io"
	"strconv"
	"strings"
)

// IdxInfo represents parsed information from an .idx file.
type IdxInfo struct {
	Width   int
	Height  int
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
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	trimmed := strings.TrimSpace(string(data))
	trimmed = strings.TrimPrefix(trimmed, "0x")
	trimmed = strings.TrimPrefix(trimmed, "0X")
	if decoded, err := hex.DecodeString(trimmed); err == nil && len(decoded) > 0 {
		lowerDecoded := bytes.ToLower(decoded)
		if bytes.Contains(lowerDecoded, []byte("palette:")) || bytes.Contains(lowerDecoded, []byte("size:")) || bytes.Contains(lowerDecoded, []byte("id:")) {
			data = decoded
		}
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	info := &IdxInfo{
		Palette: DefaultPalette(),
	}

	var currentTrack *TrackInfo

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		lowerLine := strings.ToLower(line)
		if strings.HasPrefix(lowerLine, "size:") {
			dimStr := strings.TrimSpace(line[len("size:"):])
			parts := strings.Split(strings.ToLower(dimStr), "x")
			if len(parts) == 2 {
				w, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
				h, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
				if err1 == nil && err2 == nil && w > 0 && h > 0 {
					info.Width = w
					info.Height = h
				}
			}
		} else if strings.HasPrefix(lowerLine, "palette:") {
			p, err := ParsePalette(line)
			if err != nil {
				return nil, err
			}
			info.Palette = p
		} else if strings.HasPrefix(lowerLine, "id:") {
			// id: en, index: 0
			parts := strings.Split(line, ",")
			if len(parts) >= 2 {
				colonIdx := strings.Index(parts[0], ":")
				var lang string
				if colonIdx != -1 {
					lang = strings.TrimSpace(parts[0][colonIdx+1:])
				}
				idxSub := strings.Index(strings.ToLower(parts[1]), "index:")
				if idxSub != -1 {
					idxStr := strings.TrimSpace(parts[1][idxSub+len("index:"):])
					idx, err := strconv.Atoi(idxStr)
					if err == nil {
						info.Tracks = append(info.Tracks, TrackInfo{
							Index:    idx,
							Language: lang,
						})
						currentTrack = &info.Tracks[len(info.Tracks)-1]
					}
				}
			}
		} else if strings.HasPrefix(lowerLine, "timestamp:") {
			// timestamp: 00:01:23:456, filepos: 000001000
			if currentTrack == nil {
				continue
			}
			parts := strings.Split(line, ",")
			if len(parts) >= 2 {
				colonIdx := strings.Index(parts[0], ":")
				var ts string
				if colonIdx != -1 {
					ts = strings.TrimSpace(parts[0][colonIdx+1:])
				}
				fpSub := strings.Index(strings.ToLower(parts[1]), "filepos:")
				if fpSub != -1 {
					fpStr := strings.TrimSpace(parts[1][fpSub+len("filepos:"):])
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
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return info, nil
}

