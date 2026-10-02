package scan

import (
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

// ignoreRule is one pattern from a gitignore file.
type ignoreRule struct {
	glob     []string // the pattern's segments, in path.Match syntax
	anchored bool     // whether glob is a path from the gitignore's directory, rather than a name at any depth
	depth    int      // how many directories below the repository root the gitignore is
	negate   bool
	dirOnly  bool
}

// parseIgnore reads the patterns of a gitignore file depth directories below
// the repository root.
func parseIgnore(content string, depth int) []ignoreRule {
	var rules []ignoreRule
	for line := range strings.Lines(content) {
		line = trimIgnoreLine(line)
		if line == "" || line[0] == '#' {
			continue
		}
		rule := ignoreRule{depth: depth}
		if line[0] == '!' {
			rule.negate, line = true, line[1:]
		}
		line, rule.dirOnly = strings.CutSuffix(line, "/")
		// A slash anywhere else ties the pattern to the gitignore's directory.
		rule.anchored = strings.Contains(line, "/")
		line = strings.TrimPrefix(line, "/")
		if line == "" {
			continue
		}
		for segment := range strings.SplitSeq(line, "/") {
			rule.glob = append(rule.glob, pathMatchGlob(segment))
		}
		rules = append(rules, rule)
	}
	return rules
}

// trimIgnoreLine drops a line's ending and trailing spaces, keeping a space
// escaped with a backslash.
func trimIgnoreLine(line string) string {
	line = strings.TrimRight(line, "\r\n")
	trimmed := strings.TrimRight(line, " ")
	backslashes := len(trimmed) - len(strings.TrimRight(trimmed, `\`))
	if backslashes%2 == 1 && len(trimmed) < len(line) {
		return trimmed + " "
	}
	return trimmed
}

// pathMatchGlob turns gitignore's negated class, "[!a]", into path.Match's
// "[^a]".
func pathMatchGlob(glob string) string {
	b := []byte(glob)
	for i := 0; i < len(b)-1; i++ {
		switch {
		case b[i] == '\\':
			i++
		case b[i] == '[' && b[i+1] == '!':
			b[i+1] = '^'
		}
	}
	return string(b)
}

// matches reports whether the rule matches a path, given as its segments
// from the repository root. The path must be below the rule's gitignore.
func (r ignoreRule) matches(segments []string, isDir bool) bool {
	if r.dirOnly && !isDir {
		return false
	}
	if !r.anchored {
		ok, _ := path.Match(r.glob[0], segments[len(segments)-1])
		return ok
	}
	return matchSegments(r.glob, segments[r.depth:])
}

// matchSegments matches path segments against glob segments, where "**"
// matches any number of segments; at the end, at least one.
func matchSegments(glob, segments []string) bool {
	for len(glob) > 0 {
		if glob[0] == "**" {
			if len(glob) == 1 {
				return len(segments) > 0
			}
			for i := range len(segments) + 1 {
				if matchSegments(glob[1:], segments[i:]) {
					return true
				}
			}
			return false
		}
		if len(segments) == 0 {
			return false
		}
		if ok, _ := path.Match(glob[0], segments[0]); !ok {
			return false
		}
		glob, segments = glob[1:], segments[1:]
	}
	return len(segments) == 0
}

// ignoreScope is a directory's place in its repository and the ignore rules
// that apply there, least important first. Outside a repository, nothing is
// ignored.
type ignoreScope struct {
	inRepo   bool
	segments []string // the directory's path from the repository root
	rules    []ignoreRule
}

// ignores reports whether a path in the scope, given as its segments from
// the repository root, is ignored. The last rule that matches decides.
func (s ignoreScope) ignores(segments []string, isDir bool) bool {
	for _, r := range slices.Backward(s.rules) {
		if r.matches(segments, isDir) {
			return !r.negate
		}
	}
	return false
}

// withIgnoreFile adds the rules of a gitignore file depth directories below
// the repository root, leaving rules that other directories share untouched.
func withIgnoreFile(rules []ignoreRule, file string, depth int) []ignoreRule {
	content, err := os.ReadFile(file)
	if err != nil {
		return rules
	}
	return append(slices.Clip(rules), parseIgnore(string(content), depth)...)
}

// globalIgnoreFile is the user's global gitignore: core.excludesFile in
// their Git configuration, or Git's default. Includes in the configuration
// aren't followed.
func globalIgnoreFile() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	config := os.Getenv("XDG_CONFIG_HOME")
	if config == "" {
		config = filepath.Join(home, ".config")
	}
	file := filepath.Join(config, "git", "ignore")
	configs := []string{filepath.Join(config, "git", "config"), filepath.Join(home, ".gitconfig")}
	if global := os.Getenv("GIT_CONFIG_GLOBAL"); global != "" {
		configs = []string{global}
	}
	for _, c := range configs {
		if value, ok := configValue(c, "core", "excludesfile"); ok {
			file = value
			if rest, ok := strings.CutPrefix(value, "~/"); ok {
				file = filepath.Join(home, rest)
			}
		}
	}
	return file
}

// configValue finds the last value of a key in a section of a Git config
// file. Names are compared case-insensitively.
func configValue(file, section, key string) (value string, found bool) {
	content, err := os.ReadFile(file)
	if err != nil {
		return "", false
	}
	var current string
	for line := range strings.Lines(string(content)) {
		line = strings.TrimSpace(line)
		if header, ok := strings.CutPrefix(line, "["); ok {
			header, _, _ = strings.Cut(header, "]")
			current, _, _ = strings.Cut(strings.TrimSpace(header), " ")
			continue
		}
		name, v, ok := strings.Cut(line, "=")
		if ok && strings.EqualFold(current, section) && strings.EqualFold(strings.TrimSpace(name), key) {
			value, found = configString(strings.TrimSpace(v)), true
		}
	}
	return value, found
}

// configString reads a config value, which may be quoted or followed by a
// comment.
func configString(v string) string {
	if quoted, ok := strings.CutPrefix(v, `"`); ok {
		value, _, _ := strings.Cut(quoted, `"`)
		return value
	}
	if i := strings.IndexAny(v, "#;"); i >= 0 {
		v = v[:i]
	}
	return strings.TrimSpace(v)
}
