package main

import "fmt"

const KiB = 1024
const MiB = KiB * 1024
const GiB = MiB * 1024

func formatBytes(bytes int64) string {
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
