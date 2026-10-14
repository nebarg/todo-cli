package dashboard

import (
	"strings"
	"unicode/utf8"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
)

// textState is what a text box holds, and where its cursor is, in runes from
// the start.
type textState struct {
	value  string
	cursor int
}

// editKind is what a change to a text box did, so that a run of one kind
// undoes in one step.
type editKind int

const (
	editNone    editKind = iota // no run is going: the next change starts a step
	editInsert                  // text went in at one place
	editDelete                  // text came out of one place
	editReplace                 // anything else, such as typing over a selection
)

// undoHistory is one text box's undo and redo steps, each the state the box
// was in before or after it. A run of typing, or of deleting, without the
// cursor moving in between is one step; a paste or a replacement is a step
// of its own.
type undoHistory struct {
	past, future []textState
	run          editKind
}

// record notes a text box going from before to after. alone makes a change a
// step of its own, as a paste is.
func (h *undoHistory) record(before, after textState, alone bool) {
	switch {
	case before == after:
		return
	case before.value == after.value:
		h.run = editNone
		return
	}
	kind := editKindOf(before.value, after.value)
	if alone || kind == editReplace || kind != h.run {
		h.past = append(h.past, before)
	}
	h.future = nil
	h.run = kind
	if alone || kind == editReplace {
		h.run = editNone
	}
}

// undo is the state before the last step, taking current to redo, or false
// when there's nothing to undo.
func (h *undoHistory) undo(current textState) (textState, bool) {
	return h.step(&h.past, &h.future, current)
}

// redo is the state after the last step undone, taking current to undo, or
// false when there's nothing to redo.
func (h *undoHistory) redo(current textState) (textState, bool) {
	return h.step(&h.future, &h.past, current)
}

func (h *undoHistory) step(from, to *[]textState, current textState) (textState, bool) {
	if len(*from) == 0 {
		return textState{}, false
	}
	state := (*from)[len(*from)-1]
	*from = (*from)[:len(*from)-1]
	*to = append(*to, current)
	h.run = editNone
	return state, true
}

// breakRun makes the next change a step of its own, as when the box regains
// focus.
func (h *undoHistory) breakRun() { h.run = editNone }

// editKindOf is what changing before to after did: whether all of before is
// still there around something put in, or all of after was there around
// something taken out.
func editKindOf(before, after string) editKind {
	prefix := 0
	for prefix < len(before) && prefix < len(after) && before[prefix] == after[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < len(before)-prefix && suffix < len(after)-prefix && before[len(before)-1-suffix] == after[len(after)-1-suffix] {
		suffix++
	}
	switch {
	case prefix+suffix == len(before):
		return editInsert
	case prefix+suffix == len(after):
		return editDelete
	}
	return editReplace
}

func textareaState(t textarea.Model) textState {
	cursor := t.Column()
	for _, line := range strings.Split(t.Value(), "\n")[:t.Line()] {
		cursor += utf8.RuneCountInString(line) + 1
	}
	return textState{t.Value(), cursor}
}

// setTextarea puts s in t. The text after the cursor goes in first, then the
// text before it at the start, which leaves the cursor between the two.
func setTextarea(t *textarea.Model, s textState) {
	runes := []rune(s.value)
	t.SetValue(string(runes[s.cursor:]))
	t.MoveToBegin()
	t.InsertString(string(runes[:s.cursor]))
}

func textinputState(t textinput.Model) textState {
	return textState{t.Value(), t.Position()}
}

func setTextinput(t *textinput.Model, s textState) {
	t.SetValue(s.value)
	t.SetCursor(s.cursor)
}
