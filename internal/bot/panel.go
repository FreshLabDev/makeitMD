// SPDX-License-Identifier: Apache-2.0
package bot

import (
	"html"
	"strings"

	"github.com/FreshLabDev/tg"

	"github.com/FreshLabDev/makeitMD/internal/build"
	"github.com/FreshLabDev/makeitMD/internal/i18n"
)

// The panel is three screens wide and one screen deep. makeitMD still has
// nothing to configure: "How it works" is what a first-time user needs, "About"
// is what an operator needs, and everything else the bot does is done by
// pasting text. The one thing a button here changes is the language, and that
// is not this bot's state either -- it lives in the hub the whole family
// shares, so choosing it once chooses it everywhere.
const (
	panelRoot  = "panel:root"
	panelHow   = "panel:how"
	panelAbout = "panel:about"
	panelLang  = "panel:lang"

	// panelLangPick prefixes one language button. panelLangFollow withdraws the
	// choice instead of making one; "follow" is not a language code, so the two
	// can never collide.
	panelLangPick   = "panel:lang:"
	panelLangFollow = "panel:lang:follow"
)

// Every option of a set carries a glyph, chosen or not, so the column has one
// left edge and "not chosen" reads as a state rather than as an absence.
const (
	markChosen    = "◉ "
	markNotChosen = "◎ "
)

// Facts the panel states about itself. They read the same in every language, so
// they are constants rather than translation keys: a translator asked for the
// Japanese of "@amtiyo" has been asked the wrong question.
const (
	productName = "makeitMD"
	repoURL     = "https://github.com/FreshLabDev/makeitMD"
	repoName    = "FreshLabDev/makeitMD"
	licenseName = "Apache-2.0"
	adminURL    = "https://t.me/amtiyo"
	adminName   = "@amtiyo"
)

// screen is one panel view: what the message says and what it offers next.
type screen struct {
	text   string
	markup *tg.InlineKeyboardMarkup
}

// panelText is the shape every screen in this bot is built from: a bold title,
// an italic line saying what the screen is for, and the substance in a quote.
// It exists once so that no screen can quietly grow a shape of its own -- the
// root screen used to open with bare prose while every other panel in the
// family opened with a title, which made this bot look like a different
// product to anyone who used two of them.
func panelText(title, hint, body string) string {
	text := "<b>" + title + "</b>"
	if hint != "" {
		text += "\n<i>" + hint + "</i>"
	}
	if body != "" {
		text += "\n\n" + quote(body)
	}
	return text
}

func quote(body string) string { return "<blockquote>" + body + "</blockquote>" }

// rootScreen is what /start opens.
func rootScreen(lang string) screen {
	return screen{
		text: panelText(productName, i18n.T(lang, "home.hint"), i18n.T(lang, "home.body")),
		markup: &tg.InlineKeyboardMarkup{InlineKeyboard: [][]tg.InlineKeyboardButton{{
			// The only thing a first-time user has to read is how to use the
			// bot, so that button is the primary one. About is reference
			// material and stays unstyled: colouring both would single out
			// neither. Nothing here destroys anything, so no button is danger.
			{Text: i18n.T(lang, "btn.how"), CallbackData: panelHow, Style: tg.StylePrimary},
			{Text: i18n.T(lang, "btn.about"), CallbackData: panelAbout},
		}, {
			// The language is a correction, not a first step: it is already
			// resolved from the shared hub or from the Telegram client before
			// anybody taps anything. So it sits below the two tabs and takes no
			// colour -- a second primary would single out neither.
			{Text: i18n.T(lang, "btn.language"), CallbackData: panelLang},
		}}},
	}
}

// howScreen keeps the note about what makeitMD does not do next to the list of
// what it renders: both answer the same question, and a person who wonders why
// their paste arrived as one message is reading this screen.
func howScreen(lang string) screen {
	body := i18n.T(lang, "how.body") + "\n\n" +
		i18n.T(lang, "how.engine", "bot", productName) + " " + i18n.T(lang, "how.split")
	return screen{
		text:   panelText(i18n.T(lang, "how.title"), i18n.T(lang, "how.hint"), body),
		markup: backKeyboard(lang),
	}
}

// aboutScreen is the family's standard card, and the one screen that does not
// take the title-hint-quote shape: the first line is the name and the running
// version, which every bot in the family states the same way. The repository is
// a link in the text rather than a button -- a button would be a second way to
// do the same thing, on a screen whose only action is going back.
func aboutScreen(lang string, info build.Info) screen {
	head := "<b>" + productName + "</b> · <i>" + html.EscapeString(buildLabel(info)) + "</i>\n" +
		i18n.T(lang, "about.tagline")
	rows := []string{
		i18n.T(lang, "about.rendering") + " · Telegram Bot API " + tg.BotAPI + " rich messages",
		i18n.T(lang, "about.source") + " · " + link(repoURL, repoName) + " · " + licenseName,
		i18n.T(lang, "about.admin") + " · " + link(adminURL, adminName),
	}
	return screen{text: head + "\n\n" + quote(strings.Join(rows, "\n")), markup: backKeyboard(lang)}
}

