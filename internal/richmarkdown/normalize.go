// SPDX-License-Identifier: Apache-2.0
package richmarkdown

import (
	"html"
	"regexp"
	"strings"
)

var (
	linkedImage = regexp.MustCompile(`(?is)<a\s+[^>]*href=["']([^"']+)["'][^>]*>\s*<img\s+([^>]*)/?>\s*</a>`)
	altAttr     = regexp.MustCompile(`(?is)\balt=["']([^"']*)["']`)
	imageTag    = regexp.MustCompile(`(?is)<img\s+([^>]*)/?>`)
	headingTag  = regexp.MustCompile(`(?is)<h([1-6])(?:\s+[^>]*)?>(.*?)</h[1-6]>`)
	paragraph   = regexp.MustCompile(`(?is)</?p(?:\s+[^>]*)?>`)
	div         = regexp.MustCompile(`(?is)</?div(?:\s+[^>]*)?>`)
	breakTag    = regexp.MustCompile(`(?is)<br\s*/?>`)
)

// NormalizeFallback converts common GitHub README layout HTML into Telegram's
// Rich Markdown subset. It is used only after Telegram definitively rejects the
// exact source, so valid input always keeps the byte-for-byte fast path.
func NormalizeFallback(source string) string {
	result := linkedImage.ReplaceAllStringFunc(source, func(match string) string {
		parts := linkedImage.FindStringSubmatch(match)
		label := "link"
		if alt := altAttr.FindStringSubmatch(parts[2]); len(alt) == 2 && strings.TrimSpace(alt[1]) != "" {
			label = html.UnescapeString(alt[1])
		}
		return "[" + escapeLabel(label) + "](" + parts[1] + ")"
	})
	result = imageTag.ReplaceAllStringFunc(result, func(match string) string {
		parts := imageTag.FindStringSubmatch(match)
		if alt := altAttr.FindStringSubmatch(parts[1]); len(alt) == 2 {
			return html.UnescapeString(alt[1])
		}
		return ""
	})
	result = headingTag.ReplaceAllStringFunc(result, func(match string) string {
		parts := headingTag.FindStringSubmatch(match)
		level := int(parts[1][0] - '0')
		return strings.Repeat("#", level) + " " + stripSimpleInlineHTML(parts[2])
	})
	result = paragraph.ReplaceAllString(result, "\n")
	result = div.ReplaceAllString(result, "\n")
	result = breakTag.ReplaceAllString(result, "\n")
	result = stripSimpleInlineHTML(result)
	return collapseBlankLines(result)
}

func stripSimpleInlineHTML(value string) string {
	replacer := strings.NewReplacer("<strong>", "**", "</strong>", "**", "<b>", "**", "</b>", "**", "<em>", "_", "</em>", "_", "<i>", "_", "</i>", "_")
	return replacer.Replace(value)
}

func escapeLabel(value string) string {
	return strings.NewReplacer(`\`, `\\`, `[`, `\[`, `]`, `\]`).Replace(value)
}

func collapseBlankLines(value string) string {
	for strings.Contains(value, "\n\n\n") {
		value = strings.ReplaceAll(value, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(value)
}
