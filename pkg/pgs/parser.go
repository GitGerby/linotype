package pgs

import (
	"encoding/binary"
	"fmt"
	"io"
	"time"
)

// Parse reads a PGS/SUP stream and returns all display sets.
func Parse(r io.Reader) ([]DisplaySet, error) {
	var sets []DisplaySet
	var currentSet DisplaySet
	var buildingSet bool

	// Map of ObjectID -> ObjectData to track potentially fragmented ODS in current set
	objects := make(map[uint16]*ObjectData)
	// Epoch buffers to retain palettes and objects across display sets in an epoch
	epochPalettes := make(map[byte]*Palette)
	epochObjects := make(map[uint16]*ObjectData)

	for {
		var header [13]byte
		_, err := io.ReadFull(r, header[:])
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read segment header: %w", err)
		}

		if header[0] != 0x50 || header[1] != 0x47 {
			return nil, fmt.Errorf("invalid magic number: %02x %02x", header[0], header[1])
		}

		ptsRaw := binary.BigEndian.Uint32(header[2:6])
		dtsRaw := binary.BigEndian.Uint32(header[6:10])
		segType := header[10]
		size := binary.BigEndian.Uint16(header[11:13])

		data := make([]byte, size)
		if size > 0 {
			if _, err := io.ReadFull(r, data); err != nil {
				return nil, fmt.Errorf("failed to read segment data: %w", err)
			}
		}

		// A display set starts with a PCS
		if segType == SegmentTypePCS {
			if buildingSet {
				sets = append(sets, currentSet)
			}
			buildingSet = true
			currentSet = DisplaySet{
				PTS: time.Duration(ptsRaw) * time.Second / 90000,
				DTS: time.Duration(dtsRaw) * time.Second / 90000,
			}
			objects = make(map[uint16]*ObjectData)
		}

		switch segType {
		case SegmentTypePCS:
			if len(data) < 11 {
				continue
			}
			pcs := PCS{
				Width:             binary.BigEndian.Uint16(data[0:2]),
				Height:            binary.BigEndian.Uint16(data[2:4]),
				FrameRate:         data[4],
				CompositionNumber: binary.BigEndian.Uint16(data[5:7]),
				State:             CompositionState(data[7]),
				PaletteUpdateFlag: data[8] != 0,
				PaletteID:         data[9],
			}

			if pcs.State == CompositionEpochStart {
				epochPalettes = make(map[byte]*Palette)
				epochObjects = make(map[uint16]*ObjectData)
			}

			numObj := int(data[10])
			offset := 11
			for i := 0; i < numObj; i++ {
				if offset+8 > len(data) {
					break
				}
				flags := data[offset+3]
				obj := CompositionObject{
					ObjectID:     binary.BigEndian.Uint16(data[offset : offset+2]),
					WindowID:     data[offset+2],
					CroppedFlag:  (flags & 0x80) != 0,
					ForcedOnFlag: (flags & 0x40) != 0,
					X:            binary.BigEndian.Uint16(data[offset+4 : offset+6]),
					Y:            binary.BigEndian.Uint16(data[offset+6 : offset+8]),
				}
				offset += 8

				if obj.CroppedFlag {
					if offset+8 > len(data) {
						break
					}
					obj.CropX = binary.BigEndian.Uint16(data[offset : offset+2])
					obj.CropY = binary.BigEndian.Uint16(data[offset+2 : offset+4])
					obj.CropWidth = binary.BigEndian.Uint16(data[offset+4 : offset+6])
					obj.CropHeight = binary.BigEndian.Uint16(data[offset+6 : offset+8])
					offset += 8
				}
				pcs.Objects = append(pcs.Objects, obj)
			}
			currentSet.PCS = pcs

		case SegmentTypeWDS:
			if len(data) < 1 {
				continue
			}
			numWindows := int(data[0])
			offset := 1
			for i := 0; i < numWindows; i++ {
				if offset+9 > len(data) {
					break
				}
				w := Window{
					ID:     data[offset],
					X:      binary.BigEndian.Uint16(data[offset+1 : offset+3]),
					Y:      binary.BigEndian.Uint16(data[offset+3 : offset+5]),
					Width:  binary.BigEndian.Uint16(data[offset+5 : offset+7]),
					Height: binary.BigEndian.Uint16(data[offset+7 : offset+9]),
				}
				currentSet.Windows = append(currentSet.Windows, w)
				offset += 9
			}

		case SegmentTypePDS:
			if len(data) < 2 {
				continue
			}
			currentSet.Palette.ID = data[0]
			currentSet.Palette.Version = data[1]
			offset := 2
			for offset+5 <= len(data) {
				entryIdx := data[offset]
				currentSet.Palette.Entries[entryIdx] = PaletteEntry{
					Y:     data[offset+1],
					Cb:    data[offset+2],
					Cr:    data[offset+3],
					Alpha: data[offset+4],
				}
				offset += 5
			}
			palCopy := currentSet.Palette
			epochPalettes[currentSet.Palette.ID] = &palCopy

		case SegmentTypeODS:
			if len(data) < 4 {
				continue
			}
			objID := binary.BigEndian.Uint16(data[0:2])
			version := data[2]
			seqFlag := data[3]
			isFirst := (seqFlag & 0x80) != 0

			if isFirst {
				if len(data) < 11 {
					continue
				}
				width := binary.BigEndian.Uint16(data[7:9])
				height := binary.BigEndian.Uint16(data[9:11])

				obj := &ObjectData{
					ID:           objID,
					Version:      version,
					SequenceFlag: seqFlag,
					Width:        width,
					Height:       height,
					Data:         append([]byte(nil), data[11:]...),
				}
				objects[objID] = obj
				epochObjects[objID] = obj
			} else {
				if obj, ok := objects[objID]; ok {
					obj.Data = append(obj.Data, data[4:]...)
					if (seqFlag & 0x40) != 0 {
						obj.SequenceFlag |= 0x40
					}
				} else if obj, ok := epochObjects[objID]; ok {
					obj.Data = append(obj.Data, data[4:]...)
					if (seqFlag & 0x40) != 0 {
						obj.SequenceFlag |= 0x40
					}
					objects[objID] = obj
				}
			}

		case SegmentTypeEND:
			if buildingSet {
				if pal, ok := epochPalettes[currentSet.PCS.PaletteID]; ok {
					hasPalette := false
					for _, e := range currentSet.Palette.Entries {
						if e.Alpha > 0 {
							hasPalette = true
							break
						}
					}
					if !hasPalette {
						currentSet.Palette = *pal
					}
				}

				for _, comp := range currentSet.PCS.Objects {
					if obj, ok := objects[comp.ObjectID]; ok {
						currentSet.Objects = append(currentSet.Objects, *obj)
					} else if obj, ok := epochObjects[comp.ObjectID]; ok {
						currentSet.Objects = append(currentSet.Objects, *obj)
					}
				}
				if len(currentSet.PCS.Objects) == 0 {
					for _, obj := range objects {
						currentSet.Objects = append(currentSet.Objects, *obj)
					}
				}

				sets = append(sets, currentSet)
				buildingSet = false
			}
		}
	}

	return sets, nil
}

