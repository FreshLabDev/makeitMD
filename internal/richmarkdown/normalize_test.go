// SPDX-License-Identifier: Apache-2.0
package richmarkdown

import (
	"strings"
	"testing"
)

func TestNormalizeGitHubHeader(t *testing.T) {
	source := `<h1 align="center">agents</h1>
<p align="center"><strong>One config.</strong><br/>Practical standard.</p>
<p align="center"><a href="https://npm.example"><img src="https://img.example" alt="npm"></a></p>`
	got := NormalizeFallback(source)
	for _, want := range []string{"# agents", "<strong>One config.</strong>", "Practical standard.", "[npm](https://npm.example)"} {
		if !strings.Contains(got, want) {
			t.Fatalf("normalized output missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "<img") || strings.Contains(got, "<p") {
		t.Fatalf("unsupported layout HTML remains:\n%s", got)
	}
}

func TestNormalizePreservesInlineHTMLInsideTable(t *testing.T) {
	source := `<table><tr><td><strong>Codex</strong></td><td><em>ready</em></td></tr></table>`
	if got := NormalizeFallback(source); got != source {
		t.Fatalf("got %q want %q", got, source)
	}
}

func TestNormalizeLinkedMarkdownBadge(t *testing.T) {
	source := `[![GitHub Stars](https://img.shields.io/badge/stars-blue?logo=data:image/svg%2bxml;base64,AAA)](https://github.com/example/stargazers)`
	want := `[GitHub Stars](https://github.com/example/stargazers)`
	if got := NormalizeFallback(source); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
