// Package level reads todo-system's priority levels: one digit, as in
// todo0 to todo9, or only zeros, as in todo00, where more zeros is more
// urgent.
package level

import (
	"math"
	"strconv"
	"strings"
)

// Valid reports whether l is a level todo-system accepts: one digit or only
// zeros.
func Valid(l string) bool {
	return (len(l) == 1 && l[0] >= '0' && l[0] <= '9') || Zeros(l)
}

// Zeros reports whether l is a level of only zeros, the most urgent kind.
func Zeros(l string) bool {
	return l != "" && strings.Trim(l, "0") == ""
}

// Rank orders levels, most urgent first: more zeros, then 1, 2 and so on,
// then no level.
func Rank(l string) int {
	switch {
	case l == "":
		return math.MaxInt
	case Zeros(l):
		return -len(l)
	}
	n, err := strconv.Atoi(l)
	if err != nil {
		return math.MaxInt - 1
	}
	return n
}
