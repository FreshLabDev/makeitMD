// SPDX-License-Identifier: Apache-2.0
package richmarkdown

import (
	"regexp"
	"strings"
	"unicode"
)

// WrapBareFormulas wraps lines of undelimited LaTeX in $$...$$.
//
// Chatbot apps on phones copy a display formula as its bare LaTeX source,
// "A=P\cdot\frac{r}{1-(1+r)^{-n}}", with no delimiters left. A line becomes a
// formula only when it reads as one: a LaTeX command, or a sub/superscript
// with a relation, and no word of three or more letters once commands and
// braced arguments are removed, so prose, code and links stay text. An
// environment split over lines, \begin{cases} to \end{cases}, is one formula,
// and consecutive formula lines are joined: a phone breaks long formulas. The
// <b>/<i> a client puts around parts of a formula are dropped.
//
// Some apps copy a display formula as a code block with no language;
// such a block that holds only formulas is one too. Source that already
// delimits any formula ("$", "\(", "\[") is left alone, and so is other code.
func WrapBareFormulas(source string) string {
	if strings.Contains(source, "$") || strings.Contains(source, `\(`) || strings.Contains(source, `\[`) {
		return source
	}
	if !strings.Contains(source, `\`) && !strings.ContainsAny(source, "^_") {
		return source
	}
	lines := strings.Split(source, "\n")
	out := make([]string, 0, len(lines))
	fence := ""
	for i := 0; i < len(lines); {
		line := lines[i]
		if fence != "" {
			out = append(out, line)
			if closesFence(line, fence) {
				fence = ""
			}
			i++
			continue
		}
		if opened := openingFence(line); opened != "" {
			if formula, end, ok := fencedFormula(lines, i, opened); ok {
				out = append(out, "$$"+formula+"$$")
				i = end
				continue
			}
			fence = opened
			out = append(out, line)
			i++
			continue
		}
		var parts []string
		next := i
		for next < len(lines) {
			formula, end, ok := formulaAt(lines, next)
			if !ok {
				break
			}
			parts = append(parts, formula)
			next = end
		}
		if len(parts) == 0 {
			out = append(out, line)
			i++
			continue
		}
		out = append(out, "$$"+strings.Join(parts, " ")+"$$")
		i = next
	}
	return strings.Join(out, "\n")
}

// fencedFormula reads a code block with no language that holds nothing but
// formulas: some apps copy a display formula as one. end is the index
// after the closing fence.
func fencedFormula(lines []string, start int, fence string) (formula string, end int, ok bool) {
	if strings.TrimSpace(lines[start]) != fence {
		return "", 0, false
	}
	closing := -1
	for j := start + 1; j < len(lines); j++ {
		if closesFence(lines[j], fence) {
			closing = j
			break
		}
	}
	if closing < 0 || closing == start+1 {
		return "", 0, false
	}
	body := lines[start+1 : closing]
	var parts []string
	for i := 0; i < len(body); {
		part, next, ok := formulaAt(body, i)
		if !ok {
			return "", 0, false
		}
		parts = append(parts, part)
		i = next
	}
	return strings.Join(parts, " "), closing + 1, true
}

// formulaAt reads the formula starting at lines[i]: one line, or an
// environment running to its \end. end is the index after it.
func formulaAt(lines []string, i int) (formula string, end int, ok bool) {
	line := stripInlineTags(lines[i])
	if !formulaLine(line) {
		return "", 0, false
	}
	open := beginEnvironment.FindStringSubmatch(line)
	if open == nil || strings.Contains(line, `\end{`+open[1]+`}`) {
		return line, i + 1, true
	}
	parts := []string{line}
	for j := i + 1; j < len(lines) && j < i+maxEnvironmentLines; j++ {
		raw := lines[j]
		if openingFence(raw) != "" {
			break
		}
		part := stripInlineTags(raw)
		if part == "" {
			if strings.TrimSpace(raw) == "" {
				break // A formula does not cross a blank line.
			}
			continue // Only tags: a client's bold around a line break.
		}
		if !mathOnly(part) {
			break
		}
		parts = append(parts, part)
		if strings.Contains(part, `\end{`+open[1]+`}`) {
			return strings.Join(parts, " "), j + 1, true
		}
	}
	// An unclosed environment is still a formula line of its own.
	return line, i + 1, true
}

const maxEnvironmentLines = 40

var (
	beginEnvironment = regexp.MustCompile(`\\begin\{([a-zA-Z*]+)\}`)
	latexCommand     = regexp.MustCompile(`\\[a-zA-Z]+`)
	inlineTag        = regexp.MustCompile(`</?(?:b|i|u|s|em|strong)>`)
	innermostBraces  = regexp.MustCompile(`\{[^{}]*\}`)
	blockMarker      = regexp.MustCompile(`^(?:#{1,6}\s|>|\||[-*+]\s|\d+[.)]\s|\[[ xX^])`)
)

func stripInlineTags(line string) string {
	return strings.TrimSpace(inlineTag.ReplaceAllString(line, ""))
}

// formulaLine reports whether a trimmed line is bare LaTeX on its own.
func formulaLine(line string) bool {
	if line == "" || strings.Contains(line, "`") || blockMarker.MatchString(line) {
		return false
	}
	// A command alone is no signal: C:\Users\name has two. It needs math next
	// to it, as do scripts, which also appear in identifiers.
	commanded := latexCommand.MatchString(line) && strings.ContainsAny(line, "=<>^_{}&")
	scripted := strings.ContainsAny(line, "^_") && strings.ContainsAny(line, "=<>")
	if !commanded && !scripted {
		return false
	}
	return mathOnly(line)
}

// mathOnly reports whether no word of three or more letters is left once
// LaTeX commands and braced arguments are removed.
func mathOnly(line string) bool {
	rest := latexCommand.ReplaceAllString(line, " ")
	for {
		stripped := innermostBraces.ReplaceAllString(rest, " ")
		if stripped == rest {
			break
		}
		rest = stripped
	}
	run := 0
	for _, r := range rest {
		if unicode.IsLetter(r) {
			run++
			if run >= 3 {
				return false
			}
			continue
		}
		run = 0
	}
	return true
}
