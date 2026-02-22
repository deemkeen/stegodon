package util

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
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
// characters with 24-bit true color foreground/background. Each character cell encodes
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

			sb.WriteString(fmt.Sprintf("\033[38;2;%d;%d;%dm\033[48;2;%d;%d;%dm%s",
				tr>>8, tg>>8, tb>>8, br>>8, bg>>8, bb>>8, halfBlock))
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
