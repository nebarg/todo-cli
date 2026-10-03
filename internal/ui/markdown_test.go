package ui

import (
	"slices"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// describe writes spans as plain text, with formatted runs as {flags:text}:
// b for bold, i for italic, s for strikethrough and c for code.
func describe(spans []span) string {
	var result strings.Builder
	for _, s := range spans {
		flags := ""
		if s.bold {
			flags += "b"
		}
		if s.italic {
			flags += "i"
		}
		if s.strike {
			flags += "s"
		}
		if s.code {
			flags += "c"
		}
		if flags == "" {
			result.WriteString(s.text)
		} else {
			result.WriteString("{" + flags + ":" + s.text + "}")
		}
	}
	return result.String()
}

func TestInlineMarkdownIsParsed(t *testing.T) {
	for _, c := range []struct{ text, want string }{
		{"plain text", "plain text"},
		{"**bold** text", "{b:bold} text"},
		{"__bold__ text", "{b:bold} text"},
		{"*italic* and _italic_", "{i:italic} and {i:italic}"},
		{"***both***", "{bi:both}"},
		{"**bold *and italic* inside**", "{b:bold }{bi:and italic}{b: inside}"},
		{"*italic **and bold** inside*", "{i:italic }{bi:and bold}{i: inside}"},
		{"a*b*c", "a{i:b}c"},
		{"**foo*", "*{i:foo}"},
		{"*foo**bar*", "{i:foo**bar}"},
		{"run `go test ./...` first", "run {c:go test ./...} first"},
		{"``code with ` inside``", "{c:code with ` inside}"},
		{"`` `ticks` ``", "{c:`ticks`}"},
		{"`  `", "{c:  }"},
		{"`a``b`", "{c:a``b}"},
		{"`**not bold**`", "{c:**not bold**}"},
		{"`\\*`", "{c:\\*}"},
		{"**`bold code`**", "{bc:bold code}"},
		{"`one` `two`", "{c:one} {c:two}"},
		{"~~gone~~ kept", "{s:gone} kept"},
		{"~gone~ kept", "{s:gone} kept"},
		{"~~**both**~~", "{bs:both}"},
		{"**~~both~~**", "{bs:both}"},
		{"~~`code`~~", "{sc:code}"},
		{"~~a~b~~", "{s:a~b}"},
		{"~~a~", "~~a~"},
		{"**~~a**~~", "{b:~~a}~~"},
		{"\\~not struck\\~", "~not struck~"},
		{"\\*not italic\\*", "*not italic*"},
		{"\\`not code\\`", "`not code`"},
		{"C:\\Users\\me", "C:\\Users\\me"},
		{"trailing \\", "trailing \\"},
	} {
		if got := describe(parseInline(c.text)); got != c.want {
			t.Errorf("parseInline(%q) = %q, want %q", c.text, got, c.want)
		}
	}
}

func TestMarkersOutsideMarkdownStayAsWritten(t *testing.T) {
	for _, text := range []string{
		"rename user_id_field to user_key",
		"a_b_c",
		"2 * 3 * 4",
		"pass *args and **kwargs through",
		"ignore src/**/*.ts and *.min.js",
		"* not a list *",
		"**unclosed bold",
		"unopened bold**",
		"`unclosed code",
		"``mismatched` ticks",
		"****",
		"don't_stop *",
		"copy ~/notes to ~/backup",
		"takes ~5 minutes",
		"~~~three tildes~~~",
		"~~unclosed strikethrough",
		"a ~~ b ~~ c",
		"",
	} {
		if got := describe(parseInline(text)); got != text {
			t.Errorf("parseInline(%q) = %q, want it unchanged", text, got)
		}
	}
}

func TestInlineDrawsSpansInTheBaseStyle(t *testing.T) {
	theme := NewTheme(true)
	base := lipgloss.NewStyle().Foreground(theme.ColorStrong).Background(theme.ColorSelection)
	got := theme.Inline("Fix **login** in `auth.go` *soon* ~~or not~~", base)
	want := base.Render("Fix ") + base.Bold(true).Render("login") + base.Render(" in ") +
		base.Background(theme.ColorField).Foreground(theme.ColorCode).Render(" auth.go ") + base.Render(" ") + base.Italic(true).Render("soon") +
		base.Render(" ") + base.Foreground(theme.SelectedDoneStyle.GetForeground()).Strikethrough(true).Render("or not")
	if got != want {
		t.Errorf("Inline drew\n%q\nwant\n%q", got, want)
	}
	if plain := ansi.Strip(got); plain != "Fix login in  auth.go  soon or not" {
		t.Errorf("Inline text = %q", plain)
	}
}

func TestStruckTextIsGreyedOut(t *testing.T) {
	for _, dark := range []bool{true, false} {
		theme := NewTheme(dark)
		for _, base := range []lipgloss.Style{theme.TaskTitleStyle, lipgloss.NewStyle().Foreground(theme.ColorStrong), lipgloss.NewStyle()} {
			want := base.Foreground(theme.ColorMuted).Strikethrough(true).Render("gone")
			if got := theme.Inline("~~gone~~", base); got != want {
				t.Errorf("dark %v: struck text = %q, want %q", dark, got, want)
			}
		}
		selected := lipgloss.NewStyle().Foreground(theme.ColorStrong).Background(theme.ColorSelection)
		want := theme.SelectedDoneStyle.Strikethrough(true).Render("gone")
		if got := theme.Inline("~~gone~~", selected); got != want {
			t.Errorf("dark %v: selected struck text = %q, want %q", dark, got, want)
		}
	}
}

func TestGreyedOutCodeStaysGrey(t *testing.T) {
	for _, dark := range []bool{true, false} {
		theme := NewTheme(dark)
		for _, base := range []lipgloss.Style{theme.MutedStyle, theme.SelectedDoneStyle} {
			want := base.Background(theme.ColorField).Render(" go test ")
			if got := theme.Inline("`go test`", base); got != want {
				t.Errorf("dark %v: done task's code = %q, want %q", dark, got, want)
			}
		}
		grey := theme.MutedStyle.GetForeground()
		want := lipgloss.NewStyle().Foreground(grey).Strikethrough(true).Background(theme.ColorField).Render(" go test ")
		if got := theme.Inline("~~`go test`~~", lipgloss.NewStyle()); got != want {
			t.Errorf("dark %v: struck code = %q, want %q", dark, got, want)
		}
	}
}

func TestInlineReplacesControlCharacters(t *testing.T) {
	got := ansi.Strip(NewTheme(true).Inline("a\x1b[31mb\tc", lipgloss.NewStyle()))
	if got != "a [31mb c" {
		t.Errorf("Inline kept control characters: %q", got)
	}
}

func TestMarkdownLinesLeaveCodeBlocksAsWritten(t *testing.T) {
	theme := NewTheme(true)
	for _, c := range []struct {
		name, text string
		want       []string
	}{
		{
			"backticks",
			"Run **this**\n```sh\necho `that` **x**\n```\nAfter *it*",
			[]string{"Run this", "```sh", "echo `that` **x**", "```", "After it"},
		},
		{
			"tildes",
			"~~~\n**x**\n~~~\n**y**",
			[]string{"~~~", "**x**", "~~~", "y"},
		},
		{
			"indented",
			"- step\n  ```\n  **x**\n  ```\n- **next**",
			[]string{"- step", "  ```", "  **x**", "  ```", "- next"},
		},
		{
			"a shorter fence doesn't close a longer one",
			"````\n```\n**x**\n````\n**y**",
			[]string{"````", "```", "**x**", "````", "y"},
		},
		{
			"a fence with text after it doesn't close the block",
			"```\n``` **x**\n**y**\n```\n**z**",
			[]string{"```", "``` **x**", "**y**", "```", "z"},
		},
		{
			"an unclosed fence runs to the end",
			"```\n**x**",
			[]string{"```", "**x**"},
		},
		{
			"backticks on one line are inline code",
			"```code``` and **bold**",
			[]string{" code  and bold"},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			lines := theme.MarkdownLines(c.text)
			for i, line := range lines {
				lines[i] = ansi.Strip(line)
			}
			if !slices.Equal(lines, c.want) {
				t.Errorf("MarkdownLines(%q) = %q, want %q", c.text, lines, c.want)
			}
		})
	}
}

func TestWrappedLinesKeepTheirStyle(t *testing.T) {
	bold := lipgloss.NewStyle().Bold(true)
	lines := WrapLines([]string{bold.Render("aaa bbb ccc")}, 4)
	if len(lines) != 3 {
		t.Fatalf("WrapLines gave %d lines, want 3: %q", len(lines), lines)
	}
	for _, line := range lines {
		if line != bold.Render(ansi.Strip(line)) {
			t.Errorf("wrapped line %q is not bold on its own", line)
		}
	}
}

func TestStyledColumnPadsInTheGivenStyle(t *testing.T) {
	pad := lipgloss.NewStyle().Background(lipgloss.Color("#2457A6"))
	bold := pad.Bold(true)
	if got, want := StyledColumn(bold.Render("ab"), 5, pad), bold.Render("ab")+pad.Render("   "); got != want {
		t.Errorf("StyledColumn padded to %q, want %q", got, want)
	}
	if got := ansi.Strip(StyledColumn(bold.Render("abcdef"), 4, pad)); got != "abc…" {
		t.Errorf("StyledColumn cut to %q, want %q", got, "abc…")
	}
}
