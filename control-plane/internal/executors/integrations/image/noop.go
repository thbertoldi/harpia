package image

import (
	"context"
	"fmt"
	"strings"
)

// NoopProvider generates a minimal SVG placeholder so plans can complete end to
// end without external API calls. It is the default dev/test provider.
type NoopProvider struct{}

func NewNoopProvider() *NoopProvider {
	return &NoopProvider{}
}

func (NoopProvider) Generate(_ context.Context, req GenerateRequest) (GenerateResult, error) {
	width, height := parseDimensions(req.Size, 1024, 1024)
	label := truncForLabel(req.Prompt, 80)

	svg := buildPlaceholderSVG(width, height, label)
	return GenerateResult{
		ImageBytes: []byte(svg),
		MimeType:   "image/svg+xml",
		Width:      int32(width),
		Height:     int32(height),
	}, nil
}

// buildPlaceholderSVG renders a deterministic solid-color SVG with the prompt
// text. Escaping keeps the embedded text safe inside the SVG body.
func buildPlaceholderSVG(width, height int, label string) string {
	const bg = "#0f172a"
	const fg = "#e2e8f0"
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`, width, height, width, height)
	fmt.Fprintf(&b, `<rect width="%d" height="%d" fill="%s"/>`, width, height, bg)
	if label != "" {
		// Word-agnostic single-line label; svgEscape neutralizes markup.
		fmt.Fprintf(&b, `<text x="50%%" y="50%%" font-family="sans-serif" font-size="28" fill="%s" text-anchor="middle" dominant-baseline="middle">%s</text>`, fg, svgEscape(label))
	}
	b.WriteString(`</svg>`)
	return b.String()
}

func svgEscape(s string) string {
	r := strings.NewReplacer(
		`&`, `&amp;`,
		`<`, `&lt;`,
		`>`, `&gt;`,
		`"`, `&#34;`,
		`'`, `&#39;`,
	)
	return r.Replace(s)
}

func truncForLabel(s string, max int) string {
	s = strings.TrimSpace(s)
	if max > 0 && len(s) > max {
		return s[:max] + "…"
	}
	return s
}

// parseDimensions parses a "WxH" size string, falling back to the supplied
// defaults when it is empty or malformed.
func parseDimensions(size string, defaultW, defaultH int) (int, int) {
	size = strings.TrimSpace(size)
	if size == "" {
		return defaultW, defaultH
	}
	var w, h int
	if _, err := fmt.Sscanf(size, "%dx%d", &w, &h); err != nil || w <= 0 || h <= 0 {
		return defaultW, defaultH
	}
	return w, h
}