func link(url, label string) string {
	return "<a href=\"" + url + "\">" + html.EscapeString(label) + "</a>"
}

// groupScreen redirects a group /start into a direct message. The deep link is
// the one action of that screen, so it is the primary button; without a known
// username there is no link to offer and the sentence has to carry it alone.
// It carries no Back and no Close: this is not a panel somebody navigates but a
// one-line answer to a command, and it is already invisible to the group.
func groupScreen(lang, username string) screen {
	view := screen{text: panelText(productName, i18n.T(lang, "home.hint"),
		i18n.T(lang, "home.group", "bot", productName))}
	if username != "" {
		view.markup = &tg.InlineKeyboardMarkup{InlineKeyboard: [][]tg.InlineKeyboardButton{{
			{Text: i18n.T(lang, "btn.open", "bot", productName),
				URL: "https://t.me/" + username, Style: tg.StylePrimary},
		}}}
	}
	return view
}

// languageScreen is the family's picker: the same sixteen languages in the same
// order with the same labels every sibling bot offers, two per row. The choice
// is stored in the shared hub, so what this screen sets is not this bot's
// language but the person's.
func languageScreen(lang string) screen {
	options := i18n.LANGUAGE_OPTIONS
	rows := make([][]tg.InlineKeyboardButton, 0, len(options)/2+2)
	for i := 0; i < len(options); i += 2 {
		row := []tg.InlineKeyboardButton{languageButton(options[i], lang)}
		if i+1 < len(options) {
			row = append(row, languageButton(options[i+1], lang))
		}
		rows = append(rows, row)
	}
	// Handing the decision back to Telegram is a different thing from making
	// one, so it sits under the grid, in no colour, where it cannot be mistaken
	// for a seventeenth language.
	rows = append(rows, []tg.InlineKeyboardButton{
		{Text: i18n.T(lang, "btn.follow_telegram"), CallbackData: panelLangFollow},
	})
	rows = append(rows, backRow(lang))
	// The body says nothing the buttons already say. Which language is current
	// lives on the buttons; repeating it as a line of text would be one state
	// kept in two places, and they would disagree the first time one changed.
	return screen{
		text:   panelText(i18n.T(lang, "lang.title"), i18n.T(lang, "lang.hint"), ""),
		markup: &tg.InlineKeyboardMarkup{InlineKeyboard: rows},
	}
}

// languageButton paints the current language and nothing else: Success means
// "this is the state you are in", never "this button acts".
func languageButton(option i18n.LangOption, lang string) tg.InlineKeyboardButton {
	button := tg.InlineKeyboardButton{
		Text:         markNotChosen + option.Label,
		CallbackData: panelLangPick + option.Code,
	}
	if option.Code == lang {
		button.Text = markChosen + option.Label
		button.Style = tg.StyleSuccess
	}
	return button
}

// languageChoice reads a tap on the language screen. An empty code with ok is
// "follow Telegram", which is the choice to stop having one. A code this
// release does not offer is not a choice at all: it came from a keyboard an
// older release sent, and lands on the root panel like any other stale button.
func languageChoice(data string) (code string, ok bool) {
	if data == panelLangFollow {
		return "", true
	}
	if !strings.HasPrefix(data, panelLangPick) {
		return "", false
	}
	code = strings.TrimPrefix(data, panelLangPick)
	if !i18n.IsSupported(code) {
		return "", false
	}
	return code, true
}

// backKeyboard is the whole navigation vocabulary of this bot. There is no
// Close button: in a private chat the conversation is the panel, so there is
// nothing covering anything to close.
func backKeyboard(lang string) *tg.InlineKeyboardMarkup {
	return &tg.InlineKeyboardMarkup{InlineKeyboard: [][]tg.InlineKeyboardButton{backRow(lang)}}
}

// backRow is one word, whatever screen it sits on and however deep that screen
// is. "Menu", "Home" and per-screen variants are what this bot does not say.
func backRow(lang string) []tg.InlineKeyboardButton {
	return []tg.InlineKeyboardButton{{Text: i18n.T(lang, "btn.back"), CallbackData: panelRoot}}
}

// screenFor maps callback data to a view. Unknown data belongs to a keyboard an
// older release sent, so it lands on the root panel: a stale button that still
// opens something beats one that does nothing.
func (b *Bot) screenFor(lang, data string) screen {
	switch data {
	case panelHow:
		return howScreen(lang)
	case panelAbout:
		return aboutScreen(lang, b.build)
	case panelLang:
		return languageScreen(lang)
	default:
		return rootScreen(lang)
	}
}

// buildLabel is the version line of the About card, and the version is all of
// it. Every bot in the family states its version the same way, and a commit
// hash appended here would make this one card read differently from the rest
// while answering a question its readers are not asking. The commit is still
// on the startup log line and in /health, where whoever needs it is looking.
func buildLabel(info build.Info) string {
	version := strings.TrimSpace(info.Version)
	if version == "" {
		return "dev"
	}
	return version
}
