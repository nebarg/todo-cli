package scan

import (
	"path/filepath"
	"strings"
)

// syntax is how comments and strings are written in a kind of file. It is
// enough to tell comments from code, not to parse the language.
type syntax struct {
	line   []string    // line comment openers
	blocks [][2]string // block comment openers and closers
	quotes string      // string quotes, which end with the line
	spans  []string    // string delimiters that can span lines
}

var (
	cLike    = syntax{line: []string{"//"}, blocks: [][2]string{{"/*", "*/"}}, quotes: `"'`, spans: []string{"`"}}
	rustLike = syntax{line: []string{"//"}, blocks: [][2]string{{"/*", "*/"}}, quotes: `"`}
	cssLike  = syntax{blocks: [][2]string{{"/*", "*/"}}, quotes: `"'`}
	sassLike = syntax{line: []string{"//"}, blocks: [][2]string{{"/*", "*/"}}, quotes: `"'`}
	php      = syntax{line: []string{"//", "#"}, blocks: [][2]string{{"/*", "*/"}, {"<!--", "-->"}}, quotes: `"'`, spans: []string{"`"}}
	blade    = syntax{line: []string{"//", "#"}, blocks: [][2]string{{"{{--", "--}}"}, {"/*", "*/"}, {"<!--", "-->"}}, quotes: `"'`}
	hash     = syntax{line: []string{"#"}, quotes: `"'`}
	shell    = syntax{line: []string{"#"}, quotes: `"'`, spans: []string{"`"}}
	python   = syntax{line: []string{"#"}, quotes: `"'`, spans: []string{`"""`, `'''`}}
	sql      = syntax{line: []string{"--", "#"}, blocks: [][2]string{{"/*", "*/"}}, quotes: `"'`, spans: []string{"`"}}
	lua      = syntax{line: []string{"--"}, blocks: [][2]string{{"--[[", "]]"}}, quotes: `"'`}
	haskell  = syntax{line: []string{"--"}, blocks: [][2]string{{"{-", "-}"}}, quotes: `"`}
	markup   = syntax{blocks: [][2]string{{"<!--", "-->"}}}
	// Pages and templates carry scripts and styles, so their comments
	// count too. Only double quotes are strings, as an apostrophe in the
	// page's text would otherwise hide the rest of its line.
	page    = syntax{line: []string{"//"}, blocks: [][2]string{{"<!--", "-->"}, {"/*", "*/"}}, quotes: `"`, spans: []string{"`"}}
	twig    = syntax{line: []string{"//"}, blocks: [][2]string{{"{#", "#}"}, {"<!--", "-->"}, {"/*", "*/"}}, quotes: `"`, spans: []string{"`"}}
	smarty  = syntax{line: []string{"//"}, blocks: [][2]string{{"{*", "*}"}, {"<!--", "-->"}, {"/*", "*/"}}, quotes: `"`, spans: []string{"`"}}
	lisp    = syntax{line: []string{";"}, quotes: `"`}
	percent = syntax{line: []string{"%"}, quotes: `"`}
)

var syntaxByExtension = map[string]syntax{
	".go": cLike, ".c": cLike, ".h": cLike, ".cc": cLike, ".cpp": cLike, ".hpp": cLike, ".cs": cLike,
	".java": cLike, ".kt": cLike, ".kts": cLike, ".scala": cLike, ".groovy": cLike, ".gradle": cLike,
	".js": cLike, ".jsx": cLike, ".mjs": cLike, ".cjs": cLike, ".ts": cLike, ".tsx": cLike,
	".swift": cLike, ".dart": cLike, ".proto": cLike, ".sol": cLike, ".m": cLike, ".mm": cLike,
	".rs": rustLike, ".zig": rustLike,
	".css":  cssLike,
	".scss": sassLike, ".less": sassLike,
	".php": php, ".phtml": php, ".inc": php,
	".py": python, ".pyi": python,
	".rb": hash, ".pl": hash, ".pm": hash, ".r": hash, ".ex": hash, ".exs": hash, ".cr": hash, ".jl": hash,
	".yaml": hash, ".yml": hash, ".toml": hash, ".tf": hash, ".nix": hash, ".cmake": hash, ".mk": hash,
	".ps1": hash, ".tcl": hash, ".awk": hash, ".conf": hash,
	".sh": shell, ".bash": shell, ".zsh": shell, ".fish": shell,
	".sql": sql, ".mysql": sql, ".pgsql": sql,
	".lua": lua,
	".hs":  haskell, ".elm": haskell,
	".xml": markup, ".svg": markup, ".md": markup, ".markdown": markup,
	".html": page, ".htm": page, ".vue": page, ".svelte": page, ".astro": page, ".ejs": page, ".hbs": page, ".mustache": page,
	".twig": twig, ".jinja": twig, ".j2": twig,
	".tpl": smarty,
	".clj": lisp, ".cljs": lisp, ".el": lisp, ".lisp": lisp, ".scm": lisp, ".asm": lisp, ".s": lisp,
	".erl": percent, ".hrl": percent, ".tex": percent,
}

var syntaxByName = map[string]syntax{
	"makefile": hash, "gnumakefile": hash, "dockerfile": hash, "containerfile": hash,
	"gemfile": hash, "rakefile": hash, "vagrantfile": hash, "brewfile": hash,
}

