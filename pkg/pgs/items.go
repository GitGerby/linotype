package pgs

import (
	"time"
	"git.dyson.somuchcrypto.com/gearnsc/linotype/pkg/subtitle"
)

// ExtractSubtitleItems analyzes a sequence of DisplaySets, pairs appearance and
// disappearance sets, and renders each subtitle into a subtitle.Item.
func ExtractSubtitleItems(sets []DisplaySet) ([]subtitle.Item, error) {
	var items []subtitle.Item

	type activeSub struct {
		set   DisplaySet
		start time.Duration
	}

	var current *activeSub

	for i := 0; i < len(sets); i++ {
		set := sets[i]
		hasObjects := len(set.PCS.Objects) > 0

		if hasObjects {
			// If there was an active subtitle already on screen, close it now
			if current != nil {
				endTime := set.PTS
				if endTime <= current.start {
					endTime = current.start + 2*time.Second
				}
				item, err := buildItem(len(items)+1, current.set, current.start, endTime)
				if err == nil {
					items = append(items, item)
				}
				current = nil
			}

			// Start new active subtitle
			current = &activeSub{
				set:   set,
				start: set.PTS,
			}
		} else {
			// Screen cleared
			if current != nil {
				endTime := set.PTS
				if endTime <= current.start {
					endTime = current.start + 2*time.Second
				}
				item, err := buildItem(len(items)+1, current.set, current.start, endTime)
				if err == nil {
					items = append(items, item)
				}
				current = nil
			}
		}
	}

	// If a subtitle was still active at the end of the stream
	if current != nil {
		endTime := current.start + 2*time.Second
		item, err := buildItem(len(items)+1, current.set, current.start, endTime)
		if err == nil {
			items = append(items, item)
		}
	}

	return items, nil
}

func buildItem(idx int, set DisplaySet, start, end time.Duration) (subtitle.Item, error) {
	pngData, dominantColor, x, y, w, h, err := RenderDisplaySet(&set)
	if err != nil {
		return subtitle.Item{}, err
	}

	vw := int(set.PCS.Width)
	vh := int(set.PCS.Height)
	if vw == 0 {
		vw = 1920
	}
	if vh == 0 {
		vh = 1080
	}

	return subtitle.Item{
		Index:       idx,
		StartTime:   start,
		EndTime:     end,
		PNGData:     pngData,
		X:           x,
		Y:           y,
		Width:       w,
		Height:      h,
		VideoWidth:  vw,
		VideoHeight: vh,
		Color:       dominantColor,
	}, nil
}

