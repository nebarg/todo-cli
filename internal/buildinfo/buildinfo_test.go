package buildinfo

import "testing"

func TestVersionIsNotEmpty(t *testing.T) {
	if Version() == "" {
		t.Fatal("Version() is empty")
	}
}
