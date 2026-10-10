package ass

import (
	"bytes"
	"image/color"
	"testing"
	"time"

	"git.dyson.somuchcrypto.com/gearnsc/linotype/pkg/subtitle"
)

func TestWriteASS(t *testing.T) {
	events := []subtitle.Event{
		{
			Index:       1,
			StartTime:   time.Second + 500*time.Millisecond,
			EndTime:     2*time.Second + 750*time.Millisecond,
			Text:        "Hello\nWorld",
			X:           100,
			Y:           100,
			VideoWidth:  1920,
			VideoHeight: 1080,
			Color:       color.RGBA{255, 255, 255, 255},
		},
		{
			Index:       2,
			StartTime:   3600 * time.Second, // 1 hour
			EndTime:     3601 * time.Second,
			Text:        "Test 2",
			X:           960,
			Y:           900,
			VideoWidth:  1920,
			VideoHeight: 1080,
			Color:       color.RGBA{255, 0, 0, 255}, // Red
		},
	}

	var buf bytes.Buffer
	err := Write(&buf, events)
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}

	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("Dialogue: 0,0:00:01.50,0:00:02.75,Default,,0,0,0,,{\\an7\\pos(100,100)}Hello\\NWorld")) {
		t.Errorf("Event 1 not formatted correctly, got:\n%s", output)
	}
	if !bytes.Contains([]byte(output), []byte("Dialogue: 0,1:00:00.00,1:00:01.00,Default,,0,0,0,,{\\an2\\pos(960,900)\\c&H0000FF&}Test 2")) {
		t.Errorf("Event 2 not formatted correctly, got:\n%s", output)
	}
}

func TestWriteASS_4K(t *testing.T) {
	events := []subtitle.Event{
		{
			Index:       1,
			StartTime:   time.Second,
			EndTime:     2 * time.Second,
			Text:        "4K Subtitle",
			X:           400,
			Y:           1600,
			Width:       800,
			Height:      200,
			VideoWidth:  3840,
			VideoHeight: 2160,
			Color:       color.RGBA{255, 255, 255, 255},
		},
	}

	var buf bytes.Buffer
	err := Write(&buf, events)
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}

	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("PlayResX: 3840")) {
		t.Errorf("expected PlayResX: 3840, got:\n%s", output)
	}
	if !bytes.Contains([]byte(output), []byte("PlayResY: 2160")) {
		t.Errorf("expected PlayResY: 2160, got:\n%s", output)
	}
	if !bytes.Contains([]byte(output), []byte("Style: Default,Arial,96")) {
		t.Errorf("expected Style with Fontsize 96, got:\n%s", output)
	}
	if !bytes.Contains([]byte(output), []byte("{\\an1\\pos(400,1800)}4K Subtitle")) {
		t.Errorf("expected Dialogue with \\pos(400,1800), got:\n%s", output)
	}
}

func TestWriteASS_ScalingMismatchedEvent(t *testing.T) {
	// First event has 4k header, but individual event coordinates are 1080p
	events := []subtitle.Event{
		{
			Index:       1,
			StartTime:   time.Second,
			EndTime:     2 * time.Second,
			Text:        "Scaled Subtitle",
			X:           200,
			Y:           800,
			Width:       400,
			Height:      100,
			VideoWidth:  3840,
			VideoHeight: 2160,
			Color:       color.RGBA{255, 255, 255, 255},
		},
		{
			Index:       2,
			StartTime:   3 * time.Second,
			EndTime:     4 * time.Second,
			Text:        "1080p Event in 4K Script",
			X:           200,
			Y:           800,
			Width:       400,
			Height:      100,
			VideoWidth:  1920,
			VideoHeight: 1080,
			Color:       color.RGBA{255, 255, 255, 255},
		},
	}

	var buf bytes.Buffer
	err := Write(&buf, events)
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}

	output := buf.String()
	// Event 2 with 1080p coords (200, 800, w:400, h:100) should be scaled to (400, 1600, w:800, h:200), anchor for \an1 is (400, 1800)
	if !bytes.Contains([]byte(output), []byte("{\\an1\\pos(400,1800)}1080p Event in 4K Script")) {
		t.Errorf("expected Event 2 to be scaled to \\pos(400,1800), got:\n%s", output)
	}
}

