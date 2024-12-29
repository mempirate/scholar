package util

import "testing"

func TestDownloadPDF(t *testing.T) {
	url := "https://bitcoin.org/bitcoin.pdf"

	path, err := DownloadPDF(url)
	if err != nil {
		t.Error(err)
	}

	if path == "" {
		t.Error("DownloadPDF failed to download a PDF")
	}

	url = "https://github.com/pdfcpu/pdfcpu"

	_, err = DownloadPDF(url)
	if err == nil {
		t.Error("DownloadPDF failed to return an error")
	}

}
