package ai

import (
	"fmt"
	"strings"
	"testing"
)

func TestWrapDoc(t *testing.T) {
	var wrapTests = []struct {
		width  int
		string string
	}{
		{1, "Lorem ipsum dolor sit amet"},
		{80, "Lorem ipsum dolor sit amet"},
		{80, "Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum."},
		{80, "Lorem Loremipsumdolorsitamet,consecteturadipiscingelit,seddoeiusmodtemporincididuntutlaboreetdoloremagna"},
		{30, "Loremipsumdolorsitamet,consecteturadipiscingelit,seddoeiusmodtemporincididuntutlaboreetdoloremagna"},
		{80, ""},
		{80, "a\nb\nc\nd"},
	}
	for _, test := range wrapTests {
		t.Run(fmt.Sprintf("WrapDoc(%d, %s)", test.width, test.string), func(t *testing.T) {
			result := wrapDoc(test.string, test.width)
			lines := strings.Split(result, "\n")
			for i, line := range lines {
				if len(line) > test.width && strings.Contains(line, " ") {
					t.Errorf("Line too long: %s", line)
				}
				if i+1 < len(lines) {
					nextWord, _, _ := strings.Cut(lines[i+1], " ")
					if len(line)+len(nextWord) < test.width {
						t.Errorf("Line too short: %s", line)
					}
				}
			}
		})
	}
}
