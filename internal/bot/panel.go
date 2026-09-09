// SPDX-License-Identifier: Apache-2.0
package bot

import (
	"html"
	"strings"

	"github.com/FreshLabDev/tg"

	"github.com/FreshLabDev/makeitMD/internal/build"
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

const (
	startText = "Send me Markdown. I’ll render it."

	howText = "<b>How it works</b>\n" +
		"Send Markdown in this chat and Telegram renders it natively: headings, " +
		"nested styles, lists, task lists, tables, quotes, code blocks, details, " +
		"links and formulas.\n\n" +
		"<blockquote>makeitMD parses nothing itself — your source goes to Telegram " +
		"and Telegram renders it. A paste your client splits into several messages " +
		"is stitched back together first.</blockquote>"

	// groupStartText answers /start in a group, where the bot cannot do its job.
	groupStartText = "makeitMD renders Markdown in a direct message. Open it there and paste your source."
)

// screen is one panel view: what the message says and what it offers next.
type screen struct {
	text   string
	markup *tg.InlineKeyboardMarkup
}

// rootScreen is what /start opens.
func rootScreen() screen {
	return screen{
		text: startText,
		markup: &tg.InlineKeyboardMarkup{InlineKeyboard: [][]tg.InlineKeyboardButton{{
			// The only thing a first-time user has to read is how to use the
			// bot, so that button is the primary one. About is reference
			// material and stays unstyled: colouring both would single out
			// neither. Nothing here destroys anything, so no button is danger.
			{Text: "How it works", CallbackData: panelHow, Style: tg.StylePrimary},
			{Text: "About", CallbackData: panelAbout},
		}}},
	}
}

func howScreen() screen {
	return screen{text: howText, markup: backKeyboard()}
}

// aboutScreen is the family's standard card: name and build on the first line,
// one sentence of what the bot is, then key-value rows in a quote. The
// repository is a link in the text rather than a button -- a button would be a
// second way to do the same thing, on a screen whose only action is going back.
func aboutScreen(info build.Info) screen {
	text := "<b>makeitMD</b> · <i>" + html.EscapeString(buildLabel(info)) + "</i>\n" +
		"Send Markdown. Get native Telegram rich text.\n\n" +
		"<blockquote>Rendering · Telegram Bot API " + tg.BotAPI + " rich messages\n" +
		"Source · <a href=\"https://github.com/FreshLabDev/makeitMD\">FreshLabDev/makeitMD</a> · Apache-2.0\n" +
		"Admin · <a href=\"https://t.me/amtiyo\">@amtiyo</a></blockquote>"
	return screen{text: text, markup: backKeyboard()}
}

// groupScreen redirects a group /start into a direct message. The deep link is
// the one action of that screen, so it is the primary button; without a known
// username there is no link to offer and the sentence has to carry it alone.
func groupScreen(username string) screen {
	view := screen{text: groupStartText}
	if username != "" {
		view.markup = &tg.InlineKeyboardMarkup{InlineKeyboard: [][]tg.InlineKeyboardButton{{
			{Text: "Open makeitMD", URL: "https://t.me/" + username, Style: tg.StylePrimary},
		}}}
	}
	return view
}

// backKeyboard is the whole navigation vocabulary of this bot. There is no
// Close button: in a private chat the conversation is the panel, so there is
// nothing covering anything to close.
func backKeyboard() *tg.InlineKeyboardMarkup {
	return &tg.InlineKeyboardMarkup{InlineKeyboard: [][]tg.InlineKeyboardButton{{
		{Text: "Back", CallbackData: panelRoot},
	}}}
}

// screenFor maps callback data to a view. Unknown data belongs to a keyboard an
// older release sent, so it lands on the root panel: a stale button that still
// opens something beats one that does nothing.
func (b *Bot) screenFor(data string) screen {
	switch data {
	case panelHow:
		return howScreen()
	case panelAbout:
		return aboutScreen(b.build)
	default:
		return rootScreen()
	}
}

// buildLabel is the version line of the About card. The commit is what tells
// two builds of one version apart, which is the question anyone reading this
// line is actually asking. The placeholders a bare `go build` leaves behind
// answer nothing, so they are dropped rather than shown as "none".
func buildLabel(info build.Info) string {
	version := strings.TrimSpace(info.Version)
	if version == "" {
		version = "dev"
	}
	commit := strings.TrimSpace(info.Commit)
	if commit == "" || commit == "none" {
		return version
	}
	if len(commit) > 7 {
		commit = commit[:7]
	}
	return version + " · " + commit
}
