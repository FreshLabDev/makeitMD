// SPDX-License-Identifier: Apache-2.0
package bot

import (
	"context"
	"strings"
	"testing"

	"github.com/FreshLabDev/tg"

	"github.com/FreshLabDev/makeitMD/internal/build"
	"github.com/FreshLabDev/makeitMD/internal/i18n"
)

func TestStartOpensThePanel(t *testing.T) {
	client := &fakeTelegram{}
	store := &fakeStore{}
	if err := newTestBot(client, store).handle(context.Background(), testPaste(testUpdate("/start"))); err != nil {
		t.Fatal(err)
	}
	if len(client.messages) != 1 {
		t.Fatalf("messages=%+v", client.messages)
	}
	buttons := flatten(client.messages[0].markup)
	if len(buttons) != 2 {
		t.Fatalf("the panel is exactly two tabs, got %+v", buttons)
	}
	if buttons[0].Text != i18n.T("en", "btn.how") || buttons[0].Style != tg.StylePrimary {
		t.Fatalf("the screen's main action must be primary: %+v", buttons[0])
	}
	if buttons[1].Text != i18n.T("en", "btn.about") || buttons[1].Style != "" {
		t.Fatalf("only the main action is styled: %+v", buttons[1])
	}
}

// Every panel in the family opens with a bold title and an italic line saying
// what the screen is for. This one used to open with bare prose, which made
// makeitMD read as a different product from the bot next to it.
func TestEveryScreenOpensWithATitleAndAHint(t *testing.T) {
	for name, view := range map[string]screen{
		"root":  rootScreen("en"),
		"how":   howScreen("en"),
		"about": aboutScreen("en", build.Info{Version: "v9.9.9"}),
		"group": groupScreen("en", "makeitMD_bot"),
	} {
		if !strings.HasPrefix(view.text, "<b>") {
			t.Errorf("%s does not open with a title: %q", name, view.text)
		}
		// The About card states the version where the others state a hint, and
		// is the one screen allowed to.
		if name != "about" && !strings.Contains(view.text, "</b>\n<i>") {
			t.Errorf("%s has no one-line hint under its title: %q", name, view.text)
		}
		if !strings.Contains(view.text, "<blockquote>") {
			t.Errorf("%s says its substance outside a quote: %q", name, view.text)
		}
		// A string the catalogue is missing renders as its own key.
		if strings.Contains(view.text, "[") && strings.Contains(view.text, "]") {
			t.Errorf("%s carries an unrendered key: %q", name, view.text)
		}
	}
}

func TestAboutCardCarriesTheRunningBuild(t *testing.T) {
	client := &fakeTelegram{}
	bot := newTestBot(client, &fakeStore{})
	if err := bot.handleCallback(context.Background(), testCallback(panelAbout)); err != nil {
		t.Fatal(err)
	}
	if len(client.answered) != 1 || len(client.edits) != 1 {
		t.Fatalf("answered=%v edits=%d", client.answered, len(client.edits))
	}
	card := client.edits[0].text
	// The version comes from the same build info /healthz reports, so the card
	// cannot claim a build that is not running.
	for _, want := range []string{
		"<b>makeitMD</b> · <i>v9.9.9</i>",
		i18n.T("en", "about.tagline"),
		"Rendering · Telegram Bot API " + tg.BotAPI,
		`Source · <a href="https://github.com/FreshLabDev/makeitMD">FreshLabDev/makeitMD</a> · Apache-2.0`,
		`Admin · <a href="https://t.me/amtiyo">@amtiyo</a>`,
	} {
		if !strings.Contains(card, want) {
			t.Fatalf("about card is missing %q:\n%s", want, card)
		}
	}
	buttons := flatten(client.edits[0].markup)
	if len(buttons) != 1 || buttons[0].Text != i18n.T("en", "btn.back") {
		t.Fatalf("about offers Back and nothing else: %+v", buttons)
	}
	// The repository is a link in the text; a button for it would be a second
	// way to do one thing. Close has nothing to close in a private chat.
	if buttons[0].URL != "" {
		t.Fatalf("no button duplicates the source link: %+v", buttons[0])
	}
}

func TestBackAndStaleDataReturnToTheRootPanel(t *testing.T) {
	client := &fakeTelegram{}
	bot := newTestBot(client, &fakeStore{})
	for _, data := range []string{panelRoot, "panel:from-an-older-release"} {
		if err := bot.handleCallback(context.Background(), testCallback(data)); err != nil {
			t.Fatal(err)
		}
	}
	for index, edit := range client.edits {
		if edit.text != rootScreen("en").text || len(flatten(edit.markup)) != 2 {
			t.Fatalf("edit %d is not the root panel: %+v", index, edit)
		}
	}
}

func TestPanelIgnoresACallbackWithoutItsMessage(t *testing.T) {
	client := &fakeTelegram{}
	query := testCallback(panelAbout)
	query.Message = tg.Message{}
	if err := newTestBot(client, &fakeStore{}).handleCallback(context.Background(), query); err != nil {
		t.Fatal(err)
	}
	// The spinner still stops; nothing is edited in chat 0.
	if len(client.answered) != 1 || len(client.edits) != 0 {
		t.Fatalf("answered=%v edits=%+v", client.answered, client.edits)
	}
}

