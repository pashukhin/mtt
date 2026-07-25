package cli

import (
	"strings"
	"testing"
)

func TestConfirmRemote(t *testing.T) {
	yes, _ := confirmRemote(strings.NewReader("y\n"), true, false)
	if !yes {
		t.Fatal("y must proceed")
	}
	no, _ := confirmRemote(strings.NewReader("\n"), true, false)
	if no {
		t.Fatal("empty must abort")
	}
	auto, _ := confirmRemote(strings.NewReader(""), false, true) // --yes, no read
	if !auto {
		t.Fatal("--yes must proceed without reading")
	}
	refuse, err := confirmRemote(strings.NewReader(""), false, false) // non-TTY, no --yes
	if refuse || err == nil {
		t.Fatal("non-TTY without --yes must refuse with an error")
	}
}
