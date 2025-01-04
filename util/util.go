package util

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"

	"github.com/pkg/errors"
)

const KiB = 1024
const MiB = KiB * 1024
const GiB = MiB * 1024

func FormatBytes(bytes int64) string {
	if bytes < KiB {
		return fmt.Sprintf("%dB", bytes)
	} else if bytes < MiB {
		return fmt.Sprintf("%.1fKiB", float64(bytes)/KiB)
	} else if bytes < GiB {
		return fmt.Sprintf("%.1fMiB", float64(bytes)/MiB)
	} else {
		return fmt.Sprintf("%.1fGiB", float64(bytes)/GiB)
	}
}

// DownloadContent downloads the content from a URL and returns it, with the content type.
// The content type is determined by the Content-Type header of the response.
func DownloadContent(url *url.URL) (body []byte, ct string, err error) {
	resp, err := http.Get(url.String())
	if err != nil {
		return nil, "", errors.Wrap(err, "failed to download file")
	}

	defer resp.Body.Close()

	ct, _, err = mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil {
		return nil, "", errors.Wrap(err, "failed to parse content type")
	}

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", errors.Wrap(err, "failed to read file")
	}

	return
}

func DownloadWebPage(url *url.URL) (name string, body []byte, err error) {
	resp, err := http.Get(url.String())
	if err != nil {
		return "", nil, fmt.Errorf("failed to download file: %w", err)
	}

	defer resp.Body.Close()

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read file: %w", err)
	}

	return url.Host, content, nil
}

// generateRandomString creates a random string of the specified length in base64.
func generateRandomString(length int) (string, error) {
	// Calculate the number of bytes needed for the desired string length.
	byteLength := (length*6 + 7) / 8 // 6 bits per character, rounded up

	// Create a byte slice to hold the random data.
	randomBytes := make([]byte, byteLength)

	// Read random bytes from the crypto/rand package.
	_, err := rand.Read(randomBytes)
	if err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Encode the random bytes to a base64 string.
	randomString := base64.URLEncoding.EncodeToString(randomBytes)

	// Trim the string to the desired length.
	return randomString[:length], nil
}
