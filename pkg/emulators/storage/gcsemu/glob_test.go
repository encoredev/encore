package gcsemu

import "testing"

func TestCompileGlob(t *testing.T) {
	tests := []struct {
		pattern string
		match   []string
		noMatch []string
	}{
		{
			pattern: "*.png",
			match:   []string{"a.png", ".png"},
			noMatch: []string{"a.jpg", "dir/a.png", "a.png.bak"},
		},
		{
			pattern: "**.png",
			match:   []string{"a.png", "dir/a.png", "a/b/c.png"},
			noMatch: []string{"a.jpg", "dir/a.png/b"},
		},
		{
			// The shape the dev dash uses for a free-text search term.
			pattern: "**report**",
			match:   []string{"report", "2024/q1-report.pdf", "reports/x", "a/b/annual_report_v2"},
			noMatch: []string{"summary.pdf", "Report"},
		},
		{
			pattern: "photos/*/thumb.jpg",
			match:   []string{"photos/2024/thumb.jpg"},
			noMatch: []string{"photos/thumb.jpg", "photos/2024/06/thumb.jpg"},
		},
		{
			// "**/" at a directory boundary may also match nothing at all, so this
			// covers the folder-less case too. Cloud Storage documents exactly this
			// example: "foo/**/bar can be used to match foo/bar and foo/baz/bar".
			pattern: "photos/**/thumb.jpg",
			match:   []string{"photos/thumb.jpg", "photos/2024/thumb.jpg", "photos/2024/06/thumb.jpg"},
			noMatch: []string{"thumb.jpg", "photos/2024/thumb.jpeg"},
		},
		{
			// The same collapse at the very start of a pattern, which is also a boundary.
			pattern: "**/thumb.jpg",
			match:   []string{"thumb.jpg", "photos/thumb.jpg", "photos/2024/thumb.jpg"},
			noMatch: []string{"athumb.jpg"},
		},
		{
			// Not at a boundary, so no collapse: "**" here is just "any characters".
			pattern: "photos**/thumb.jpg",
			match:   []string{"photos/thumb.jpg", "photosX/thumb.jpg", "photos/a/thumb.jpg"},
			noMatch: []string{"photos/thumbXjpg"},
		},
		{
			// An escaped slash is a literal character with no structural role, so it
			// opens no directory boundary and the "**/" collapse does not apply after
			// one. This is the same reading that lets a brace hold an escaped slash.
			pattern: `photos\/**/thumb.jpg`,
			match:   []string{"photos/a/thumb.jpg"},
			noMatch: []string{"photos/thumb.jpg"},
		},
		{
			pattern: "a?c",
			match:   []string{"abc", "a-c"},
			noMatch: []string{"ac", "abbc", "a/c"},
		},
		{
			pattern: "img[0-9].png",
			match:   []string{"img0.png", "img7.png"},
			noMatch: []string{"imgx.png", "img10.png"},
		},
		{
			pattern: "img[!0-9].png",
			match:   []string{"imgx.png"},
			noMatch: []string{"img0.png", "img/.png"},
		},
		{
			pattern: "img[^0-9].png",
			match:   []string{"imgx.png"},
			noMatch: []string{"img0.png", "img/.png"},
		},
		{
			pattern: "*.{png,jpg}",
			match:   []string{"a.png", "a.jpg"},
			noMatch: []string{"a.gif", "dir/a.png"},
		},
		{
			// "a/y.png" matches: the "**/" follows a slash, so it collapses away.
			pattern: "{a,b}/**/*.{png,jpg}",
			match:   []string{"a/x/y.png", "b/x/y/z.jpg", "a/y.png"},
			noMatch: []string{"c/x/y.png", "a/y.gif"},
		},
		{
			// Regexp metacharacters outside of glob syntax are literal.
			pattern: "a+b.c(d)",
			match:   []string{"a+b.c(d)"},
			noMatch: []string{"aab.c(d)", "a+bxc(d)"},
		},
		{
			pattern: `a\*b`,
			match:   []string{"a*b"},
			noMatch: []string{"axb", "ab"},
		},
		{
			// A comma outside an alternation is literal.
			pattern: "a,b",
			match:   []string{"a,b"},
			noMatch: []string{"a", "b"},
		},
		{
			pattern: "*",
			match:   []string{"a", ""},
			noMatch: []string{"a/b"},
		},
		{
			pattern: "**",
			match:   []string{"a", "", "a/b/c"},
		},
		{
			pattern: "räksmörgås/*.txt",
			match:   []string{"räksmörgås/a.txt"},
			noMatch: []string{"raksmorgas/a.txt"},
		},
	}

	for _, test := range tests {
		re, err := compileGlob(test.pattern)
		if err != nil {
			t.Errorf("compileGlob(%q): unexpected error: %v", test.pattern, err)
			continue
		}
		for _, s := range test.match {
			if !re.MatchString(s) {
				t.Errorf("compileGlob(%q) [%s]: expected %q to match", test.pattern, re, s)
			}
		}
		for _, s := range test.noMatch {
			if re.MatchString(s) {
				t.Errorf("compileGlob(%q) [%s]: expected %q not to match", test.pattern, re, s)
			}
		}
	}
}