// syntaxFor picks a file's comment syntax from its name, or reports that
// the kind of file is unknown. Any file with PHP in it is read as PHP, as
// PHP often lives in files named for what they serve, such as .html.
func syntaxFor(path, content string) (syntax, bool) {
	name := strings.ToLower(filepath.Base(path))
	if strings.HasSuffix(name, ".blade.php") {
		return blade, true
	}
	if strings.Contains(content, "<?php") || strings.Contains(content, "<?=") {
		return php, true
	}
	if s, ok := syntaxByName[name]; ok {
		return s, true
	}
	s, ok := syntaxByExtension[filepath.Ext(name)]
	return s, ok
}

// comments returns the comment text on each line, following block comments
// and multi-line strings from one line to the next. A line can hold several
// comments, such as "/* a */ code // b".
func (s syntax) comments(lines []string) [][]string {
	result := make([][]string, len(lines))
	starts := s.starts()
	closer, span := "", ""
	for n, line := range lines {
		var parts []string
		i := 0
		for i <= len(line) {
			if closer != "" {
				end := strings.Index(line[i:], closer)
				if end < 0 {
					parts = append(parts, line[i:])
					break
				}
				parts = append(parts, line[i:i+end])
				i += end + len(closer)
				closer = ""
				continue
			}
			if span != "" {
				end := strings.Index(line[i:], span)
				if end < 0 {
					break
				}
				i += end + len(span)
				span = ""
				continue
			}
			if i == len(line) {
				break
			}
			if !starts[line[i]] {
				i++
				continue
			}
			if opener, ok := s.lineComment(line, i); ok {
				parts = append(parts, line[i+len(opener):])
				break
			}
			if block, ok := s.blockComment(line, i); ok {
				i += len(block[0])
				closer = block[1]
				continue
			}
			if delimiter, ok := s.spanString(line, i); ok {
				i += len(delimiter)
				span = delimiter
				continue
			}
			if strings.IndexByte(s.quotes, line[i]) >= 0 {
				i = stringEnd(line, i)
				continue
			}
			i++
		}
		result[n] = parts
	}
	return result
}

// starts marks the bytes that can begin a comment or a string, so the
// rest are skipped without checking every opener.
func (s syntax) starts() *[256]bool {
	var table [256]bool
	for _, opener := range s.line {
		table[opener[0]] = true
	}
	for _, block := range s.blocks {
		table[block[0][0]] = true
	}
	for _, delimiter := range s.spans {
		table[delimiter[0]] = true
	}
	for i := range len(s.quotes) {
		table[s.quotes[i]] = true
	}
	return &table
}

// blockComment is checked before line comments' openers can claim a
// prefix, so Lua's "--[[" opens a block rather than a line comment.
func (s syntax) blockComment(line string, i int) ([2]string, bool) {
	for _, block := range s.blocks {
		if strings.HasPrefix(line[i:], block[0]) {
			return block, true
		}
	}
	return [2]string{}, false
}

// lineComment reports a line comment opening at i. A "#" only counts at the
// start of a word, so shell's "$#" and "${#x}" stay code, and never before
// "[", which opens PHP and Rust attributes.
func (s syntax) lineComment(line string, i int) (string, bool) {
	if _, ok := s.blockComment(line, i); ok {
		return "", false
	}
	for _, opener := range s.line {
		if !strings.HasPrefix(line[i:], opener) {
			continue
		}
		if opener == "#" && ((i > 0 && line[i-1] != ' ' && line[i-1] != '\t') || strings.HasPrefix(line[i+1:], "[")) {
			continue
		}
		return opener, true
	}
	return "", false
}

func (s syntax) spanString(line string, i int) (string, bool) {
	for _, delimiter := range s.spans {
		if strings.HasPrefix(line[i:], delimiter) {
			return delimiter, true
		}
	}
	return "", false
}

// stringEnd is the index after the string quoted at i, or the end of the
// line for a string left open, so a stray quote can't hide later lines.
func stringEnd(line string, i int) int {
	quote := line[i]
	for j := i + 1; j < len(line); j++ {
		switch line[j] {
		case '\\':
			j++
		case quote:
			return j + 1
		}
	}
	return len(line)
}

// containsTodo reports whether s holds "todo" in any case. It is much
// cheaper than the marker patterns, so it rules out most text first.
func containsTodo(s string) bool {
	for i := 0; i+4 <= len(s); i++ {
		// OR-ing 0x20 lower-cases an ASCII letter and leaves no other byte
		// equal to a lower-case one.
		if s[i]|0x20 == 't' && s[i+1]|0x20 == 'o' && s[i+2]|0x20 == 'd' && s[i+3]|0x20 == 'o' {
			return true
		}
	}
	return false
}

// fileTodos finds the to-dos in a file's content. Files of a known kind are
// read with their comment syntax; others line by line, guessing where each
// line's comment starts.
func fileTodos(path string, content string) []Match {
	if !containsTodo(content) {
		return nil
	}
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSuffix(line, "\r")
	}
	var matches []Match
	s, known := syntaxFor(path, content)
	if !known {
		for i, line := range lines {
			if c, ok := commentTodo(line); ok {
				matches = append(matches, newMatch(path, i+1, line, c))
			}
		}
		return matches
	}
	for i, parts := range s.comments(lines) {
		for _, part := range parts {
			if c, ok := todoInComment(part); ok {
				matches = append(matches, newMatch(path, i+1, lines[i], c))
				break
			}
		}
	}
	return matches
}
