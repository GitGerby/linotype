package vobsub

import (
	"bytes"
	"fmt"
	"io"
	"time"

	"git.dyson.somuchcrypto.com/gearnsc/linotype/pkg/subtitle"
)

// ParseStream coordinates parsing to produce []subtitle.Item.
func ParseStream(subReader io.Reader, palette *Palette, videoW, videoH int) ([]subtitle.Item, error) {
	// Usually extracted tracks are mapped to 0x20
	rawSPUs, err := ParsePSStream(subReader, 0x20)
	if err != nil {
		return nil, err
	}

	var items []subtitle.Item
	for i, raw := range rawSPUs {
		spu, err := ParseSPU(raw.Data)
		if err != nil {
			// Skip invalid SPUs
			continue
		}

		img, domColor, err := Render(spu.Pixels, spu.Width, spu.Height, spu.Colors, spu.Alphas, palette)
		if err != nil {
			continue
		}

		pngData, err := EncodePNG(img)
		if err != nil {
			continue
		}

		start := raw.PTS + spu.StartDelay
		end := raw.PTS + spu.EndDelay
		if end <= start {
			// fallback if end time is missing or invalid
			if i+1 < len(rawSPUs) {
				end = rawSPUs[i+1].PTS
			} else {
				end = start + 5*time.Second
			}
		}

		items = append(items, subtitle.Item{
			Index:       i + 1,
			StartTime:   start,
			EndTime:     end,
			X:           spu.X,
			Y:           spu.Y,
			Width:       spu.Width,
			Height:      spu.Height,
			VideoWidth:  videoW,
			VideoHeight: videoH,
			Color:       domColor,
			PNGData:     pngData,
		})
	}

	return items, nil
}

// ParseIndexed coordinates parsing with idx file.
func ParseIndexed(idxReader io.Reader, subReader io.ReadSeeker, trackIndex int) ([]subtitle.Item, error) {
	idxInfo, err := ParseIdx(idxReader)
	if err != nil {
		return nil, err
	}
	return ParseIndexedWithInfo(idxInfo, subReader, trackIndex)
}

// ParseIndexedWithInfo parses subtitle items using pre-parsed IdxInfo.
func ParseIndexedWithInfo(idxInfo *IdxInfo, subReader io.ReadSeeker, trackIndex int) ([]subtitle.Item, error) {
	var track *TrackInfo
	for i := range idxInfo.Tracks {
		if idxInfo.Tracks[i].Index == trackIndex {
			track = &idxInfo.Tracks[i]
			break
		}
	}

	if track == nil {
		return nil, fmt.Errorf("track %d not found in idx", trackIndex)
	}

	var items []subtitle.Item
	// Substream ID in idx/sub is usually 0x20 + track.Index, but the cue points directly to the packet start.
	// We can just read from the file position.
	
	for i, cue := range track.Cues {
		_, err := subReader.Seek(cue.FilePos, io.SeekStart)
		if err != nil {
			continue
		}

		// Read one packet. Since it's PS/PES, we can just use ParsePSStream but we only want one SPU.
		// However ParsePSStream reads to EOF. Let's make a limited reader, or just read a chunk?
		// Actually, if we just parse the PES packet directly...
		// But the prompt says: "For the requested track, seek to each cue's FilePos in subReader. Read and parse each SPU at that position."
		// Wait, a PES packet might be up to 64KB. Let's read 128KB and parse one SPU.
		buf := make([]byte, 128*1024)
		n, err := io.ReadFull(subReader, buf)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			continue
		}
		if n == 0 {
			continue
		}

		rawSPUs, _ := ParsePSStream(bytes.NewReader(buf[:n]), byte(0x20+trackIndex))
		if len(rawSPUs) == 0 {
			continue
		}

		raw := rawSPUs[0]
		spu, err := ParseSPU(raw.Data)
		if err != nil {
			continue
		}

		img, domColor, err := Render(spu.Pixels, spu.Width, spu.Height, spu.Colors, spu.Alphas, idxInfo.Palette)
		if err != nil {
			continue
		}

		pngData, err := EncodePNG(img)
		if err != nil {
			continue
		}

		start, err := parseTimestamp(cue.Timestamp)
		if err != nil {
			continue
		}
		var end time.Duration
		if spu.EndDelay > 0 {
			end = start + spu.EndDelay
		} else if i+1 < len(track.Cues) {
			end, err = parseTimestamp(track.Cues[i+1].Timestamp)
			if err != nil {
				end = start + 5 * time.Second
			}
		} else {
			end = start + 5 * time.Second
		}

		items = append(items, subtitle.Item{
			Index:       i + 1,
			StartTime:   start,
			EndTime:     end,
			X:           spu.X,
			Y:           spu.Y,
			Width:       spu.Width,
			Height:      spu.Height,
			Color:       domColor,
			PNGData:     pngData,
		})
	}

	return items, nil
}

func parseTimestamp(ts string) (time.Duration, error) {
	// timestamp: 00:01:23:456
	var h, m, s, ms int
	_, err := fmt.Sscanf(ts, "%02d:%02d:%02d:%03d", &h, &m, &s, &ms)
	if err != nil {
		return 0, err
	}
	return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute + time.Duration(s)*time.Second + time.Duration(ms)*time.Millisecond, nil
}
