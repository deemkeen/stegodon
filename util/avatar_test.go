package util

import (
	"image"
	"image/color"
	"strings"
	"testing"
)

func TestRenderImageToHalfBlocks_1x2(t *testing.T) {
	// Create a 1x2 image: top pixel red, bottom pixel blue
	img := image.NewRGBA(image.Rect(0, 0, 1, 2))
	img.Set(0, 0, color.RGBA{255, 0, 0, 255})
	img.Set(0, 1, color.RGBA{0, 0, 255, 255})

	result := RenderImageToHalfBlocks(img, 1, 1)

	if !strings.Contains(result, "▀") {
		t.Error("Expected half-block character in output")
	}
	if !strings.Contains(result, "\033[38;2;") {
		t.Error("Expected true-color foreground ANSI escape code")
	}
	if !strings.Contains(result, "\033[48;2;") {
		t.Error("Expected true-color background ANSI escape code")
	}
	if !strings.HasSuffix(result, "\033[0m") {
		t.Error("Expected reset escape code at end")
	}
}

func TestRenderImageToHalfBlocks_MultiRow(t *testing.T) {
	// Create a 2x4 image (should render as 2 cols x 2 rows of characters)
	img := image.NewRGBA(image.Rect(0, 0, 2, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 2; x++ {
			img.Set(x, y, color.RGBA{100, 150, 200, 255})
		}
	}

	result := RenderImageToHalfBlocks(img, 2, 2)

	lines := strings.Split(result, "\n")
	if len(lines) != 2 {
		t.Errorf("Expected 2 lines, got %d", len(lines))
	}

	for i, line := range lines {
		if !strings.HasSuffix(line, "\033[0m") {
			t.Errorf("Line %d should end with reset code", i)
		}
	}
}

func TestRenderImageToHalfBlocks_TrueColorValues(t *testing.T) {
	// Create a 1x2 image with known colors
	img := image.NewRGBA(image.Rect(0, 0, 1, 2))
	img.Set(0, 0, color.RGBA{255, 0, 0, 255}) // red foreground
	img.Set(0, 1, color.RGBA{0, 255, 0, 255}) // green background

	result := RenderImageToHalfBlocks(img, 1, 1)

	if !strings.Contains(result, "\033[38;2;255;0;0m") {
		t.Errorf("Expected true-color red foreground, got: %s", result)
	}
	if !strings.Contains(result, "\033[48;2;0;255;0m") {
		t.Errorf("Expected true-color green background, got: %s", result)
	}
}

func TestLoadAvatarImage_EmptyURL(t *testing.T) {
	img := LoadAvatarImage("")
	if img != nil {
		t.Error("Expected nil for empty URL")
	}
}

func TestLoadAvatarImage_NonexistentFile(t *testing.T) {
	img := LoadAvatarImage("/avatars/nonexistent-file-12345.png")
	if img != nil {
		t.Error("Expected nil for nonexistent file")
	}
}
