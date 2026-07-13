// SPDX-License-Identifier: Apache-2.0
package richmarkdown

import (
	"html"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/FreshLabDev/makeitMD/internal/telegram"
)

type entityMarker struct {
	start, end  int
	open, close string
	order       int
}

// RestoreEntities reconstructs formatting consumed by the Telegram client
// before the Bot API delivered Message.text. Entity offsets are UTF-16 code
// units, not bytes or runes.
func RestoreEntities(text string, entities []telegram.MessageEntity) string {
	if text == "" || len(entities) == 0 {
		return text
	}
	boundaries := utf16Boundaries(text)
	markers := make([]entityMarker, 0, len(entities))
	prefixes := make(map[int][]string)
	for index, entity := range entities {
		start, startOK := boundaries[entity.Offset]
		end, endOK := boundaries[entity.Offset+entity.Length]
		if !startOK || !endOK || start >= end {
			continue
		}
		if entity.Type == "blockquote" || entity.Type == "expandable_blockquote" {
			prefixes[start] = append(prefixes[start], "> ")
			for pos := start; pos < end; pos++ {
				if text[pos] == '\n' && pos+1 < end {
					prefixes[pos+1] = append(prefixes[pos+1], "> ")
				}
			}
			continue
		}
		open, close := entityDelimiters(text[start:end], entity)
		if open == "" && close == "" {
			continue
		}
		markers = append(markers, entityMarker{start: start, end: end, open: open, close: close, order: index})
	}
	if len(markers) == 0 && len(prefixes) == 0 {
		return text
	}
	opens := make(map[int][]entityMarker)
	closes := make(map[int][]entityMarker)
	positions := make(map[int]struct{})
	positions[0] = struct{}{}
	positions[len(text)] = struct{}{}
	for _, marker := range markers {
		opens[marker.start] = append(opens[marker.start], marker)
		closes[marker.end] = append(closes[marker.end], marker)
		positions[marker.start] = struct{}{}
		positions[marker.end] = struct{}{}
	}
	for position := range prefixes {
		positions[position] = struct{}{}
	}
	ordered := make([]int, 0, len(positions))
	for position := range positions {
		ordered = append(ordered, position)
	}
	sort.Ints(ordered)
	var out strings.Builder
	out.Grow(len(text) + len(markers)*7)
	previous := 0
	for _, position := range ordered {
		out.WriteString(text[previous:position])
		closing := closes[position]
		sort.SliceStable(closing, func(i, j int) bool {
			if closing[i].start != closing[j].start {
				return closing[i].start > closing[j].start
			}
			return closing[i].order > closing[j].order
		})
		for _, marker := range closing {
			out.WriteString(marker.close)
		}
		for _, prefix := range prefixes[position] {
			out.WriteString(prefix)
		}
		opening := opens[position]
		sort.SliceStable(opening, func(i, j int) bool {
			if opening[i].end != opening[j].end {
				return opening[i].end > opening[j].end
			}
			return opening[i].order < opening[j].order
		})
		for _, marker := range opening {
			out.WriteString(marker.open)
		}
		previous = position
	}
	return out.String()
}

func utf16Boundaries(text string) map[int]int {
	boundaries := map[int]int{0: 0}
	units := 0
	for byteIndex, r := range text {
		units += utf16.RuneLen(r)
		boundaries[units] = byteIndex + len(string(r))
	}
	return boundaries
}

func entityDelimiters(content string, entity telegram.MessageEntity) (string, string) {
	switch entity.Type {
	case "bold":
		return "<b>", "</b>"
	case "italic":
		return "<i>", "</i>"
	case "underline":
		return "<u>", "</u>"
	case "strikethrough":
		return "<s>", "</s>"
	case "spoiler":
		return "<tg-spoiler>", "</tg-spoiler>"
	case "code":
		fence := strings.Repeat("`", longestBacktickRun(content)+1)
		return fence, fence
	case "pre":
		fence := strings.Repeat("`", max(3, longestBacktickRun(content)+1))
		language := strings.Map(func(r rune) rune {
			if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-' {
				return r
			}
			return -1
		}, entity.Language)
		return fence + language + "\n", "\n" + fence
	case "text_link":
		return `<a href="` + html.EscapeString(entity.URL) + `">`, "</a>"
	case "text_mention":
		if entity.User != nil {
			return `<a href="tg://user?id=` + strconv.FormatInt(entity.User.ID, 10) + `">`, "</a>"
		}
	case "custom_emoji":
		if entity.CustomEmojiID != "" {
			return `<tg-emoji emoji-id="` + html.EscapeString(entity.CustomEmojiID) + `">`, "</tg-emoji>"
		}
	}
	return "", ""
}

func longestBacktickRun(text string) int {
	longest, current := 0, 0
	for _, r := range text {
		if r == '`' {
			current++
			if current > longest {
				longest = current
			}
		} else {
			current = 0
		}
	}
	return longest
}
