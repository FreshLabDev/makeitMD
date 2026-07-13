// SPDX-License-Identifier: Apache-2.0
package richmarkdown

import (
	"testing"

	"github.com/FreshLabDev/makeitMD/internal/telegram"
)

func TestRestoreEntitiesUsesUTF16Offsets(t *testing.T) {
	text := "😀 hello world"
	entities := []telegram.MessageEntity{{Type: "bold", Offset: 3, Length: 5}}
	if got, want := RestoreEntities(text, entities), "😀 <b>hello</b> world"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRestoreEntitiesKeepsNestedFormatting(t *testing.T) {
	text := "bold italic"
	entities := []telegram.MessageEntity{
		{Type: "bold", Offset: 0, Length: 11},
		{Type: "italic", Offset: 5, Length: 6},
	}
	if got, want := RestoreEntities(text, entities), "<b>bold <i>italic</i></b>"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRestoreEntitiesSupportsLinksCodeAndQuotes(t *testing.T) {
	text := "site and x\nsecond"
	entities := []telegram.MessageEntity{
		{Type: "text_link", Offset: 0, Length: 4, URL: "https://example.com?a=1&b=2"},
		{Type: "code", Offset: 9, Length: 1},
		{Type: "blockquote", Offset: 11, Length: 6},
	}
	want := `<a href="https://example.com?a=1&amp;b=2">site</a> and ` + "`x`\n> second"
	if got := RestoreEntities(text, entities); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRestoreEntitiesIgnoresLexicalEntities(t *testing.T) {
	text := "https://example.com #tag"
	entities := []telegram.MessageEntity{{Type: "url", Offset: 0, Length: 19}, {Type: "hashtag", Offset: 20, Length: 4}}
	if got := RestoreEntities(text, entities); got != text {
		t.Fatalf("got %q", got)
	}
}
