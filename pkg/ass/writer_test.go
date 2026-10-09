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

