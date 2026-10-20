// SPDX-License-Identifier: Apache-2.0
package richmarkdown

import "strings"

// ConvertLaTeXDelimiters rewrites \(...\) as $...$ and \[...\] as $$...$$.
//
// ChatGPT, and LaTeX habit generally, delimit formulas with escaped brackets.
// Rich Markdown documents only the dollar forms, and in Markdown "\(" is an
// escaped parenthesis, so Telegram renders "\(x^2\)" as "(x^2)": the formula
// is lost without any error. Code is literal and stays untouched: fenced
// blocks, inline code spans, and <code>/<pre> elements.
//
// Two guards keep ordinary Markdown escapes alone. A delimiter whose backslash
// is itself escaped ("\\(") is not a delimiter. And "\[...\]" is converted only
// when it reads as a formula -- on lines of its own, or containing LaTeX
// syntax -- so an escaped reference such as "\[1\]" keeps its brackets.
func ConvertLaTeXDelimiters(source string) string {
	if !strings.Contains(source, `\(`) && !strings.Contains(source, `\[`) {
		return source
	}
	var out strings.Builder
	out.Grow(len(source))
	lines := strings.SplitAfter(source, "\n")
	fence := ""
	var prose []string
	flush := func() {
		if len(prose) > 0 {
			out.WriteString(convertProse(strings.Join(prose, "")))
			prose = prose[:0]
		}
	}
	for _, line := range lines {
		if fence != "" {
			out.WriteString(line)
			if closesFence(line, fence) {
				fence = ""
			}
			continue
		}
		if opened := openingFence(line); opened != "" {
			flush()
			fence = opened
			out.WriteString(line)
			continue
		}
		prose = append(prose, line)
	}
	flush()
	return out.String()
}

// openingFence returns the backtick or tilde run that opens a fenced code
// block on this line, or "" when the line opens none.
func openingFence(line string) string {
	trimmed := strings.TrimLeft(line, " ")
	if len(line)-len(trimmed) > 3 || trimmed == "" {
		return ""
	}
	marker := trimmed[0]
	if marker != '`' && marker != '~' {
		return ""
	}
	run := len(trimmed) - len(strings.TrimLeft(trimmed, string(marker)))
	if run < 3 {
		return ""
	}
	// A backtick fence's info string cannot contain a backtick.
	if marker == '`' && strings.Contains(trimmed[run:], "`") {
		return ""
	}
	return trimmed[:run]
}

func closesFence(line, fence string) bool {
	trimmed := strings.TrimSpace(line)
	if len(line)-len(strings.TrimLeft(line, " ")) > 3 {
		return false
	}
	return len(trimmed) >= len(fence) && strings.Trim(trimmed, fence[:1]) == ""
}

// convertProse rewrites delimiters in text that holds no fenced code block.
func convertProse(text string) string {
	var out strings.Builder
	out.Grow(len(text))
	for i := 0; i < len(text); {
		switch {
		case text[i] == '`':
			end := codeSpanEnd(text, i)
			out.WriteString(text[i:end])
			i = end
		case text[i] == '<':
			end := literalElementEnd(text, i)
			out.WriteString(text[i:end])
			i = end
		case text[i] == '\\' && i+1 < len(text) && (text[i+1] == '(' || text[i+1] == '['):
			if replacement, end, ok := delimitedMath(text, i); ok {
				out.WriteString(replacement)
				i = end
				continue
			}
			out.WriteString(text[i : i+2])
			i += 2
		case text[i] == '\\' && i+1 < len(text):
			// Any other escape, including "\\", consumes both bytes, so the
			// backslash it escapes can never start a delimiter.
			out.WriteString(text[i : i+2])
			i += 2
		default:
			out.WriteByte(text[i])
			i++
		}
	}
	return out.String()
}

// codeSpanEnd returns the index just past the code span opened by the
// backtick run at start, or past the run itself when nothing closes it.
func codeSpanEnd(text string, start int) int {
	run := backtickRun(text, start)
	for i := start + run; i < len(text); {
		if text[i] != '`' {
			i++
			continue
		}
		closing := backtickRun(text, i)
		if closing == run {
			return i + closing
		}
		i += closing
	}
	return start + run
}

func backtickRun(text string, start int) int {
	n := 0
	for start+n < len(text) && text[start+n] == '`' {
		n++
	}
	return n
}

// literalElementEnd skips a <code> or <pre> element whole; any other "<" is
// one ordinary byte.
func literalElementEnd(text string, start int) int {
	for _, tag := range []string{"code", "pre"} {
		open := "<" + tag
		if len(text)-start <= len(open) || !strings.EqualFold(text[start:start+len(open)], open) {
			continue
		}
		next := text[start+len(open)]
		if next != '>' && next != ' ' && next != '\t' && next != '\n' {
			continue
		}
		closing := "</" + tag + ">"
		if end := strings.Index(strings.ToLower(text[start:]), closing); end >= 0 {
			return start + end + len(closing)
		}
	}
	return start + 1
}

// delimitedMath converts the formula whose opening delimiter is at start.
func delimitedMath(text string, start int) (string, int, bool) {
	display := text[start+1] == '['
	closing := `\)`
	if display {
		closing = `\]`
	}
	contentStart := start + 2
	end := -1
	for i := contentStart; i+1 < len(text); i++ {
		if text[i] == '\n' && i+1 < len(text) && strings.TrimSpace(lineAt(text, i+1)) == "" {
			return "", 0, false // A formula does not cross a blank line.
		}
		if text[i] == '\\' {
			if text[i:i+2] == closing {
				end = i
				break
			}
			i++ // Skip the escaped byte: "\\)" is a LaTeX newline then ")".
		}
	}
	if end < 0 {
		return "", 0, false
	}
	content := strings.TrimSpace(text[contentStart:end])
	if content == "" || strings.Contains(content, "$") {
		return "", 0, false
	}
	after := end + 2
	if display && !standsAlone(text, start, after) && !looksLikeLaTeX(content) {
		return "", 0, false
	}
	// Newlines are whitespace in LaTeX; one line keeps the documented
	// single-line $$...$$ form.
	content = strings.Join(strings.Fields(content), " ")
	if display {
		return "$$" + content + "$$", after, true
	}
	return "$" + content + "$", after, true
}

// lineAt returns the line starting at index i, without its newline.
func lineAt(text string, i int) string {
	if end := strings.IndexByte(text[i:], '\n'); end >= 0 {
		return text[i : i+end]
	}
	return text[i:]
}

// standsAlone reports whether text[start:end] is the only thing on its lines.
func standsAlone(text string, start, end int) bool {
	lineStart := strings.LastIndexByte(text[:start], '\n') + 1
	lineEnd := len(text)
	if i := strings.IndexByte(text[end:], '\n'); i >= 0 {
		lineEnd = end + i
	}
	return strings.TrimSpace(text[lineStart:start]) == "" && strings.TrimSpace(text[end:lineEnd]) == ""
}

func looksLikeLaTeX(content string) bool {
	return strings.ContainsAny(content, `\^_={}`)
}
