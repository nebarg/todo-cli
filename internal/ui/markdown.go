package ui

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
)

// Inline draws text in style with its Markdown bold, italic, strikethrough
// and code spans formatted and their markers removed. Control characters are
// replaced as CleanDisplay does.
func (t Theme) Inline(text string, style lipgloss.Style) string {
	var result strings.Builder
	for _, s := range parseInline(CleanDisplay(text)) {
		spanStyle := style
		if s.bold {
			spanStyle = spanStyle.Bold(true)
		}
		if s.italic {
			spanStyle = spanStyle.Italic(true)
		}
		if s.strike {
			spanStyle = t.struck(spanStyle)
		}
		if s.code {
			spanStyle = t.code(spanStyle)
			s.text = " " + s.text + " "
		}
		result.WriteString(spanStyle.Render(s.text))
	}
	return result.String()
}

// struck strikes style through and greys it out, as done tasks are, so struck
// text stands out in terminals that don't draw strikethrough. On a selected
// row it takes the selection's lighter grey, which stays readable there.
func (t Theme) struck(style lipgloss.Style) lipgloss.Style {
	grey := t.MutedStyle.GetForeground()
	if style.GetBackground() == t.ColorSelection {
		grey = t.SelectedDoneStyle.GetForeground()
	}
	return style.Foreground(grey).Strikethrough(true)
}

// code draws a code span in its own colour, padded on the field colour like a
// key cap. Text greyed out, as a done task's or struck text is, stays grey.
func (t Theme) code(style lipgloss.Style) lipgloss.Style {
	style = style.Background(t.ColorField)
	switch style.GetForeground() {
	case t.MutedStyle.GetForeground(), t.SelectedDoneStyle.GetForeground():
		return style
	}
	return style.Foreground(t.ColorCode)
}

// MarkdownLines draws each line of a Markdown text with Inline, except in
// fenced code blocks, which are shown as written.
func (t Theme) MarkdownLines(text string) []string {
	var lines []string
	fence := ""
	for line := range strings.SplitSeq(text, "\n") {
		switch {
		case fence != "":
			lines = append(lines, CleanDisplay(line))
			if closesFence(line, fence) {
				fence = ""
			}
		case openingFence(line) != "":
			fence = openingFence(line)
			lines = append(lines, CleanDisplay(line))
		default:
			lines = append(lines, t.Inline(line, lipgloss.NewStyle()))
		}
	}
	return lines
}

// openingFence is the run of three or more backticks or tildes that opens a
// fenced code block on line, or "" when line doesn't open one.
func openingFence(line string) string {
	line = strings.TrimLeft(line, " ")
	for _, marker := range []string{"`", "~"} {
		rest := strings.TrimLeft(line, marker)
		run := line[:len(line)-len(rest)]
		// A backtick in the info string makes the line inline code instead.
		if len(run) >= 3 && (marker == "~" || !strings.Contains(rest, "`")) {
			return run
		}
	}
	return ""
}

// closesFence is true when line ends the block fence opened: a run of the
// same marker at least as long, and nothing else.
func closesFence(line, fence string) bool {
	line = strings.TrimSpace(line)
	return strings.HasPrefix(line, fence) && strings.Trim(line, fence[:1]) == ""
}

// span is a run of text with one formatting.
type span struct {
	text string
	spanFormat
}

type spanFormat struct {
	bold, italic, strike, code bool
}

// inlineNode is a piece of a line being parsed: text, a code span, or a run
// of *, _ or ~ markers that may open or close emphasis or strikethrough.
type inlineNode struct {
	text   string // the text, or a code span's content
	code   bool
	marker rune // *, _ or ~ for a run of markers, otherwise 0
	length int  // a run's length as written
	unused int  // a run's markers not yet matched, which show as written
	// canOpen and canClose follow CommonMark's flanking rules.
	canOpen, canClose bool
}

// emphasis spans the nodes strictly between from and to, struck through for
// tildes, otherwise bold for two markers or italic for one.
type emphasis struct {
	from, to int
	marker   rune
	strong   bool
}

// parseInline splits a line into spans, following CommonMark for code spans,
// backslash escapes and emphasis, and GitHub's Markdown for strikethrough, so
// that markers in snake_case, 2 * 3, **kwargs or ~/notes stay as written.
func parseInline(text string) []span {
	nodes := inlineNodes(text)
	ranges := matchEmphasis(nodes)
	var spans []span
	for i, n := range nodes {
		s := span{text: n.text, code: n.code}
		if n.marker != 0 {
			s.text = strings.Repeat(string(n.marker), n.unused)
		}
		for _, r := range ranges {
			if r.from < i && i < r.to {
				switch {
				case r.marker == '~':
					s.strike = true
				case r.strong:
					s.bold = true
				default:
					s.italic = true
				}
			}
		}
		if s.text == "" {
			continue
		}
		if last := len(spans) - 1; last >= 0 && !s.code && spans[last].spanFormat == s.spanFormat {
			spans[last].text += s.text
			continue
		}
		spans = append(spans, s)
	}
	return spans
}

