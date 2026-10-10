package ass

import (
	"fmt"
	"io"
	"math"
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

	fontSize := int(math.Round(48.0 * float64(videoHeight) / 1080.0))
	outline := int(math.Round(2.0 * float64(videoHeight) / 1080.0))
	shadow := int(math.Round(1.0 * float64(videoHeight) / 1080.0))
	margin := int(math.Round(10.0 * float64(videoHeight) / 1080.0))
	if fontSize <= 0 {
		fontSize = 48
	}
	if outline <= 0 {
		outline = 2
	}
	if shadow <= 0 {
		shadow = 1
	}
	if margin <= 0 {
		margin = 10
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
Style: Default,Arial,%d,&H00FFFFFF,&H000000FF,&H00000000,&H80000000,0,0,0,0,100,100,0,0,1,%d,%d,2,%d,%d,%d,1

[Events]
Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text
`, videoWidth, videoHeight, fontSize, outline, shadow, margin, margin, margin)

	if _, err := io.WriteString(w, header); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	for _, ev := range events {
		start := formatASSTime(ev.StartTime)
		end := formatASSTime(ev.EndTime)

		x := ev.X
		y := ev.Y
		width := ev.Width
		height := ev.Height
		if ev.VideoWidth > 0 && ev.VideoHeight > 0 && (ev.VideoWidth != videoWidth || ev.VideoHeight != videoHeight) {
			x = int(math.Round(float64(x) * float64(videoWidth) / float64(ev.VideoWidth)))
			y = int(math.Round(float64(y) * float64(videoHeight) / float64(ev.VideoHeight)))
			width = int(math.Round(float64(width) * float64(videoWidth) / float64(ev.VideoWidth)))
			height = int(math.Round(float64(height) * float64(videoHeight) / float64(ev.VideoHeight)))
		}

		align := calculateAlignment(x, y, width, height, videoWidth, videoHeight)
		posX, posY := calculateAnchor(x, y, width, height, align)
		colorTag := ""
		if ev.Color.A != 0 && (ev.Color.R != 255 || ev.Color.G != 255 || ev.Color.B != 255) {
			colorTag = fmt.Sprintf("\\c&H%02X%02X%02X&", ev.Color.B, ev.Color.G, ev.Color.R)
		}

		text := strings.ReplaceAll(ev.Text, "\n", "\\N")
		
		tags := fmt.Sprintf("{\\an%d\\pos(%d,%d)%s}", align, posX, posY, colorTag)
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

func calculateAlignment(x, y, w, h, vw, vh int) int {
	if vw == 0 {
		vw = 1920
	}
	if vh == 0 {
		vh = 1080
	}

	centerX := x + w/2
	centerY := y + h/2

	col := 0
	if centerX >= (2 * vw / 3) {
		col = 3
	} else if centerX >= vw/3 {
		col = 2
	} else {
		col = 1
	}

	row := 0
	if centerY >= (2 * vh / 3) {
		row = 1 // Bottom
	} else if centerY >= vh/3 {
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

func calculateAnchor(x, y, w, h, align int) (int, int) {
	posX := x
	posY := y

	switch align {
	case 1: // Bottom-Left
		posX = x
		posY = y + h
	case 2: // Bottom-Center
		posX = x + w/2
		posY = y + h
	case 3: // Bottom-Right
		posX = x + w
		posY = y + h
	case 4: // Mid-Left
		posX = x
		posY = y + h/2
	case 5: // Mid-Center
		posX = x + w/2
		posY = y + h/2
	case 6: // Mid-Right
		posX = x + w
		posY = y + h/2
	case 7: // Top-Left
		posX = x
		posY = y
	case 8: // Top-Center
		posX = x + w/2
		posY = y
	case 9: // Top-Right
		posX = x + w
		posY = y
	}

	return posX, posY
}

