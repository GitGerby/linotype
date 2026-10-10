package subtitle

import (
	"image/color"
	"testing"
)

func TestIsCustomTextColor(t *testing.T) {
	tests := []struct {
		name     string
		color    color.RGBA
		expected bool
	}{
		{
			name:     "Pure white",
			color:    color.RGBA{255, 255, 255, 255},
			expected: false,
		},
		{
			name:     "Rec.709 studio white",
			color:    color.RGBA{235, 235, 235, 255},
			expected: false,
		},
		{
			name:     "Antialiased off-white",
			color:    color.RGBA{240, 245, 238, 255},
			expected: false,
		},
		{
			name:     "Pure black outline",
			color:    color.RGBA{0, 0, 0, 255},
			expected: false,
		},
		{
			name:     "Video black outline",
			color:    color.RGBA{16, 16, 16, 255},
			expected: false,
		},
		{
			name:     "Dark gray shadow",
			color:    color.RGBA{50, 50, 50, 255},
			expected: false,
		},
		{
			name:     "Transparent",
			color:    color.RGBA{0, 0, 0, 0},
			expected: false,
		},
		{
			name:     "Bright red",
			color:    color.RGBA{255, 0, 0, 255},
			expected: true,
		},
		{
			name:     "Bright yellow",
			color:    color.RGBA{255, 255, 0, 255},
			expected: true,
		},
		{
			name:     "DVD/Blu-ray yellow",
			color:    color.RGBA{238, 210, 3, 255},
			expected: true,
		},
		{
			name:     "Cyan",
			color:    color.RGBA{0, 255, 255, 255},
			expected: true,
		},
		{
			name:     "Green",
			color:    color.RGBA{0, 255, 0, 255},
			expected: true,
		},
		{
			name:     "Pure blue",
			color:    color.RGBA{0, 0, 255, 255},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsCustomTextColor(tt.color)
			if got != tt.expected {
				t.Errorf("IsCustomTextColor(%+v) = %v, want %v", tt.color, got, tt.expected)
			}
		})
	}
}