// inlineNodes reads code spans, escapes and runs of emphasis markers from
// text, leaving the rest as text.
func inlineNodes(text string) []inlineNode {
	var nodes []inlineNode
	var plain strings.Builder
	flush := func() {
		if plain.Len() > 0 {
			nodes = append(nodes, inlineNode{text: plain.String()})
			plain.Reset()
		}
	}
	for i := 0; i < len(text); {
		c := text[i]
		switch {
		case c == '\\' && i+1 < len(text) && isASCIIPunct(text[i+1]):
			plain.WriteByte(text[i+1])
			i += 2
		case c == '`':
			run := runLength(text, i)
			end := closingBackticks(text, i+run, run)
			if end < 0 {
				plain.WriteString(text[i : i+run])
				i += run
				continue
			}
			flush()
			nodes = append(nodes, inlineNode{text: codeContent(text[i+run : end]), code: true})
			i = end + run
		case c == '*' || c == '_' || c == '~':
			run := runLength(text, i)
			flush()
			nodes = append(nodes, markerRun(text, i, run))
			i += run
		default:
			plain.WriteByte(c)
			i++
		}
	}
	flush()
	return nodes
}

// runLength is how many times the byte at text[i] repeats from i.
func runLength(text string, i int) int {
	n := 1
	for i+n < len(text) && text[i+n] == text[i] {
		n++
	}
	return n
}

// closingBackticks is where the next run of exactly length backticks starts
// at or after from, or -1 if there is none.
func closingBackticks(text string, from, length int) int {
	for i := from; i < len(text); {
		if text[i] != '`' {
			i++
			continue
		}
		run := runLength(text, i)
		if run == length {
			return i
		}
		i += run
	}
	return -1
}

// codeContent strips the single space either side of a code span's content
// that lets it start or end with a backtick.
func codeContent(code string) string {
	if len(code) >= 2 && code[0] == ' ' && code[len(code)-1] == ' ' && strings.Trim(code, " ") != "" {
		return code[1 : len(code)-1]
	}
	return code
}

// markerRun is the run of length markers at text[i], with whether it can open
// or close emphasis by the characters either side of it.
func markerRun(text string, i, length int) inlineNode {
	before, after := ' ', ' '
	if i > 0 {
		before, _ = utf8.DecodeLastRuneInString(text[:i])
	}
	if i+length < len(text) {
		after, _ = utf8.DecodeRuneInString(text[i+length:])
	}
	left := !unicode.IsSpace(after) && (!isPunct(after) || unicode.IsSpace(before) || isPunct(before))
	right := !unicode.IsSpace(before) && (!isPunct(before) || unicode.IsSpace(after) || isPunct(after))
	n := inlineNode{marker: rune(text[i]), length: length, unused: length, canOpen: left, canClose: right}
	switch {
	case n.marker == '_':
		// Underscores inside a word, as in snake_case, aren't emphasis.
		n.canOpen = left && (!right || isPunct(before))
		n.canClose = right && (!left || isPunct(after))
	case n.marker == '~' && length > 2:
		// Strikethrough takes one or two tildes.
		n.canOpen, n.canClose = false, false
	}
	return n
}

// matchEmphasis pairs runs of markers as CommonMark does, using up their
// markers: each closer takes the nearest opener of the same marker, two
// markers at a time for bold, or one for italic. Tildes pair only as runs of
// the same length, as in GitHub's Markdown.
func matchEmphasis(nodes []inlineNode) []emphasis {
	var found []emphasis
	for c := range nodes {
		closer := &nodes[c]
		for o := c - 1; o >= 0 && closer.canClose && closer.unused > 0; o-- {
			opener := &nodes[o]
			if !opener.canOpen || opener.marker != closer.marker || opener.unused == 0 || breaksRuleOfThree(opener, closer) {
				continue
			}
			if closer.marker == '~' && opener.unused != closer.unused {
				break
			}
			n := 1
			if opener.unused >= 2 && closer.unused >= 2 {
				n = 2
			}
			opener.unused -= n
			closer.unused -= n
			found = append(found, emphasis{from: o, to: c, marker: closer.marker, strong: n == 2})
			for i := o + 1; i < c; i++ {
				nodes[i].canOpen = false
			}
			o++ // The opener may have markers left for the rest of the closer's.
		}
	}
	return found
}

// breaksRuleOfThree keeps runs such as the ** and * in *a**b* from pairing
// up: when either run could both open and close, their lengths must not add
// up to a multiple of three, unless both are multiples of three.
func breaksRuleOfThree(opener, closer *inlineNode) bool {
	if !opener.canClose && !closer.canOpen {
		return false
	}
	return (opener.length+closer.length)%3 == 0 && (opener.length%3 != 0 || closer.length%3 != 0)
}

func isPunct(r rune) bool { return unicode.IsPunct(r) || unicode.IsSymbol(r) }

func isASCIIPunct(c byte) bool { return c < utf8.RuneSelf && isPunct(rune(c)) }
