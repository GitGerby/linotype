package vobsub

import (
	"encoding/binary"
	"fmt"
	"time"
)

// SPU represents a Subpicture Unit (raw bytes).
type SPU struct {
	Data []byte
}

// SPUData represents a parsed Subpicture Unit.
type SPUData struct {
	Colors     [4]uint8
	Alphas     [4]uint8
	X, Y       int
	Width      int
	Height     int
	Pixels     []byte
	StartDelay time.Duration
	EndDelay   time.Duration
}

// ParseSPU parses a DVD subpicture unit.
func ParseSPU(data []byte) (*SPUData, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("SPU data too short")
	}

	size := int(binary.BigEndian.Uint16(data[0:2]))
	if len(data) < size {
		return nil, fmt.Errorf("SPU data incomplete: expected %d, got %d", size, len(data))
	}

	dcsqtOffset := int(binary.BigEndian.Uint16(data[2:4]))
	if dcsqtOffset >= size || dcsqtOffset < 4 {
		return nil, fmt.Errorf("invalid DCSQT offset: %d", dcsqtOffset)
	}

	spu := &SPUData{}
	var topOffset, bottomOffset int
	hasStartDelay := false

	offset := dcsqtOffset
	for offset < size {
		if offset+4 > size {
			break
		}
		delayUnits := binary.BigEndian.Uint16(data[offset : offset+2])
		nextOffset := int(binary.BigEndian.Uint16(data[offset+2 : offset+4]))

		delay := time.Duration(delayUnits) * 1024 * time.Second / 90000

		cmdOffset := offset + 4
		endOfDcs := false
		isStop := false
		isStart := false

		for cmdOffset < size && !endOfDcs {
			cmd := data[cmdOffset]
			cmdOffset++
			switch cmd {
			case 0x00: // FSTA_DSP
				isStart = true
			case 0x01: // STA_DSP
				isStart = true
			case 0x02: // STP_DSP
				isStop = true
			case 0x03: // SET_COLOR
				if cmdOffset+2 > size {
					return nil, fmt.Errorf("unexpected end of SPU in SET_COLOR")
				}
				c1 := data[cmdOffset]
				c2 := data[cmdOffset+1]
				spu.Colors[3] = c1 >> 4
				spu.Colors[2] = c1 & 0x0F
				spu.Colors[1] = c2 >> 4
				spu.Colors[0] = c2 & 0x0F
				cmdOffset += 2
			case 0x04: // SET_CONTR
				if cmdOffset+2 > size {
					return nil, fmt.Errorf("unexpected end of SPU in SET_CONTR")
				}
				c1 := data[cmdOffset]
				c2 := data[cmdOffset+1]
				spu.Alphas[3] = (c1 >> 4) * 17
				spu.Alphas[2] = (c1 & 0x0F) * 17
				spu.Alphas[1] = (c2 >> 4) * 17
				spu.Alphas[0] = (c2 & 0x0F) * 17
				cmdOffset += 2
			case 0x05: // SET_DAREA
				if cmdOffset+6 > size {
					return nil, fmt.Errorf("unexpected end of SPU in SET_DAREA")
				}
				b := data[cmdOffset : cmdOffset+6]
				colStart := (int(b[0]) << 4) | (int(b[1]) >> 4)
				colEnd := ((int(b[1]) & 0x0F) << 8) | int(b[2])
				rowStart := (int(b[3]) << 4) | (int(b[4]) >> 4)
				rowEnd := ((int(b[4]) & 0x0F) << 8) | int(b[5])
				spu.X = colStart
				spu.Y = rowStart
				spu.Width = colEnd - colStart + 1
				spu.Height = rowEnd - rowStart + 1
				cmdOffset += 6
			case 0x06: // SET_DSPXA
				if cmdOffset+4 > size {
					return nil, fmt.Errorf("unexpected end of SPU in SET_DSPXA")
				}
				topOffset = int(binary.BigEndian.Uint16(data[cmdOffset : cmdOffset+2]))
				bottomOffset = int(binary.BigEndian.Uint16(data[cmdOffset+2 : cmdOffset+4]))
				cmdOffset += 4
			case 0xFF: // CMD_END
				endOfDcs = true
			default:
				// Unknown command, skip 1 byte? Actually it's dangerous, we might break parsing.
				// But let's assume we don't encounter unknown.
				return nil, fmt.Errorf("unknown SPU command: 0x%X", cmd)
			}
		}

		if isStart && !hasStartDelay {
			spu.StartDelay = delay
			hasStartDelay = true
		}
		if isStop {
			spu.EndDelay = delay
		}

		if nextOffset <= offset || nextOffset >= size {
			break
		}
		offset = nextOffset
	}

	if spu.Width > 0 && spu.Height > 0 && topOffset > 0 && bottomOffset > 0 {
		spu.Pixels = decodeRLE(data, spu.Width, spu.Height, topOffset, bottomOffset)
	}

	return spu, nil
}

type bitReader struct {
	data []byte
	pos  int // byte position
	bits uint8 // current byte
	n    int // number of bits left in current byte
}

func (r *bitReader) readBits(count int) int {
	res := 0
	for count > 0 {
		if r.n == 0 {
			if r.pos < len(r.data) {
				r.bits = r.data[r.pos]
				r.pos++
			} else {
				r.bits = 0
			}
			r.n = 8
		}
		take := count
		if take > r.n {
			take = r.n
		}
		res = (res << take) | int((r.bits>>(r.n-take))&((1<<take)-1))
		r.n -= take
		count -= take
	}
	return res
}

func (r *bitReader) align() {
	r.n = 0
}

func decodeRLE(data []byte, width, height, topOffset, bottomOffset int) []byte {
	pixels := make([]byte, width*height)

	decodeField := func(offset, startRow int) {
		if offset >= len(data) {
			return
		}
		r := &bitReader{data: data, pos: offset}
		for y := startRow; y < height; y += 2 {
			x := 0
			for x < width {
				val := r.readBits(4)
				length := 0
				color := 0
				if val >= 4 {
					length = val >> 2
					color = val & 3
				} else {
					val = (val << 4) | r.readBits(4)
					if val >= 16 {
						length = val >> 2
						color = val & 3
					} else {
						val = (val << 4) | r.readBits(4)
						if val >= 64 {
							length = val >> 2
							color = val & 3
						} else {
							val = (val << 4) | r.readBits(4)
							length = val >> 2
							color = val & 3
						}
					}
				}
				
				if length == 0 {
					length = width - x
				}
				
				for i := 0; i < length && x < width; i++ {
					pixels[y*width+x] = byte(color)
					x++
				}
			}
			r.align() // align to byte boundary at end of scanline
		}
	}

	decodeField(topOffset, 0)
	decodeField(bottomOffset, 1)

	return pixels
}

// DecodeRLE decompresses a DVD subpicture run-length encoded buffer.
// Kept for backwards compatibility if needed, though replaced by internal decodeRLE.
func DecodeRLE(data []byte, width, height int) []byte {
	// Not fully implementable without top/bottom offset, return empty
	return make([]byte, width*height)
}
