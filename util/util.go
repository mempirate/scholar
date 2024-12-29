package util

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
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

// DownloadPDF downloads a PDF from a URL and returns the path to the downloaded file.
func DownloadPDF(url string) (path string, err error) {
	// Extract file name from URL
	fileName := url[strings.LastIndex(url, "/")+1:]

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to download file: %w", err)
	}

	defer resp.Body.Close()

	ct := resp.Header.Get("Content-Type")
	if ct == "" && ct != "application/pdf" {
		return "", fmt.Errorf("invalid content-type: %s", ct)
	}

	reader := bufio.NewReader(resp.Body)

	if !strings.HasSuffix(fileName, ".pdf") {
		fileName += ".pdf"
	}

	const pdfMagicNumber = "%PDF-"
	buf, err := reader.Peek(len(pdfMagicNumber))
	if err != nil {
		return "", fmt.Errorf("failed to read file header: %w", err)
	}

	if !bytes.Equal(buf, []byte(pdfMagicNumber)) {
		return "", errors.New("invalid magic")
	}

	path = fmt.Sprintf("/tmp/%s", fileName)

	// Create a temporary file
	tmpFile, err := os.Create(path)

	// Write the body to file
	_, err = io.Copy(tmpFile, reader)
	if err != nil {
		return "", fmt.Errorf("failed to write to file: %w", err)
	}

	return path, nil
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
