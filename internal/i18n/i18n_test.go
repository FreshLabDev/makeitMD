// SPDX-License-Identifier: Apache-2.0
package i18n

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// English is the fallback every other language leans on, so a key without it
// renders as "[key]" to whoever is reading the panel.
func TestEveryKeyHasEnglish(t *testing.T) {
	if len(Keys()) == 0 {
		t.Fatal("no strings are embedded at all")
	}
	for _, key := range Keys() {
		if strings.TrimSpace(translations[key][DefaultLang]) == "" {
			t.Errorf("%s has no English text", key)
		}
	}
}

// Locales arrive one at a time, so the catalogue is checked for stray language
// codes rather than for completeness: a "ua" or "en-GB" column would be
// translated work that no picker can ever select.
func TestOnlyOfferedLanguagesAppear(t *testing.T) {
	for _, key := range Keys() {
		for code := range translations[key] {
			if !IsSupported(code) {
				t.Errorf("%s carries %q, which is not a language this bot offers", key, code)
			}
		}
	}
}

// Key names are the contract with whoever adds the remaining languages: they
// are lowercase, dotted and grouped by screen, and they do not move once a
// translation is keyed off them.
func TestKeyNamesAreHierarchical(t *testing.T) {
	for _, key := range Keys() {
		area, name, ok := strings.Cut(key, ".")
		if !ok || area == "" || name == "" {
			t.Errorf("%q is not <area>.<name>", key)
			continue
		}
		if key != strings.ToLower(key) || strings.ContainsAny(key, " -") {
			t.Errorf("%q is not lowercase snake_case within its dots", key)
		}
	}
}

// A placeholder that survives into one language but not another renders a
// literal "{bot}" to that user.
func TestPlaceholdersMatchEnglish(t *testing.T) {
	for _, key := range Keys() {
		want := placeholders(translations[key][DefaultLang])
		for code, text := range translations[key] {
			if got := placeholders(text); got != want {
				t.Errorf("%s [%s] has placeholders %q, English has %q", key, code, got, want)
			}
		}
	}
}

func placeholders(text string) string {
	var found []string
	for {
		open := strings.IndexByte(text, '{')
		if open < 0 {
			break
		}
		end := strings.IndexByte(text[open:], '}')
		if end < 0 {
			break
		}
		found = append(found, text[open:open+end+1])
		text = text[open+end+1:]
	}
	// Order does not matter across languages, only the set does.
	for i := 0; i < len(found); i++ {
		for j := i + 1; j < len(found); j++ {
			if found[j] < found[i] {
				found[i], found[j] = found[j], found[i]
			}
		}
	}
	return strings.Join(found, ",")
}

// Until a language is translated it must read as English rather than as an
// empty message or a "[key]" marker.
func TestMissingLanguageFallsBackToEnglish(t *testing.T) {
	for _, key := range Keys() {
		for _, code := range Codes() {
			if got := T(code, key); got == "" || got == "["+key+"]" {
				t.Fatalf("T(%q, %q) = %q", code, key, got)
			}
		}
	}
}

func TestResolveFallsBackToEnglish(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"ru", "ru"},
		{"ru-RU", "ru"},
		{"uk", "uk"},
		{"be", "be"},
		{"zh_CN", "zh"},
		{"EN-gb", "en"},
		{"", "en"},
		{"kl", "en"},
		{"klingon", "en"},
	} {
		if got := Resolve(tc.in); got != tc.want {
			t.Errorf("Resolve(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestInterpolation(t *testing.T) {
	if got, want := T("en", "btn.open", "bot", "makeitMD"), "Open makeitMD"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	// An unpaired trailing argument is ignored rather than half-applied.
	if got, want := T("en", "btn.open", "bot"), "Open {bot}"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if got := T("en", "no.such.key"); got != "[no.such.key]" {
		t.Fatalf("unknown key = %q", got)
	}
}

// Every option in the picker must be renderable, and every renderable language
// must be offered.
func TestPickerMatchesSupportedSet(t *testing.T) {
	want := []string{"en", "ru", "uk", "es", "fr", "de", "it", "pl", "cs", "tr", "sv", "be", "ca", "zh", "ja", "ar"}
	if got := strings.Join(Codes(), " "); got != strings.Join(want, " ") {
		t.Fatalf("the family's language order is fixed:\ngot  %s\nwant %s", got, strings.Join(want, " "))
	}
	for _, o := range LANGUAGE_OPTIONS {
		if !IsSupported(o.Code) {
			t.Errorf("%s is offered but not supported", o.Code)
		}
		if o.Label == "" || o.Label == o.Code || LabelOf(o.Code) != o.Label {
			t.Errorf("%s has no native label", o.Code)
		}
	}
}

// A key nothing renders is fifteen translations nobody reads. Every key has to
// appear literally in the Go source outside this package, which is also what
// keeps the names stable.
func TestEveryKeyIsUsed(t *testing.T) {
	sources := readGoSources(t, "../..")
	for _, key := range Keys() {
		if !strings.Contains(sources, `"`+key+`"`) {
			t.Errorf("%s is translated but never rendered", key)
		}
	}
}

func readGoSources(t *testing.T, root string) string {
	t.Helper()
	var all strings.Builder
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			// This package is where the keys are defined and where the tests
			// quote a few of them; only their use elsewhere counts.
			if entry.Name() == "i18n" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		all.Write(content)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return all.String()
}
