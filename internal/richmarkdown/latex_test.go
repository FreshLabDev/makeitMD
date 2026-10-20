// SPDX-License-Identifier: Apache-2.0
package richmarkdown

import "testing"

func TestConvertLaTeXDelimiters(t *testing.T) {
	for _, tc := range []struct{ name, source, want string }{
		{"inline", `где \(PV\) это капитал`, `где $PV$ это капитал`},
		{"inline with commands", `\(7^{-\frac{2}{3}}\), \(125^{\frac{1}{9}}\)`, `$7^{-\frac{2}{3}}$, $125^{\frac{1}{9}}$`},
		{"display on its own line", "Формула:\n\n\\[ FV=PV(1+r)^n \\]\n\nГде", "Формула:\n\n$$FV=PV(1+r)^n$$\n\nГде"},
		{"display spans lines", "\\[\nA = B \\\\\nC = D\n\\]", `$$A = B \\ C = D$$`},
		{"thin space and newline inside", `\[ I_1=1\,000\,000 \]`, `$$I_1=1\,000\,000$$`},
		{"latex row break before closing", `\[ \begin{cases} a\\ b \end{cases} \]`, `$$\begin{cases} a\\ b \end{cases}$$`},
		{"inside a list item", "- \\(r\\) это ставка\n  - чем больше \\(n\\)", "- $r$ это ставка\n  - чем больше $n$"},
		{"escaped backslash is no delimiter", `путь C:\\(x86\\)`, `путь C:\\(x86\\)`},
		{"escaped reference keeps brackets", `см. \[1\] и \[2\]`, `см. \[1\] и \[2\]`},
		{"inline display with latex", `сумма \[x^2\] тут`, `сумма $$x^2$$ тут`},
		{"unclosed stays", `открыто \(x и всё`, `открыто \(x и всё`},
		{"no crossing a blank line", "\\(x\n\ny\\)", "\\(x\n\ny\\)"},
		{"empty stays", `\(\)`, `\(\)`},
		{"dollar inside stays", `\(a$b\)`, `\(a$b\)`},
		{"inline code is literal", "пиши `\\(x\\)` или \\(y\\)", "пиши `\\(x\\)` или $y$"},
		{"double backtick span", "``a ` \\(x\\)`` и \\(y\\)", "``a ` \\(x\\)`` и $y$"},
		{"fenced code is literal", "```latex\n\\(x\\)\n```\n\\(y\\)", "```latex\n\\(x\\)\n```\n$y$"},
		{"tilde fence is literal", "~~~\n\\[x^2\\]\n~~~\n", "~~~\n\\[x^2\\]\n~~~\n"},
		{"html code is literal", `<code>\(x\)</code> \(y\)`, `<code>\(x\)</code> $y$`},
		{"html pre is literal", "<pre>\n\\[x^2\\]\n</pre>", "<pre>\n\\[x^2\\]\n</pre>"},
		{"other tags are text", `<b>\(x\)</b>`, `<b>$x$</b>`},
		{"dollars untouched", `$x$ и $$y$$`, `$x$ и $$y$$`},
		{"nothing to do", "# plain\n\ntext", "# plain\n\ntext"},
		// ChatGPT's copy button escapes its Markdown once more.
		{"doubled display", `\\[ I_1=1\\,000\\,000\cdot\frac{0{,}05}{12} \\]`, `$$I_1=1\,000\,000\cdot\frac{0{,}05}{12}$$`},
		{"doubled unescapes underscores", `\\[ P=\sum\_{k=1}^{n}\frac{A}{(1+r)^k} \\]`, `$$P=\sum_{k=1}^{n}\frac{A}{(1+r)^k}$$`},
		{"doubled row break", `\\[ \begin{cases} A_k=I_k+C_k\\\ I_k=rB\_{k-1} \end{cases} \\]`, `$$\begin{cases} A_k=I_k+C_k\\ I_k=rB_{k-1} \end{cases}$$`},
		{"doubled inline beside a formula", "\\\\[ FV=PV(1+r)^n \\\\]\n\n- \\\\(PV\\\\) это капитал", "$$FV=PV(1+r)^n$$\n\n- $PV$ это капитал"},
		{"doubled code is literal", "`\\\\(x^2\\\\)` и \\\\(y^2\\\\)", "`\\\\(x^2\\\\)` и $y^2$"},
		{"doubled without latex stays", `C:\\(x86\\) и \\[1\\]`, `C:\\(x86\\) и \\[1\\]`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ConvertLaTeXDelimiters(tc.source); got != tc.want {
				t.Fatalf("\ngot  %q\nwant %q", got, tc.want)
			}
		})
	}
}
