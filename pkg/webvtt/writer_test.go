package webvtt

import (
	"bytes"
	"image/color"
	"strings"
	"testing"
	"time"

	"git.dyson.somuchcrypto.com/gearnsc/linotype/pkg/subtitle"
)

func TestWriteWebVTT(t *testing.T) {
	events := []subtitle.Event{
		{
			Index:       1,
			StartTime:   time.Second + 234*time.Millisecond,
			EndTime:     2*time.Second + 567*time.Millisecond,
			Text:        "Hello\nWorld",
			X:           200,
			Y:           800,
			Width:       100,
			Height:      50,
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
			Y:           540,
			Width:       0,
			Height:      0,
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
	
	// Check Event 1
	expectedBlock1 := `1
00:00:01.234 --> 00:00:02.567 line:74% position:13% align:start
Hello
World`
	if !strings.Contains(output, expectedBlock1) {
		t.Errorf("Event 1 not formatted correctly. Got:\n%s", output)
	}

	// Check Event 2
	expectedBlock2 := `2
01:00:00.000 --> 01:00:01.000 line:50% position:50% align:center
<font color="#FF0000">Test 2</font>`
	if !strings.Contains(output, expectedBlock2) {
		t.Errorf("Event 2 not formatted correctly. Got:\n%s", output)
	}
}

func TestWriteWebVTT_4K(t *testing.T) {
	events := []subtitle.Event{
		{
			Index:       1,
			StartTime:   time.Second + 234*time.Millisecond,
			EndTime:     2*time.Second + 567*time.Millisecond,
			Text:        "4K Subtitle",
			X:           400,
			Y:           1600,
			Width:       200,
			Height:      100,
			VideoWidth:  3840,
			VideoHeight: 2160,
		},
	}

	var buf bytes.Buffer
	err := Write(&buf, events)
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}

	output := buf.String()
	expectedBlock := `1
00:00:01.234 --> 00:00:02.567 line:74% position:13% align:start
4K Subtitle`
	if !strings.Contains(output, expectedBlock) {
		t.Errorf("4K event not formatted correctly. Got:\n%s", output)
	}
}

func TestWriteWebVTT_ColorHandling(t *testing.T) {
	events := []subtitle.Event{
		{
			Index:       1,
			StartTime:   time.Second,
			EndTime:     2 * time.Second,
			Text:        "Black outline color must not produce font color tag",
			VideoWidth:  1920,
			VideoHeight: 1080,
			Color:       color.RGBA{16, 16, 16, 255}, // Studio black
		},
		{
			Index:       2,
			StartTime:   3 * time.Second,
			EndTime:     4 * time.Second,
			Text:        "Rec.709 studio white must not produce font color tag",
			VideoWidth:  1920,
			VideoHeight: 1080,
			Color:       color.RGBA{235, 235, 235, 255},
		},
		{
			Index:       3,
			StartTime:   5 * time.Second,
			EndTime:     6 * time.Second,
			Text:        "Yellow subtitle text must produce font color tag",
			VideoWidth:  1920,
			VideoHeight: 1080,
			Color:       color.RGBA{255, 255, 0, 255},
		},
	}

	var buf bytes.Buffer
	if err := Write(&buf, events); err != nil {
		t.Fatalf("Write error: %v", err)
	}
	out := buf.String()

	if strings.Contains(out, "<font color=\"#101010\">") || strings.Contains(out, "<font color=\"#000000\">") {
		t.Errorf("black/dark font color tag generated in WebVTT output:\n%s", out)
	}
	if strings.Contains(out, "<font color=\"#EBEBEB\">") {
		t.Errorf("studio white font color tag generated in WebVTT output:\n%s", out)
	}
	if !strings.Contains(out, "<font color=\"#FFFF00\">Yellow subtitle text must produce font color tag</font>") {
		t.Errorf("expected yellow font color tag in WebVTT output:\n%s", out)
	}
}



