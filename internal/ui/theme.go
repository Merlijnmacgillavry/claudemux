package ui

// Theme holds the current color theme.
// In the future we can support light/dark themes here.
type Theme struct {
	Primary        string
	Secondary      string
	Background     string
	Surface        string
	Text           string
	TextSecondary  string
	TextMuted      string
	TextDim        string
	InactiveBorder string
	Success        string
	Warning        string
	Error          string
}

var DarkTheme = Theme{
	Primary:        "#D97757", // Claude terracotta orange
	Secondary:      "#7C2D12", // dark burnt-orange — selected item background
	Background:     "#0F0F0F",
	Surface:        "#374151", // status bar / toolbar background
	Text:           "#FAFAF9", // warm near-white
	TextSecondary:  "#E5E7EB", // normal item text
	TextMuted:      "#D1D5DB", // preview / secondary text
	TextDim:        "#9CA3AF", // timestamps / dim labels
	InactiveBorder: "#6B7280", // visible but subdued
	Success:        "#10B981",
	Warning:        "#F59E0B",
	Error:          "#EF4444",
}
