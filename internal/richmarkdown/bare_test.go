// SPDX-License-Identifier: Apache-2.0
package richmarkdown

import "testing"

// Lines below are what chatbot apps put on the clipboard on a phone.
func TestWrapBareFormulas(t *testing.T) {
	for _, tc := range []struct{ name, source, want string }{
		{"display line", "Его формула:\nA=P\\cdot\\frac{r(1+r)^n}{(1+r)^n-1}\nЗдесь", "Его формула:\n$$A=P\\cdot\\frac{r(1+r)^n}{(1+r)^n-1}$$\nЗдесь"},
		{"scripts with a relation", "FV=PV(1+r)^n\nГде:", "$$FV=PV(1+r)^n$$\nГде:"},
		{"subscript word in braces", `S_{total} = \sum_{k=1}^{n} P_k`, `$$S_{total} = \sum_{k=1}^{n} P_k$$`},
		{"broken formula joined", "I_1=1\\,000\\,000\\cdot\\frac{0{,}05}{12}\n\\approx 4166{,}67\nИз 5368 zł", "$$I_1=1\\,000\\,000\\cdot\\frac{0{,}05}{12} \\approx 4166{,}67$$\nИз 5368 zł"},
		{"environment over lines", "\\begin{cases}\nA_k=I_k+C_k\\\\\nI_k=rB_{k-1}\\\\\nB_k=B_{k-1}-C_k\n\\end{cases}\nИли", "$$\\begin{cases} A_k=I_k+C_k\\\\ I_k=rB_{k-1}\\\\ B_k=B_{k-1}-C_k \\end{cases}$$\nИли"},
		{"bold around matrix parts", "<b>\\begin{pmatrix}</b>\n<b>\n</b>\n<b>B_k\\\\</b>\n<b>1</b>\n<b>\\end{pmatrix}</b>\n\\begin{pmatrix}\n0 & 1\n\\end{pmatrix}\nПо сути", "$$\\begin{pmatrix} B_k\\\\ 1 \\end{pmatrix} \\begin{pmatrix} 0 & 1 \\end{pmatrix}$$\nПо сути"},
		{"text cyrillic inside", `r = \dfrac{\text{годовая ставка}}{12}`, `$$r = \dfrac{\text{годовая ставка}}{12}$$`},
		{"blank line separates", "a^2=b\n\nc^2=d", "$$a^2=b$$\n\n$$c^2=d$$"},
		{"prose with inline latex stays", `при арифметике 5 * 3 = 15, но 10 \text{ млн} \rightarrow 37`, `при арифметике 5 * 3 = 15, но 10 \text{ млн} \rightarrow 37`},
		{"prose with lambda stays", `Собственное значение \lambda = 1 + i > 1, что`, `Собственное значение \lambda = 1 + i > 1, что`},
		{"code line stays", "r = annual_rate / 100 / 12", "r = annual_rate / 100 / 12"},
		{"python call stays", "payment, interest = mortgage(1_000_000, 5, 30)", "payment, interest = mortgage(1_000_000, 5, 30)"},
		{"formula as a plain code block", "при ставке r:\n```\nFV = PV \\cdot (1 + r)^t\n```\nВ кредите", "при ставке r:\n$$FV = PV \\cdot (1 + r)^t$$\nВ кредите"},
		{"terminal output block stays", "```\nПлатёж в месяц: 102,861.13 ₽\n```", "```\nПлатёж в месяц: 102,861.13 ₽\n```"},
		{"plain code block stays", "```\nx = compute(a_b)\n```", "```\nx = compute(a_b)\n```"},
		{"fenced code stays", "```python\nx_1 = y^2\n```\nx_1 = y^2", "```python\nx_1 = y^2\n```\n$$x_1 = y^2$$"},
		{"footnote stays", "[^1]: x^2=y", "[^1]: x^2=y"},
		{"list item stays", "- a_1 = b", "- a_1 = b"},
		{"heading stays", "# x^2 = y", "# x^2 = y"},
		{"windows path stays", `C:\Users\amti`, `C:\Users\amti`},
		{"path with a digit stays", `D:\Games\x86 = old`, `D:\Games\x86 = old`},
		{"snake case stays", "monthly_rate", "monthly_rate"},
		{"delimited source untouched", "$x$\nA=P\\cdot r", "$x$\nA=P\\cdot r"},
		{"unclosed environment is one line", "\\begin{cases}\nтекст тут", "$$\\begin{cases}$$\nтекст тут"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := WrapBareFormulas(tc.source); got != tc.want {
				t.Fatalf("\ngot  %q\nwant %q", got, tc.want)
			}
		})
	}
}
