package main

import (
	"bufio"
	"os"
	"strings"
)

type previewLine struct {
	number int
	text   string
}

func readSourceContext(path string, line int) ([]previewLine, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }() // Read-only, so a close error cannot lose data.
	start := max(line-4, 1)
	end := line + 4
	var result []previewLine
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for number := 1; scanner.Scan() && number <= end; number++ {
		if number >= start {
			result = append(result, previewLine{number: number, text: strings.TrimSuffix(scanner.Text(), "\r")})
		}
	}
	return result, scanner.Err()
}
