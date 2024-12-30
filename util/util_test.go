package util

import (
	"net/url"
	"testing"
)

func TestDownloadPDF(t *testing.T) {
	uri, err := url.Parse("https://bitcoin.org/bitcoin.pdf")
	if err != nil {
		t.Error(err)
	}

	path, err := DownloadPDF(uri)
	if err != nil {
		t.Error(err)
	}

	if path == "" {
		t.Error("DownloadPDF failed to download a PDF")
	}

	uri, err = url.Parse("https://github.com/pdfcpu/pdfcpu")
	if err != nil {
		t.Error(err)
	}

	_, err = DownloadPDF(uri)
	if err == nil {
		t.Error("DownloadPDF failed to return an error")
	}

	uri, err = url.Parse("example.com")
	if err != nil {
		t.Error(err)
	}
	t.Log(uri)

	uri, _ = url.Parse("https://whitepaper.renegade.fi/")

	path, err = DownloadPDF(uri)
	t.Log(path)

	uri, _ = url.Parse("https://www.provewith.us")

	path, err = DownloadPDF(uri)
	t.Log(path)
}
