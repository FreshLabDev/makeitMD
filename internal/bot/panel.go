// SPDX-License-Identifier: Apache-2.0
package bot

import (
	"html"
	"strings"

	"github.com/FreshLabDev/tg"

	"github.com/FreshLabDev/makeitMD/internal/build"
	"github.com/FreshLabDev/makeitMD/internal/i18n"
)

// The panel is deliberately two screens wide and one screen deep. makeitMD has
// nothing to configure, so there is no settings screen and no state a button
// could change: "How it works" is what a first-time user needs, "About" is what
// an operator needs, and everything else the bot does is done by pasting text.
const (
	panelRoot  = "panel:root"
	panelHow   = "panel:how"
	panelAbout = "panel:about"
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
