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
	for _, want := range []string{"# agents", "**One config.**", "Practical standard.", "[npm](https://npm.example)"} {
		if !strings.Contains(got, want) {
			t.Fatalf("normalized output missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "<img") || strings.Contains(got, "<p") {
		t.Fatalf("unsupported layout HTML remains:\n%s", got)
	}
}
