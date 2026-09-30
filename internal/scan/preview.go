package scan

import (
	"bufio"
	"os"
	"strings"
)

// ContextLine is one numbered source line shown around a match.
type ContextLine struct {
	Number int
	Text   string
}

// ReadContext returns up to radius lines either side of line in the file at
// path.
func ReadContext(path string, line, radius int) ([]ContextLine, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }() // Read-only, so a close error cannot lose data.
	start := max(line-radius, 1)
	end := line + radius
	var result []ContextLine
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for number := 1; scanner.Scan() && number <= end; number++ {
		if number >= start {
			result = append(result, ContextLine{Number: number, Text: strings.TrimSuffix(scanner.Text(), "\r")})
		}
	}
	return result, scanner.Err()
}
