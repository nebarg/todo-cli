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
func (r ignoreRule) matches(segments []string, isDir, foldCase bool) bool {
	if r.dirOnly && !isDir {
		return false
	}
	if !r.anchored {
		return matchSegment(r.glob[0], segments[len(segments)-1], foldCase)
	}
	return matchSegments(r.glob, segments[r.depth:], foldCase)
}

// matchSegments matches path segments against glob segments, where "**"
// matches any number of segments; at the end, at least one.
func matchSegments(glob, segments []string, foldCase bool) bool {
	for len(glob) > 0 {
		if glob[0] == "**" {
			if len(glob) == 1 {
				return len(segments) > 0
			}
			for i := range len(segments) + 1 {
				if matchSegments(glob[1:], segments[i:], foldCase) {
					return true
				}
			}
			return false
		}
		if len(segments) == 0 || !matchSegment(glob[0], segments[0], foldCase) {
			return false
		}
		glob, segments = glob[1:], segments[1:]
	}
	return len(segments) == 0
}

// matchSegment matches one path segment against one glob segment, ignoring
// case when foldCase is set.
func matchSegment(glob, segment string, foldCase bool) bool {
	if foldCase {
		glob, segment = strings.ToLower(glob), strings.ToLower(segment)
	}
	ok, _ := path.Match(glob, segment)
	return ok
}

// ignoreScope is a directory's place in its repository and the ignore rules
// that apply there, least important first. Outside a repository, nothing is
// ignored.
type ignoreScope struct {
	inRepo   bool
	foldCase bool     // the repository's core.ignoreCase: rules match regardless of case
	segments []string // the directory's path from the repository root
	rules    []ignoreRule
}

// ignores reports whether a path in the scope, given as its segments from
// the repository root, is ignored. The last rule that matches decides.
func (s ignoreScope) ignores(segments []string, isDir bool) bool {
	for _, r := range slices.Backward(s.rules) {
		if r.matches(segments, isDir, s.foldCase) {
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
// their Git configuration, or Git's default.
func globalIgnoreFile() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	value, ok := lastConfigValue(userConfigFiles(home), "core", "excludesfile")
	if !ok {
		return filepath.Join(userConfigDir(home), "git", "ignore")
	}
	if rest, ok := strings.CutPrefix(value, "~/"); ok {
		return filepath.Join(home, rest)
	}
	return value
}

// ignoresCase reads core.ignoreCase for the repository at root, from its
// own configuration or else the user's. Git sets it in a repository it
// creates on a file system that ignores case, as macOS's does by default.
func ignoresCase(root string) bool {
	var files []string
	if home, err := os.UserHomeDir(); err == nil {
		files = userConfigFiles(home)
	}
	value, _ := lastConfigValue(append(files, filepath.Join(root, ".git", "config")), "core", "ignorecase")
	return configBool(value)
}

// userConfigDir is $XDG_CONFIG_HOME, or ~/.config without it.
func userConfigDir(home string) string {
	if config := os.Getenv("XDG_CONFIG_HOME"); config != "" {
		return config
	}
	return filepath.Join(home, ".config")
}

// userConfigFiles are the user's Git configuration files, in the order Git
// reads them. The system configuration and includes aren't read.
func userConfigFiles(home string) []string {
	if global := os.Getenv("GIT_CONFIG_GLOBAL"); global != "" {
		return []string{global}
	}
	return []string{filepath.Join(userConfigDir(home), "git", "config"), filepath.Join(home, ".gitconfig")}
}

// lastConfigValue finds a key's value in the last of files that sets it,
// as a later Git configuration file overrides an earlier one.
func lastConfigValue(files []string, section, key string) (value string, found bool) {
	for _, file := range files {
		if v, ok := configValue(file, section, key); ok {
			value, found = v, true
		}
	}
	return value, found
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

// configBool reads a Git boolean, which is true written as true, yes, on
// or 1, in any case.
func configBool(v string) bool {
	switch strings.ToLower(v) {
	case "true", "yes", "on", "1":
		return true
	}
	return false
}
