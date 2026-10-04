package ui

// Wrap is cursor moved by step, -1 or 1, in a list of n rows: past either
// end it wraps around to the other.
func Wrap(cursor, step, n int) int {
	if n <= 0 {
		return 0
	}
	return ((cursor+step)%n + n) % n
}
