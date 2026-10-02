package scan

import "github.com/nebarg/todo-cli/internal/level"

// AnyZeros is the level that matches every level of zeros. It can't be 0*,
// as the shell would expand that against file names.
const AnyZeros = "0+"

// ValidLevel accepts todo-system's levels, one digit or only zeros, and
// AnyZeros.
func ValidLevel(l string) bool {
	return l == AnyZeros || level.Valid(l)
}

// LevelFilter keeps the TODOs at one of levels, or with any level when
// levels is empty and anyLevel is set. It is nil when neither is given, so
// every TODO is kept.
func LevelFilter(anyLevel bool, levels []string) func(Match) bool {
	switch {
	case len(levels) > 0:
		return func(match Match) bool {
			for _, want := range levels {
				if want == match.Level || (want == AnyZeros && level.Zeros(match.Level)) {
					return true
				}
			}
			return false
		}
	case anyLevel:
		return func(match Match) bool { return match.Level != "" }
	}
	return nil
}