func TestCompileGlobInvalid(t *testing.T) {
	for _, pattern := range []string{
		`a\`,
		"a[bc",
		"a{b,c",
		"a}b",
		`a[b\`,

		// Cloud Storage: "Brace expansions ({}) cannot contain slashes (/) or
		// double asterisks (**)."
		"{a/b,c}",
		"{a,b/c}",
		"x/{a,b/c}/y",
		"{a,**}",
		"{a,b**c}",
		"x/{**,a}/y",
	} {
		if _, err := compileGlob(pattern); err == nil {
			t.Errorf("compileGlob(%q): expected an error", pattern)
		}
	}
}

// TestCompileGlobBraceRestrictionsScoped pins that the brace restrictions apply only
// inside braces: a "/" or "**" elsewhere in the same pattern is ordinary syntax.
func TestCompileGlobBraceRestrictionsScoped(t *testing.T) {
	for _, pattern := range []string{
		"{a,b}/**/c",
		"**/{a,b}.txt",
		"x/{a,b}/y",
		`{a\/b,c}`, // escaped, so not a path separator
	} {
		if _, err := compileGlob(pattern); err != nil {
			t.Errorf("compileGlob(%q): %v, want it accepted", pattern, err)
		}
	}
}

// TestEscapeGlobLiteral pins that an escaped string matches itself and nothing else,
// which is what lets a literal key prefix be spliced into a caller's pattern.
func TestEscapeGlobLiteral(t *testing.T) {
	literals := []string{
		"a[1]/", "a{x}/", "a*b/", "a?b/", "a,b/", "a}b/", `a\b/`, "a]b/",
		"plain/", "", "**", "[!a-z]", "räksmörgås[1]/",
	}
	for _, literal := range literals {
		re, err := compileGlob(EscapeGlobLiteral(literal))
		if err != nil {
			t.Errorf("compileGlob(EscapeGlobLiteral(%q)): unexpected error: %v", literal, err)
			continue
		}
		if !re.MatchString(literal) {
			t.Errorf("EscapeGlobLiteral(%q) [%s]: expected it to match itself", literal, re)
		}
		for _, other := range literals {
			if other != literal && re.MatchString(other) {
				t.Errorf("EscapeGlobLiteral(%q) [%s]: also matched %q", literal, re, other)
			}
		}
	}

	// An escaped prefix still composes with an unescaped pattern after it.
	re, err := compileGlob(EscapeGlobLiteral("a[1]/") + "**.png")
	if err != nil {
		t.Fatal(err)
	}
	if !re.MatchString("a[1]/deep/photo.png") {
		t.Errorf("%s did not match an object below the escaped prefix", re)
	}
	if re.MatchString("a1/deep/photo.png") {
		t.Errorf("%s matched a key the literal prefix does not name", re)
	}
}
