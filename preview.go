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
	defer f.Close()
	start := line - 4
	if start < 1 {
		start = 1
	}
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
