package util

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/deemkeen/stegodon/assets"
	"golang.org/x/image/draw"
)

const halfBlock = "▀"

// RenderImageToHalfBlocks renders an image as a string using Unicode half-block
// characters with ANSI256 foreground/background colors. Each character cell encodes
// 2 vertical pixels: foreground color for the top pixel, background for the bottom.
func RenderImageToHalfBlocks(img image.Image, cols, rows int) string {
	// Resize image to cols x (rows*2) pixels
	pixelHeight := rows * 2
	dst := image.NewRGBA(image.Rect(0, 0, cols, pixelHeight))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Over, nil)

	var sb strings.Builder
	for row := 0; row < rows; row++ {
		topY := row * 2
		botY := topY + 1

		for col := 0; col < cols; col++ {
			tr, tg, tb, _ := dst.At(col, topY).RGBA()
			br, bg, bb, _ := dst.At(col, botY).RGBA()

			fg := rgbToAnsi256(uint8(tr>>8), uint8(tg>>8), uint8(tb>>8))
			bgc := rgbToAnsi256(uint8(br>>8), uint8(bg>>8), uint8(bb>>8))

			sb.WriteString(fmt.Sprintf("\033[38;5;%dm\033[48;5;%dm%s", fg, bgc, halfBlock))
		}
		sb.WriteString("\033[0m")
		if row < rows-1 {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

// LoadAvatarImage loads an avatar image from disk given a relative URL like /avatars/{uuid}.png.
// Returns nil if the URL is empty, the file doesn't exist, or decoding fails.
func LoadAvatarImage(avatarURL string) image.Image {
	if avatarURL == "" {
		return nil
	}

	// Extract filename from URL path (e.g. "/avatars/abc.png" -> "abc.png")
	filename := filepath.Base(avatarURL)
	if filename == "." || filename == "/" {
		return nil
	}

	path := ResolveFilePathWithSubdir("avatars", filename)

	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return nil
	}
	return img
}

// LoadDefaultAvatarImage returns the embedded stegodon logo as a fallback avatar.
func LoadDefaultAvatarImage() image.Image {
	img, _, err := image.Decode(bytes.NewReader(assets.StegoLogo))
	if err != nil {
		return nil
	}
	return img
}

// rgbToAnsi256 maps an RGB color to the nearest ANSI256 color index.
// It checks both the 6x6x6 color cube (indices 16-231) and the 24-step
// grayscale ramp (indices 232-255), returning whichever is closer.
func rgbToAnsi256(r, g, b uint8) int {
	// Find nearest in the 6x6x6 color cube (indices 16-231)
	cubeR := cubeIndex(r)
	cubeG := cubeIndex(g)
	cubeB := cubeIndex(b)
	cubeIdx := 16 + 36*cubeR + 6*cubeG + cubeB

	// Reconstruct the cube color for distance comparison
	cubeRV := cubeValue(cubeR)
	cubeGV := cubeValue(cubeG)
	cubeBV := cubeValue(cubeB)
	cubeDist := colorDist(r, g, b, cubeRV, cubeGV, cubeBV)

	// Find nearest in the grayscale ramp (indices 232-255)
	// Grayscale values: 8, 18, 28, ..., 238 (24 steps, step size 10)
	gray := float64(r)*0.299 + float64(g)*0.587 + float64(b)*0.114
	grayIdx := int(math.Round((gray - 8.0) / 10.0))
	if grayIdx < 0 {
		grayIdx = 0
	} else if grayIdx > 23 {
		grayIdx = 23
	}
	grayValue := uint8(8 + 10*grayIdx)
	grayDist := colorDist(r, g, b, grayValue, grayValue, grayValue)

	if grayDist < cubeDist {
		return 232 + grayIdx
	}
	return cubeIdx
}

// cubeIndex maps an 8-bit color component to a 6x6x6 cube index (0-5).
func cubeIndex(v uint8) int {
	// The cube values are: 0, 95, 135, 175, 215, 255
	// Find the nearest one
	if v < 48 {
		return 0
	} else if v < 115 {
		return 1
	} else if v < 155 {
		return 2
	} else if v < 195 {
		return 3
	} else if v < 235 {
		return 4
	}
	return 5
}

// cubeValue returns the actual RGB value for a cube index.
func cubeValue(idx int) uint8 {
	if idx == 0 {
		return 0
	}
	return uint8(55 + 40*idx)
}

// colorDist computes squared Euclidean distance between two RGB colors.
func colorDist(r1, g1, b1, r2, g2, b2 uint8) float64 {
	dr := float64(r1) - float64(r2)
	dg := float64(g1) - float64(g2)
	db := float64(b1) - float64(b2)
	return dr*dr + dg*dg + db*db
}

// LoadRemoteAvatarImage fetches an avatar image from a remote URL.
// Returns nil if the URL is empty, the request fails, or decoding fails.
// Uses a 5-second timeout to avoid blocking on slow/unresponsive servers.
func LoadRemoteAvatarImage(url string) image.Image {
	if url == "" {
		return nil
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	img, _, err := image.Decode(resp.Body)
	if err != nil {
		return nil
	}
	return img
}