func TestCommandsAreRegisteredPerScope(t *testing.T) {
	client := &fakeTelegram{}
	newTestBot(client, &fakeStore{}).registerCommands(context.Background())
	private := client.scopes["all_private_chats"]
	if len(private) != 1 || private[0].Command != "start" || private[0].Description != "Open the makeitMD panel" {
		t.Fatalf("private=%+v", private)
	}
	// Telegram serves the list matching the client's language, so every
	// language the panel can speak has a menu of its own. Without them the menu
	// would still be English in front of a translated panel.
	for _, code := range i18n.Codes() {
		if _, ok := client.scopes["all_private_chats:"+code]; !ok {
			t.Fatalf("no private menu published for %s", code)
		}
		if _, ok := client.scopes["all_group_chats:"+code]; !ok {
			t.Fatalf("no group menu published for %s", code)
		}
	}
	group := client.scopes["all_group_chats"]
	if len(group) != 1 || !group[0].IsEphemeral {
		t.Fatalf("group /start must answer ephemerally: %+v", group)
	}
	// The global list makeitMD used to publish is cleared, not left behind.
	if published, ok := client.scopes["default"]; !ok || len(published) != 0 {
		t.Fatalf("default=%+v ok=%v", published, ok)
	}
}

func TestGroupStartRedirectsEphemerally(t *testing.T) {
	client := &fakeTelegram{}
	store := &fakeStore{}
	message := groupMessage("/start@makeitMD_bot", 77)
	if err := newTestBot(client, store).handle(context.Background(), paste{updateID: 1, message: message}); err != nil {
		t.Fatal(err)
	}
	if len(client.ephemeral) != 1 || len(client.messages) != 0 {
		t.Fatalf("a group must see nothing: ephemeral=%+v public=%+v", client.ephemeral, client.messages)
	}
	buttons := flatten(client.ephemeral[0].markup)
	if len(buttons) != 1 || buttons[0].URL != "https://t.me/makeitMD_bot" || buttons[0].Style != tg.StylePrimary {
		t.Fatalf("the deep link is the one action of that screen: %+v", buttons)
	}
	// The redirect is an answer to a command, not a panel somebody navigates,
	// and Close is a group-only idea this bot has no use for either way.
	for _, button := range buttons {
		if button.CallbackData != "" || button.Style == tg.StyleDanger {
			t.Fatalf("the group redirect grew panel furniture: %+v", button)
		}
	}
	// A group /start is not a conversion and not somebody to remember.
	if store.touched != 0 || store.created != 0 {
		t.Fatalf("store=%+v", store)
	}
}

func TestGroupStartStaysSilentWhenItCannotBeEphemeral(t *testing.T) {
	client := &fakeTelegram{}
	message := groupMessage("/start", 0)
	if err := newTestBot(client, &fakeStore{}).handle(context.Background(), paste{updateID: 1, message: message}); err != nil {
		t.Fatal(err)
	}
	if len(client.ephemeral) != 0 || len(client.messages) != 0 || len(client.texts) != 0 {
		t.Fatalf("a public reply would be noise for the whole group: %+v", client)
	}
}

func TestGroupTextIsNotRendered(t *testing.T) {
	client := &fakeTelegram{}
	store := &fakeStore{}
	message := groupMessage("# not for a group", 5)
	if err := newTestBot(client, store).handle(context.Background(), paste{updateID: 1, message: message}); err != nil {
		t.Fatal(err)
	}
	if len(client.rich) != 0 || store.created != 0 {
		t.Fatalf("rich=%+v store=%+v", client.rich, store)
	}
}

// The About card states the version and nothing else, the same as every other
// bot in the family. A commit hash here made this one card read differently.
func TestAboutStatesTheVersionAndNoCommit(t *testing.T) {
	if got := buildLabel(build.Info{Version: "", Commit: "none"}); got != "dev" {
		t.Fatalf("label=%q", got)
	}
	if got := buildLabel(build.Info{Version: "v1.2.3", Commit: "0123456789abcdef"}); got != "v1.2.3" {
		t.Fatalf("label=%q", got)
	}
	text := aboutScreen("en", build.Info{Version: "v1.2.3", Commit: "0123456789abcdef"}).text
	if strings.Contains(text, "0123456") {
		t.Fatalf("about card still carries the commit: %q", text)
	}
}

func groupMessage(text string, ephemeralID int64) *tg.Message {
	return &tg.Message{
		MessageID: 3, EphemeralMessageID: ephemeralID,
		Chat: tg.Chat{ID: -100, Type: "supergroup"},
		From: &tg.User{ID: 7, FirstName: "A"}, Text: text,
	}
}

func testCallback(data string) *tg.CallbackQuery {
	return &tg.CallbackQuery{
		ID: "cb1", From: tg.User{ID: 7}, Data: data,
		Message: tg.Message{MessageID: 42, Chat: tg.Chat{ID: 7, Type: "private"}},
	}
}

func flatten(markup *tg.InlineKeyboardMarkup) []tg.InlineKeyboardButton {
	if markup == nil {
		return nil
	}
	var buttons []tg.InlineKeyboardButton
	for _, row := range markup.InlineKeyboard {
		buttons = append(buttons, row...)
	}
	return buttons
}
