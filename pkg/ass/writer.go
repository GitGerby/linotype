package ass

import (
	"fmt"
	"io"
	"strings"
	"time"

	"git.dyson.somuchcrypto.com/gearnsc/linotype/pkg/subtitle"
)

// Write outputs a complete ASS subtitle file to w.
func Write(w io.Writer, events []subtitle.Event) error {
	if len(events) == 0 {
		return nil
	}

	videoWidth := events[0].VideoWidth
	videoHeight := events[0].VideoHeight
	if videoWidth == 0 {
		videoWidth = 1920
	}
	if videoHeight == 0 {
		videoHeight = 1080
	}

	header := fmt.Sprintf(`[Script Info]
Title: Linotype Generated Subtitles
ScriptType: v4.00+
PlayResX: %d
PlayResY: %d
WrapStyle: 0
ScaledBorderAndShadow: yes

[V4+ Styles]
Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding
Style: Default,Arial,48,&H00FFFFFF,&H000000FF,&H00000000,&H80000000,0,0,0,0,100,100,0,0,1,2,1,2,10,10,10,1

[Events]
Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text
`, videoWidth, videoHeight)

	if _, err := io.WriteString(w, header); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	for _, ev := range events {
		start := formatASSTime(ev.StartTime)
		end := formatASSTime(ev.EndTime)

		align := calculateAlignment(ev.X, ev.Y, ev.VideoWidth, ev.VideoHeight)
		colorTag := ""
		if ev.Color.A != 0 && (ev.Color.R != 255 || ev.Color.G != 255 || ev.Color.B != 255) {
			colorTag = fmt.Sprintf("\\c&H%02X%02X%02X&", ev.Color.B, ev.Color.G, ev.Color.R)
		}

		text := strings.ReplaceAll(ev.Text, "\n", "\\N")
		
		tags := fmt.Sprintf("{\\an%d\\pos(%d,%d)%s}", align, ev.X, ev.Y, colorTag)
		dialogue := fmt.Sprintf("Dialogue: 0,%s,%s,Default,,0,0,0,,%s%s\n", start, end, tags, text)

		if _, err := io.WriteString(w, dialogue); err != nil {
			return fmt.Errorf("failed to write event: %w", err)
		}
	}

	return nil
}

func formatASSTime(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	cs := int(d.Milliseconds()%1000) / 10
	return fmt.Sprintf("%d:%02d:%02d.%02d", h, m, s, cs)
}

func calculateAlignment(x, y, vw, vh int) int {
	if vw == 0 {
		vw = 1920
	}
	if vh == 0 {
		vh = 1080
	}

	col := 0
	if x >= (2 * vw / 3) {
		col = 3
	} else if x >= vw/3 {
		col = 2
	} else {
		col = 1
	}

	row := 0
	if y >= (2 * vh / 3) {
		row = 1 // Bottom
	} else if y >= vh/3 {
		row = 4 // Mid
	} else {
		row = 7 // Top
	}

	// 1 2 3 (bottom left, center, right)
	// 4 5 6 (mid left, center, right)
	// 7 8 9 (top left, center, right)

	var align int
	switch row {
	case 1: // bottom
		switch col {
		case 1: align = 1
		case 2: align = 2
		case 3: align = 3
		}
	case 4: // mid
		switch col {
		case 1: align = 4
		case 2: align = 5
		case 3: align = 6
		}
	case 7: // top
		switch col {
		case 1: align = 7
		case 2: align = 8
		case 3: align = 9
		}
	}
	return align
}

