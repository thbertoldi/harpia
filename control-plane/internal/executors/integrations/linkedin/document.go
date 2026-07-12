package linkedin

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	artifactsv1 "github.com/harpia/control-plane/gen/harpia/artifacts/v1"
)

// BuildCarouselDocument renders CarouselDraft as a deliberately small, stable
// PDF. It uses PDF's built-in Helvetica font and never writes metadata, dates,
// UUIDs, compression streams, or map-iteration-derived output. The artifact
// hash is therefore a hash of the reviewed structured carousel alone.
func BuildCarouselDocument(draft *artifactsv1.CarouselDraft) ([]byte, error) {
	if draft == nil || strings.TrimSpace(draft.GetTitle()) == "" || len(draft.GetSlides()) == 0 {
		return nil, fmt.Errorf("carousel title and at least one slide are required")
	}

	objects := []string{"<< /Type /Catalog /Pages 2 0 R >>"}
	pageIDs := make([]int, len(draft.GetSlides()))
	streamIDs := make([]int, len(draft.GetSlides()))
	for i := range draft.GetSlides() {
		pageIDs[i] = 3 + i*2
		streamIDs[i] = pageIDs[i] + 1
	}
	fontID := 3 + len(draft.GetSlides())*2

	kids := make([]string, 0, len(pageIDs))
	for _, id := range pageIDs {
		kids = append(kids, fmt.Sprintf("%d 0 R", id))
	}
	objects = append(objects, fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), len(pageIDs)))
	for i, slide := range draft.GetSlides() {
		objects = append(objects,
			fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 %d 0 R >> >> /Contents %d 0 R >>", fontID, streamIDs[i]),
			pdfStream(slideText(draft, slide, i)),
		)
	}
	objects = append(objects, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")

	var out bytes.Buffer
	out.WriteString("%PDF-1.4\n%\xE2\xE3\xCF\xD3\n")
	offsets := make([]int, len(objects)+1)
	for id, object := range objects {
		offsets[id+1] = out.Len()
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", id+1, object)
	}
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&out, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	return out.Bytes(), nil
}

func pdfStream(lines []pdfLine) string {
	var content strings.Builder
	content.WriteString("BT\n/F1 28 Tf\n72 720 Td\n")
	for i, line := range lines {
		if i > 0 {
			content.WriteString("0 -")
			content.WriteString(strconv.Itoa(line.leading))
			content.WriteString(" Td\n")
		}
		content.WriteString("/")
		content.WriteString(line.font)
		content.WriteString(" ")
		content.WriteString(strconv.Itoa(line.size))
		content.WriteString(" Tf\n(")
		content.WriteString(pdfEscape(line.text))
		content.WriteString(") Tj\n")
	}
	content.WriteString("ET")
	return fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content.String()), content.String())
}

type pdfLine struct {
	text, font    string
	size, leading int
}

func slideText(draft *artifactsv1.CarouselDraft, slide *artifactsv1.CarouselSlide, index int) []pdfLine {
	lines := []pdfLine{{text: fmt.Sprintf("%s — %d", draft.GetTitle(), index+1), font: "F1", size: 16, leading: 28}}
	if index == 0 && strings.TrimSpace(draft.GetHook()) != "" {
		lines = append(lines, wrappedPDFLines(draft.GetHook(), 26, 34)...)
	}
	if slide != nil {
		if strings.TrimSpace(slide.GetHeading()) != "" {
			lines = append(lines, wrappedPDFLines(slide.GetHeading(), 24, 32)...)
		}
		if strings.TrimSpace(slide.GetBody()) != "" {
			lines = append(lines, wrappedPDFLines(slide.GetBody(), 15, 22)...)
		}
	}
	return lines
}

func wrappedPDFLines(text string, size, leading int) []pdfLine {
	const maxChars = 62
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}
	lines := make([]pdfLine, 0, 2)
	line := ""
	for _, word := range words {
		if len(line) > 0 && len(line)+1+len(word) > maxChars {
			lines = append(lines, pdfLine{text: line, font: "F1", size: size, leading: leading})
			line = word
			continue
		}
		if line != "" {
			line += " "
		}
		line += word
	}
	return append(lines, pdfLine{text: line, font: "F1", size: size, leading: leading})
}

func pdfEscape(value string) string {
	var escaped strings.Builder
	for _, r := range value {
		switch r {
		case '\\', '(', ')':
			escaped.WriteByte('\\')
			escaped.WriteRune(r)
		case '\n', '\r':
			escaped.WriteByte(' ')
		default:
			if r < 32 || r > 126 {
				escaped.WriteByte('?')
			} else {
				escaped.WriteRune(r)
			}
		}
	}
	return escaped.String()
}
