package vobsub

import (
	"bytes"
	"encoding/binary"
	"io"
	"time"
)

// RawSPU represents a single raw SPU packet extracted from PS/PES stream.
type RawSPU struct {
	Data        []byte
	PTS         time.Duration
	SubStreamID byte
}

// ParsePSStream demuxes MPEG-2 PS / PES to extract SPUs for a specific subStreamID.
func ParsePSStream(r io.Reader, targetSubStreamID byte) ([]RawSPU, error) {
	var spus []RawSPU
	var currentSPU []byte
	var currentPTS time.Duration
	var currentSubStream byte
	expectedSize := 0

	buf := make([]byte, 128*1024)
	var data []byte

	for {
		n, err := r.Read(buf)
		if n > 0 {
			data = append(data, buf[:n]...)
		}
		if n == 0 && err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		pos := 0
		for pos < len(data)-4 {
			if data[pos] == 0x00 && data[pos+1] == 0x00 && data[pos+2] == 0x01 {
				streamID := data[pos+3]
				startPos := pos

				if streamID == 0xBA {
					pos += 4
					if pos >= len(data) {
						pos = startPos
						break
					}
					if (data[pos] & 0xC0) == 0x40 { // MPEG-2
						if pos+9 >= len(data) {
							pos = startPos
							break
						}
						stuffing := int(data[pos+9] & 0x07)
						if pos+10+stuffing > len(data) {
							pos = startPos
							break
						}
						pos += 10 + stuffing
					} else {
						if pos+8 > len(data) {
							pos = startPos
							break
						}
						pos += 8
					}
					continue
				}

				if pos+6 > len(data) {
					break
				}
				pesLen := int(binary.BigEndian.Uint16(data[pos+4 : pos+6]))
				pos += 6

				var payload []byte
				if pesLen > 0 {
					if pos+pesLen > len(data) {
						// Need more data
						pos = startPos
						break
					}
					payload = data[pos : pos+pesLen]
					pos += pesLen
				} else {
					nextPos := bytes.Index(data[pos:], []byte{0x00, 0x00, 0x01})
					if nextPos == -1 {
						if err == io.EOF {
							payload = data[pos:]
							pos = len(data)
						} else {
							pos = startPos
							break
						}
					} else {
						payload = data[pos : pos+nextPos]
						pos += nextPos
					}
				}

				if streamID == 0xBD { // Private Stream 1
					if len(payload) < 3 {
						continue
					}
					// flag1 := payload[0]
					flag2 := payload[1]
					headerLen := int(payload[2])

					if 3+headerLen > len(payload) {
						continue
					}

					var pts time.Duration
					if (flag2 & 0x80) != 0 {
						if headerLen >= 5 {
							ptsBytes := payload[3:8]
							pts1 := (uint64(ptsBytes[0]) & 0x0E) >> 1
							pts2 := (uint64(ptsBytes[1]) << 7) | ((uint64(ptsBytes[2]) & 0xFE) >> 1)
							pts3 := (uint64(ptsBytes[3]) << 7) | ((uint64(ptsBytes[4]) & 0xFE) >> 1)
							ptsVal := (pts1 << 30) | (pts2 << 15) | pts3
							pts = time.Duration(ptsVal) * time.Second / 90000
						}
					}

					payloadData := payload[3+headerLen:]
					if len(payloadData) == 0 {
						continue
					}

					subStreamID := payloadData[0]
					spuPayload := payloadData[1:]

					if subStreamID == targetSubStreamID {
						if len(currentSPU) == 0 {
							if len(spuPayload) < 2 {
								continue
							}
							expectedSize = int(binary.BigEndian.Uint16(spuPayload[0:2]))
							if expectedSize < 2 {
								// Invalid SPU size, ignore this payload
								continue
							}
							currentPTS = pts
							currentSubStream = subStreamID
						}

						currentSPU = append(currentSPU, spuPayload...)

						if len(currentSPU) >= expectedSize && expectedSize > 0 {
							spus = append(spus, RawSPU{
								Data:        currentSPU[:expectedSize],
								PTS:         currentPTS,
								SubStreamID: currentSubStream,
							})
							// Reset for next
							currentSPU = nil
							expectedSize = 0
						}
					}
				}
			} else {
				// Find next start code
				nextPos := bytes.Index(data[pos:], []byte{0x00, 0x00, 0x01})
				if nextPos == -1 {
					pos = len(data) - 2
					if pos < 0 {
						pos = 0
					}
					break
				}
				pos += nextPos
			}
		}

		if pos > 0 && pos <= len(data) {
			copy(data, data[pos:])
			data = data[:len(data)-pos]
		}
	}

	return spus, nil
}