func TestWriteASS_BoundingBoxAlignment(t *testing.T) {
	events := []subtitle.Event{
		{
			Index:       1,
			StartTime:   time.Second,
			EndTime:     2 * time.Second,
			Text:        "Centered Subtitle",
			X:           560,
			Y:           950,
			Width:       800,
			Height:      50,
			VideoWidth:  1920,
			VideoHeight: 1080,
			Color:       color.RGBA{255, 255, 255, 255},
		},
		{
			Index:       2,
			StartTime:   3 * time.Second,
			EndTime:     4 * time.Second,
			Text:        "Top Right Subtitle",
			X:           1300,
			Y:           100,
			Width:       200,
			Height:      50,
			VideoWidth:  1920,
			VideoHeight: 1080,
			Color:       color.RGBA{255, 255, 255, 255},
		},
	}

	var buf bytes.Buffer
	err := Write(&buf, events)
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}

	output := buf.String()
	// Event 1 has center X=960, bottom Y=1000 -> Bottom-Center (\an2) at anchor (960, 1000)
	if !bytes.Contains([]byte(output), []byte("{\\an2\\pos(960,1000)}Centered Subtitle")) {
		t.Errorf("expected Event 1 to be \\an2\\pos(960,1000), got:\n%s", output)
	}
	// Event 2 has right X=1500, top Y=100 -> Top-Right (\an9) at anchor (1500, 100)
	if !bytes.Contains([]byte(output), []byte("{\\an9\\pos(1500,100)}Top Right Subtitle")) {
		t.Errorf("expected Event 2 to be \\an9\\pos(1500,100), got:\n%s", output)
	}
}

func TestWriteASS_AllAlignments(t *testing.T) {
	testCases := []struct {
		name     string
		event    subtitle.Event
		expected string
	}{
		{
			name:     "Bottom-Left (an1)",
			event:    subtitle.Event{Index: 1, StartTime: time.Second, EndTime: 2 * time.Second, Text: "T1", X: 100, Y: 900, Width: 200, Height: 50, VideoWidth: 1920, VideoHeight: 1080},
			expected: "{\\an1\\pos(100,950)}T1",
		},
		{
			name:     "Bottom-Center (an2)",
			event:    subtitle.Event{Index: 2, StartTime: time.Second, EndTime: 2 * time.Second, Text: "T2", X: 860, Y: 900, Width: 200, Height: 50, VideoWidth: 1920, VideoHeight: 1080},
			expected: "{\\an2\\pos(960,950)}T2",
		},
		{
			name:     "Bottom-Right (an3)",
			event:    subtitle.Event{Index: 3, StartTime: time.Second, EndTime: 2 * time.Second, Text: "T3", X: 1600, Y: 900, Width: 200, Height: 50, VideoWidth: 1920, VideoHeight: 1080},
			expected: "{\\an3\\pos(1800,950)}T3",
		},
		{
			name:     "Mid-Left (an4)",
			event:    subtitle.Event{Index: 4, StartTime: time.Second, EndTime: 2 * time.Second, Text: "T4", X: 100, Y: 500, Width: 200, Height: 50, VideoWidth: 1920, VideoHeight: 1080},
			expected: "{\\an4\\pos(100,525)}T4",
		},
		{
			name:     "Mid-Center (an5)",
			event:    subtitle.Event{Index: 5, StartTime: time.Second, EndTime: 2 * time.Second, Text: "T5", X: 860, Y: 500, Width: 200, Height: 50, VideoWidth: 1920, VideoHeight: 1080},
			expected: "{\\an5\\pos(960,525)}T5",
		},
		{
			name:     "Mid-Right (an6)",
			event:    subtitle.Event{Index: 6, StartTime: time.Second, EndTime: 2 * time.Second, Text: "T6", X: 1600, Y: 500, Width: 200, Height: 50, VideoWidth: 1920, VideoHeight: 1080},
			expected: "{\\an6\\pos(1800,525)}T6",
		},
		{
			name:     "Top-Left (an7)",
			event:    subtitle.Event{Index: 7, StartTime: time.Second, EndTime: 2 * time.Second, Text: "T7", X: 100, Y: 100, Width: 200, Height: 50, VideoWidth: 1920, VideoHeight: 1080},
			expected: "{\\an7\\pos(100,100)}T7",
		},
		{
			name:     "Top-Center (an8)",
			event:    subtitle.Event{Index: 8, StartTime: time.Second, EndTime: 2 * time.Second, Text: "T8", X: 860, Y: 100, Width: 200, Height: 50, VideoWidth: 1920, VideoHeight: 1080},
			expected: "{\\an8\\pos(960,100)}T8",
		},
		{
			name:     "Top-Right (an9)",
			event:    subtitle.Event{Index: 9, StartTime: time.Second, EndTime: 2 * time.Second, Text: "T9", X: 1600, Y: 100, Width: 200, Height: 50, VideoWidth: 1920, VideoHeight: 1080},
			expected: "{\\an9\\pos(1800,100)}T9",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := Write(&buf, []subtitle.Event{tc.event})
			if err != nil {
				t.Fatalf("Write error: %v", err)
			}
			out := buf.String()
			if !bytes.Contains([]byte(out), []byte(tc.expected)) {
				t.Errorf("expected %s in output, got:\n%s", tc.expected, out)
			}
		})
	}
}


