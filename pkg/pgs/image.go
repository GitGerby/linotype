package pgs

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
)

// DecodeRLE decodes PGS RLE bitmap data into a paletted pixel buffer.
// Returns a []byte of palette indices, one per pixel, in row-major order.
func DecodeRLE(data []byte, width, height int) ([]byte, error) {
	out := make([]byte, 0, width*height)
	i := 0
	for i < len(data) {
		b := data[i]
		i++

		if b != 0 {
			out = append(out, b)
		} else {
			if i >= len(data) {
				break
			}
			c := data[i]
			i++

			if c == 0 {
				// End of line. Pad remaining pixels on this line with 0.
				rem := len(out) % width
				if rem != 0 {
					pad := width - rem
					for j := 0; j < pad; j++ {
						out = append(out, 0)
					}
				}
			} else if c&0xC0 == 0x00 { // [0x01-0x3F]
				count := int(c & 0x3F)
				for j := 0; j < count; j++ {
					out = append(out, 0)
				}
			} else if c&0xC0 == 0x40 { // [0x40-0x7F]
				if i >= len(data) {
					break
				}
				d := data[i]
				i++
				count := (int(c&0x3F) << 8) | int(d)
				for j := 0; j < count; j++ {
					out = append(out, 0)
				}
			} else if c&0xC0 == 0x80 { // [0x80-0xBF]
				if i >= len(data) {
					break
				}
				e := data[i]
				i++
				count := int(c & 0x3F)
				for j := 0; j < count; j++ {
					out = append(out, e)
				}
			} else if c&0xC0 == 0xC0 { // [0xC0-0xFF]
				if i+1 >= len(data) {
					break
				}
				d := data[i]
				e := data[i+1]
				i += 2
				count := (int(c&0x3F) << 8) | int(d)
				for j := 0; j < count; j++ {
					out = append(out, e)
				}
			}
		}
	}

	targetSize := width * height
	if len(out) < targetSize {
		pad := targetSize - len(out)
		for j := 0; j < pad; j++ {
			out = append(out, 0)
		}
	}

	if len(out) > targetSize {
		out = out[:targetSize]
	}

	return out, nil
}

// RenderPNG renders an ODS object with the given palette to PNG bytes.
// Returns the PNG data and a map of palette index usage counts.
func RenderPNG(obj *ObjectData, palette *Palette) ([]byte, [256]int, error) {
	var counts [256]int
	if obj.Width == 0 || obj.Height == 0 {
		return nil, counts, fmt.Errorf("invalid object dimensions")
	}

	pixels, err := DecodeRLE(obj.Data, int(obj.Width), int(obj.Height))
	if err != nil {
		return nil, counts, fmt.Errorf("failed to decode RLE: %w", err)
	}

	img := image.NewRGBA(image.Rect(0, 0, int(obj.Width), int(obj.Height)))
	idx := 0
	for y := 0; y < int(obj.Height); y++ {
		for x := 0; x < int(obj.Width); x++ {
			if idx < len(pixels) {
				pIndex := pixels[idx]
				counts[pIndex]++
				img.SetRGBA(x, y, palette.ToRGBA(pIndex))
				idx++
			}
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, counts, fmt.Errorf("failed to encode PNG: %w", err)
	}

	return buf.Bytes(), counts, nil
}

// RenderDisplaySet composites all PCS objects in a DisplaySet into a single PNG image.
// It returns the PNG bytes, dominant text color, bounding box (x, y, width, height),
// and any error encountered.
func RenderDisplaySet(set *DisplaySet) ([]byte, color.RGBA, int, int, int, int, error) {
	if len(set.PCS.Objects) == 0 {
		return nil, color.RGBA{}, 0, 0, 0, 0, fmt.Errorf("no objects in display set")
	}

	objMap := make(map[uint16]*ObjectData, len(set.Objects))
	for i := range set.Objects {
		objMap[set.Objects[i].ID] = &set.Objects[i]
	}

	minX := int(set.PCS.Objects[0].X)
	minY := int(set.PCS.Objects[0].Y)
	maxX := minX
	maxY := minY
	matchedAny := false

	for _, comp := range set.PCS.Objects {
		obj, ok := objMap[comp.ObjectID]
		if !ok || obj.Width == 0 || obj.Height == 0 {
			continue
		}
		matchedAny = true
		cX := int(comp.X)
		cY := int(comp.Y)
		cW := int(obj.Width)
		cH := int(obj.Height)
		if cX < minX {
			minX = cX
		}
		if cY < minY {
			minY = cY
		}
		if cX+cW > maxX {
			maxX = cX + cW
		}
		if cY+cH > maxY {
			maxY = cY + cH
		}
	}

	if !matchedAny || maxX <= minX || maxY <= minY {
		return nil, color.RGBA{}, 0, 0, 0, 0, fmt.Errorf("no valid objects found in display set")
	}

	width := maxX - minX
	height := maxY - minY
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	var totalCounts [256]int

	for _, comp := range set.PCS.Objects {
		obj, ok := objMap[comp.ObjectID]
		if !ok || obj.Width == 0 || obj.Height == 0 {
			continue
		}

		pixels, err := DecodeRLE(obj.Data, int(obj.Width), int(obj.Height))
		if err != nil {
			return nil, color.RGBA{}, 0, 0, 0, 0, fmt.Errorf("failed to decode object %d RLE: %w", obj.ID, err)
		}

		offsetX := int(comp.X) - minX
		offsetY := int(comp.Y) - minY

		idx := 0
		for y := 0; y < int(obj.Height); y++ {
			for x := 0; x < int(obj.Width); x++ {
				if idx < len(pixels) {
					pIndex := pixels[idx]
					idx++
					rgba := set.Palette.ToRGBA(pIndex)
					if rgba.A > 0 {
						totalCounts[pIndex]++
						targetX := offsetX + x
						targetY := offsetY + y
						if targetX >= 0 && targetX < width && targetY >= 0 && targetY < height {
							img.SetRGBA(targetX, targetY, rgba)
						}
					}
				}
			}
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, color.RGBA{}, 0, 0, 0, 0, fmt.Errorf("failed to encode composite PNG: %w", err)
	}

	dominantColor := DominantColor(&set.Palette, totalCounts)
	return buf.Bytes(), dominantColor, minX, minY, width, height, nil
}

