package webvtt

import (
	"fmt"
	"io"
	"strings"
	"time"

	"git.dyson.somuchcrypto.com/gearnsc/linotype/pkg/subtitle"
)

// Write outputs a complete WebVTT subtitle file to w.
func Write(w io.Writer, events []subtitle.Event) error {
	if _, err := io.WriteString(w, "WEBVTT\n\n"); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	for i, ev := range events {
		start := formatVTTTime(ev.StartTime)
		end := formatVTTTime(ev.EndTime)

		vw := ev.VideoWidth
		vh := ev.VideoHeight
		if vw == 0 {
			vw = 1920
		}
		if vh == 0 {
			vh = 1080
		}

		linePct := (ev.Y * 100) / vh
		posX := ev.X + ev.Width/2
		posPct := (posX * 100) / vw

		align := "start"
		if posX >= (2 * vw / 3) {
			align = "end"
		} else if posX >= vw/3 {
			align = "center"
		}

		cueSettings := fmt.Sprintf("line:%d%% position:%d%% align:%s", linePct, posPct, align)
		
		text := ev.Text
		if subtitle.IsCustomTextColor(ev.Color) {
			text = fmt.Sprintf("<font color=\"#%02X%02X%02X\">%s</font>", ev.Color.R, ev.Color.G, ev.Color.B, text)
		}

		// WebVTT lines are separated by normal newlines, so we ensure standard \n
		text = strings.ReplaceAll(text, "\r\n", "\n")

		block := fmt.Sprintf("%d\n%s --> %s %s\n%s\n\n", i+1, start, end, cueSettings, text)
		if _, err := io.WriteString(w, block); err != nil {
			return fmt.Errorf("failed to write block: %w", err)
		}
	}

	return nil
}

func formatVTTTime(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	ms := int(d.Milliseconds() % 1000)
	return fmt.Sprintf("%02d:%02d:%02d.%03d", h, m, s, ms)
}

