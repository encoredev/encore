//go:build ignore
// +build ignore

package gorules

import "github.com/quasilyte/go-ruleguard/dsl"

// This is a collection of rules for ruleguard: https://github.com/quasilyte/go-ruleguard

// Remove extra conversions: mdempsky/unconvert
func unconvert(m dsl.Matcher) {
	m.Match("int($x)").Where(m["x"].Type.Is("int") && !m["x"].Const).Report("unnecessary conversion").Suggest("$x")

	m.Match("float32($x)").Where(m["x"].Type.Is("float32") && !m["x"].Const).Report("unnecessary conversion").Suggest("$x")
	m.Match("float64($x)").Where(m["x"].Type.Is("float64") && !m["x"].Const).Report("unnecessary conversion").Suggest("$x")

	// m.Match("byte($x)").Where(m["x"].Type.Is("byte")).Report("unnecessary conversion").Suggest("$x")
	// m.Match("rune($x)").Where(m["x"].Type.Is("rune")).Report("unnecessary conversion").Suggest("$x")
	m.Match("bool($x)").Where(m["x"].Type.Is("bool") && !m["x"].Const).Report("unnecessary conversion").Suggest("$x")

	m.Match("int8($x)").Where(m["x"].Type.Is("int8") && !m["x"].Const).Report("unnecessary conversion").Suggest("$x")
	m.Match("int16($x)").Where(m["x"].Type.Is("int16") && !m["x"].Const).Report("unnecessary conversion").Suggest("$x")
	m.Match("int32($x)").Where(m["x"].Type.Is("int32") && !m["x"].Const).Report("unnecessary conversion").Suggest("$x")
	m.Match("int64($x)").Where(m["x"].Type.Is("int64") && !m["x"].Const).Report("unnecessary conversion").Suggest("$x")

	m.Match("uint8($x)").Where(m["x"].Type.Is("uint8") && !m["x"].Const).Report("unnecessary conversion").Suggest("$x")
	m.Match("uint16($x)").Where(m["x"].Type.Is("uint16") && !m["x"].Const).Report("unnecessary conversion").Suggest("$x")
	m.Match("uint32($x)").Where(m["x"].Type.Is("uint32") && !m["x"].Const).Report("unnecessary conversion").Suggest("$x")
	m.Match("uint64($x)").Where(m["x"].Type.Is("uint64") && !m["x"].Const).Report("unnecessary conversion").Suggest("$x")

	m.Match("time.Duration($x)").Where(m["x"].Type.Is("time.Duration") &&
		!m["x"].Node.Is("BasicLit") &&
		!m["x"].Text.Matches("^[0-9_]*$")).Report("unnecessary conversion").Suggest("$x")
}

// Don't use == or != with time.Time
// https://github.com/dominikh/go-tools/issues/47 : Wontfix
func timeeq(m dsl.Matcher) {
	m.Match("$t0 == $t1").Where(m["t0"].Type.Is("time.Time")).Report("using == with time.Time")
	m.Match("$t0 != $t1").Where(m["t0"].Type.Is("time.Time")).Report("using != with time.Time")
	m.Match(`map[$k]$v`).Where(m["k"].Type.Is("time.Time")).Report("map with time.Time keys are easy to misuse")
}

// err but no an error
func errnoterror(m dsl.Matcher) {

	// Would be easier to check for all err identifiers instead, but then how do we get the type from m[] ?

	m.Match(
		"if $*_, err := $x; $err != nil { $*_ } else if $_ { $*_ }",
		"if $*_, err := $x; $err != nil { $*_ } else { $*_ }",
		"if $*_, err := $x; $err != nil { $*_ }",

		"if $*_, err = $x; $err != nil { $*_ } else if $_ { $*_ }",
		"if $*_, err = $x; $err != nil { $*_ } else { $*_ }",
		"if $*_, err = $x; $err != nil { $*_ }",

		"$*_, err := $x; if $err != nil { $*_ } else if $_ { $*_ }",
		"$*_, err := $x; if $err != nil { $*_ } else { $*_ }",
		"$*_, err := $x; if $err != nil { $*_ }",

		"$*_, err = $x; if $err != nil { $*_ } else if $_ { $*_ }",
		"$*_, err = $x; if $err != nil { $*_ } else { $*_ }",
		"$*_, err = $x; if $err != nil { $*_ }",
	).
		Where(m["err"].Text == "err" && !m["err"].Type.Is("error") && m["x"].Text != "recover()").
		Report("err variable not error type")
}

// Identical if and else bodies
func ifbodythenbody(m dsl.Matcher) {
	m.Match("if $*_ { $body } else { $body }").
		Report("identical if and else bodies")

	// Lots of false positives.
	// m.Match("if $*_ { $body } else if $*_ { $body }").
	//	Report("identical if and else bodies")
}

// Odd inequality: A - B < 0 instead of !=
// Too many false positives.
/*
func subtractnoteq(m dsl.Matcher) {
	m.Match("$a - $b < 0").Report("consider $a != $b")
	m.Match("$a - $b > 0").Report("consider $a != $b")
	m.Match("0 < $a - $b").Report("consider $a != $b")
	m.Match("0 > $a - $b").Report("consider $a != $b")
}
*/

// Self-assignment
func selfassign(m dsl.Matcher) {
	m.Match("$x = $x").Report("useless self-assignment")
}

// Odd nested ifs
func oddnestedif(m dsl.Matcher) {
	m.Match("if $x { if $x { $*_ }; $*_ }",
		"if $x == $y { if $x != $y {$*_ }; $*_ }",
		"if $x != $y { if $x == $y {$*_ }; $*_ }",
		"if $x { if !$x { $*_ }; $*_ }",
		"if !$x { if $x { $*_ }; $*_ }").
		Report("odd nested ifs")

	m.Match("for $x { if $x { $*_ }; $*_ }",
		"for $x == $y { if $x != $y {$*_ }; $*_ }",
		"for $x != $y { if $x == $y {$*_ }; $*_ }",
		"for $x { if !$x { $*_ }; $*_ }",
		"for !$x { if $x { $*_ }; $*_ }").
		Report("odd nested for/ifs")
}

// odd bitwise expressions
func oddbitwise(m dsl.Matcher) {
	m.Match("$x | $x",
		"$x | ^$x",
		"^$x | $x").
		Report("odd bitwise OR")

	m.Match("$x & $x",
		"$x & ^$x",
		"^$x & $x").
		Report("odd bitwise AND")

	m.Match("$x &^ $x").
		Report("odd bitwise AND-NOT")
}

// odd sequence of if tests with return
func ifreturn(m dsl.Matcher) {
	m.Match("if $x { return $*_ }; if $x {$*_ }").Report("odd sequence of if test")
	m.Match("if $x { return $*_ }; if !$x {$*_ }").Report("odd sequence of if test")
	m.Match("if !$x { return $*_ }; if $x {$*_ }").Report("odd sequence of if test")
	m.Match("if $x == $y { return $*_ }; if $x != $y {$*_ }").Report("odd sequence of if test")
	m.Match("if $x != $y { return $*_ }; if $x == $y {$*_ }").Report("odd sequence of if test")

}

func oddifsequence(m dsl.Matcher) {
	/*
		m.Match("if $x { $*_ }; if $x {$*_ }").Report("odd sequence of if test")

		m.Match("if $x == $y { $*_ }; if $y == $x {$*_ }").Report("odd sequence of if tests")
		m.Match("if $x != $y { $*_ }; if $y != $x {$*_ }").Report("odd sequence of if tests")

		m.Match("if $x < $y { $*_ }; if $y > $x {$*_ }").Report("odd sequence of if tests")
		m.Match("if $x <= $y { $*_ }; if $y >= $x {$*_ }").Report("odd sequence of if tests")

		m.Match("if $x > $y { $*_ }; if $y < $x {$*_ }").Report("odd sequence of if tests")
		m.Match("if $x >= $y { $*_ }; if $y <= $x {$*_ }").Report("odd sequence of if tests")
	*/
}

// odd sequence of nested if tests
func nestedifsequence(m dsl.Matcher) {
	/*
		m.Match("if $x < $y { if $x >= $y {$*_ }; $*_ }").Report("odd sequence of nested if tests")
		m.Match("if $x <= $y { if $x > $y {$*_ }; $*_ }").Report("odd sequence of nested if tests")
		m.Match("if $x > $y { if $x <= $y {$*_ }; $*_ }").Report("odd sequence of nested if tests")
		m.Match("if $x >= $y { if $x < $y {$*_ }; $*_ }").Report("odd sequence of nested if tests")
	*/
}

// odd sequence of assignments
func identicalassignments(m dsl.Matcher) {
	m.Match("$x  = $y; $y = $x").Report("odd sequence of assignments")
}

func oddcompoundop(m dsl.Matcher) {
	m.Match("$x += $x + $_",
		"$x += $x - $_").
		Report("odd += expression")

	m.Match("$x -= $x + $_",
		"$x -= $x - $_").
		Report("odd -= expression")
}

func constswitch(m dsl.Matcher) {
	m.Match("switch $x { $*_ }", "switch $*_; $x { $*_ }").
		Where(m["x"].Const && !m["x"].Text.Matches(`^runtime\.`)).
		Report("constant switch")
}

// oddcomparisons flags comparisons which all have simpler
// equivalents with just $x and $y and no zero term
func oddcomparisons(m dsl.Matcher) {
	m.Match("$x - $y == 0").
		Report("odd comparison").
		Suggest("$x == $y")

	m.Match("$x - $y != 0").
		Report("odd comparison").
		Suggest("$x != $y")

	m.Match("$x - $y < 0").
		Report("odd comparison").
		Suggest("$y > $x")

	m.Match("$x - $y <= 0").
		Report("odd comparison").
		Suggest("$y >= $x")

	m.Match("$x - $y > 0").
		Report("odd comparison").
		Suggest("$x > $y")

	m.Match("$x - $y >= 0").
		Report("odd comparison").
		Suggest("$x >= $y")

	m.Match("$x ^ $y == 0").
		Report("odd comparison").
		Suggest("$x == $y")

	m.Match("$x ^ $y != 0").
		Report("odd comparison").
		Suggest("$x != $y")
}

func oddmathbits(m dsl.Matcher) {
	m.Match(
		"64 - bits.LeadingZeros64($x)",
		"32 - bits.LeadingZeros32($x)",
		"16 - bits.LeadingZeros16($x)",
		"8 - bits.LeadingZeros8($x)",
	).Report("odd math/bits expression: use bits.Len*() instead?")
}

func floateq(m dsl.Matcher) {
	m.Match(
		"$x == $y",
		"$x != $y",
	).
		Where(m["x"].Type.Is("float32") && !m["x"].Const && !m["y"].Text.Matches("0(.0+)?")).
		Report("floating point tested for equality")

	m.Match(
		"$x == $y",
		"$x != $y",
	).
		Where(m["x"].Type.Is("float64") && !m["x"].Const && !m["y"].Text.Matches("0(.0+)?")).
		Report("floating point tested for equality")

	m.Match("switch $x { $*_ }", "switch $*_; $x { $*_ }").
		Where(m["x"].Type.Is("float32")).
		Report("floating point as switch expression")

	m.Match("switch $x { $*_ }", "switch $*_; $x { $*_ }").
		Where(m["x"].Type.Is("float64")).
		Report("floating point as switch expression")

}

func badexponent(m dsl.Matcher) {
	m.Match(
		"2 ^ $x",
		"10 ^ $x",
	).
		Report("caret (^) is not exponentiation")
}

func floatloop(m dsl.Matcher) {
	m.Match(
		"for $i := $x; $i < $y; $i += $z { $*_ }",
		"for $i = $x; $i < $y; $i += $z { $*_ }",
	).
		Where(m["i"].Type.Is("float64")).
		Report("floating point for loop counter")

	m.Match(
		"for $i := $x; $i < $y; $i += $z { $*_ }",
		"for $i = $x; $i < $y; $i += $z { $*_ }",
	).
		Where(m["i"].Type.Is("float32")).
		Report("floating point for loop counter")
}

func urlredacted(m dsl.Matcher) {

	m.Match(
		"log.Println($x, $*_)",
		"log.Println($*_, $x, $*_)",
		"log.Println($*_, $x)",
		"log.Printf($*_, $x, $*_)",
		"log.Printf($*_, $x)",

		"log.Println($x, $*_)",
		"log.Println($*_, $x, $*_)",
		"log.Println($*_, $x)",
		"log.Printf($*_, $x, $*_)",
		"log.Printf($*_, $x)",
	).
		Where(m["x"].Type.Is("*url.URL")).
		Report("consider $x.Redacted() when outputting URLs")
}

func sprinterr(m dsl.Matcher) {
	m.Match(`fmt.Sprint($err)`,
		`fmt.Sprintf("%s", $err)`,
		`fmt.Sprintf("%v", $err)`,
	).
		Where(m["err"].Type.Is("error")).
		Report("maybe call $err.Error() instead of fmt.Sprint()?")

}

func largeloopcopy(m dsl.Matcher) {
	m.Match(
		`for $_, $v := range $_ { $*_ }`,
	).
		Where(m["v"].Type.Size > 512).
		Report(`loop copies large value each iteration`)
}

func joinpath(m dsl.Matcher) {
	m.Match(
		`strings.Join($_, "/")`,
		`strings.Join($_, "\\")`,
		"strings.Join($_, `\\`)",
	).
		Report(`did you mean path.Join() or filepath.Join() ?`)
}

func readfull(m dsl.Matcher) {
	m.Match(`$n, $err := io.ReadFull($_, $slice)
                 if $err != nil || $n != len($slice) {
                              $*_
		 }`,
		`$n, $err := io.ReadFull($_, $slice)
                 if $n != len($slice) || $err != nil {
                              $*_
		 }`,
		`$n, $err = io.ReadFull($_, $slice)
                 if $err != nil || $n != len($slice) {
                              $*_
		 }`,
		`$n, $err = io.ReadFull($_, $slice)
                 if $n != len($slice) || $err != nil {
                              $*_
		 }`,
		`if $n, $err := io.ReadFull($_, $slice); $n != len($slice) || $err != nil {
                              $*_
		 }`,
		`if $n, $err := io.ReadFull($_, $slice); $err != nil || $n != len($slice) {
                              $*_
		 }`,
		`if $n, $err = io.ReadFull($_, $slice); $n != len($slice) || $err != nil {
                              $*_
		 }`,
		`if $n, $err = io.ReadFull($_, $slice); $err != nil || $n != len($slice) {
                              $*_
		 }`,
	).Report("io.ReadFull() returns err == nil iff n == len(slice)")
}

func nilerr(m dsl.Matcher) {
	m.Match(
		`if err == nil { return err }`,
		`if err == nil { return $*_, err }`,
	).
		Report(`return nil error instead of nil value`)

}

func mailaddress(m dsl.Matcher) {
	m.Match(
		"fmt.Sprintf(`\"%s\" <%s>`, $NAME, $EMAIL)",
		"fmt.Sprintf(`\"%s\"<%s>`, $NAME, $EMAIL)",
		"fmt.Sprintf(`%s <%s>`, $NAME, $EMAIL)",
		"fmt.Sprintf(`%s<%s>`, $NAME, $EMAIL)",
		`fmt.Sprintf("\"%s\"<%s>", $NAME, $EMAIL)`,
		`fmt.Sprintf("\"%s\" <%s>", $NAME, $EMAIL)`,
		`fmt.Sprintf("%s<%s>", $NAME, $EMAIL)`,
		`fmt.Sprintf("%s <%s>", $NAME, $EMAIL)`,
	).
		Report("use net/mail Address.String() instead of fmt.Sprintf()").
		Suggest("(&mail.Address{Name:$NAME, Address:$EMAIL}).String()")

}

func errnetclosed(m dsl.Matcher) {
	m.Match(
		`strings.Contains($err.Error(), $text)`,
	).
		Where(m["text"].Text.Matches("\".*closed network connection.*\"")).
		Report(`String matching against error texts is fragile; use net.ErrClosed instead`).
		Suggest(`errors.Is($err, net.ErrClosed)`)

}

func hmacnew(m dsl.Matcher) {
	m.Match("hmac.New(func() hash.Hash { return $x }, $_)",
		`$f := func() hash.Hash { return $x }
	$*_
	hmac.New($f, $_)`,
	).Where(m["x"].Pure).
		Report("invalid hash passed to hmac.New()")
}

func readeof(m dsl.Matcher) {
	m.Match(
		`$n, $err = $r.Read($_)
	if $err != nil {
	    return $*_
	}`,
		`$n, $err := $r.Read($_)
	if $err != nil {
	    return $*_
	}`).Where(m["r"].Type.Implements("io.Reader")).
		Report("Read() can return n bytes and io.EOF")
}

func writestring(m dsl.Matcher) {
	m.Match(`io.WriteString($w, string($b))`).
		Where(m["b"].Type.Is("[]byte")).
		Suggest("$w.Write($b)")
}

func fmtfprint(m dsl.Matcher) {
	m.Match(`fmt.Fprint($w, string($b))`).
		Where(m["b"].Type.Is("[]byte")).
		Suggest("$w.Write($b)")
}

func badlock(m dsl.Matcher) {
	// Shouldn't give many false positives without type filter
	// as Lock+Unlock pairs in combination with defer gives us pretty
	// a good chance to guess correctly. If we constrain the type to sync.Mutex
	// then it'll be harder to match embedded locks and custom methods
	// that may forward the call to the sync.Mutex (or other synchronization primitive).

	m.Match(`$mu.Lock(); defer $mu.RUnlock()`).Report(`maybe $mu.RLock() was intended?`)
	m.Match(`$mu.RLock(); defer $mu.Unlock()`).Report(`maybe $mu.Lock() was intended?`)

	// `mu1` and `mu2` are added to make possible report a line where `m2` is used (with a defer)
	m.Match(`$mu1.Lock(); defer $mu2.Lock()`).
		Where(m["mu1"].Text == m["mu2"].Text).
		At(m["mu2"]).
		Report(`maybe defer $mu1.Unlock() was intended?`)
	m.Match(`$mu1.RLock(); defer $mu2.RLock()`).
		Where(m["mu1"].Text == m["mu2"].Text).
		At(m["mu2"]).
		Report(`maybe defer $mu1.RUnlock() was intended?`)
}

func setenvUsedInTests(m dsl.Matcher) {
	m.Match(
		`os.Setenv($key, $val); defer os.Unsetenv($key)`,
		`os.Setenv($key, $val); defer os.Setenv($key, "")`,
	).
		Where(m.File().Name.Matches("_test.go")).
		Report(`should prefer t.Setenv within tests`).
		Suggest(`t.Setenv($key, $val)`)
}

func contextTODO(m dsl.Matcher) {
	m.Match(`context.TODO()`).Report(`consider to use well-defined context`)
}

func wrongerrcall(m dsl.Matcher) {
	m.Match(
		`if $x.Err() != nil { return err }`,
		`if $x.Err() != nil { return $*_, err }`,
	).
		Report(`maybe returning wrong error after Err() call`)
}

// ioutil.Discard => io.Discard
func ioutilDiscard(m dsl.Matcher) {
	m.Match(
		`ioutil.Discard`,
	).
		Report(`As of Go 1.16, this value is simply io.Discard.`).
		Suggest(`io.Discard`)
}

// ioutil.NopCloser => io.NopCloser
func ioutilNopCloser(m dsl.Matcher) {
	m.Match(
		`ioutil.NopCloser($r)`,
	).Where(m["r"].Type.Implements("io.Reader")).
		Report(`As of Go 1.16, this function simply calls io.NopCloser.`).
		Suggest(`io.NopCloser($r)`)
}

// ioutil.ReadAll => io.ReadAll
func ioutilReadAll(m dsl.Matcher) {
	m.Match(
		`ioutil.ReadAll($r)`,
	).Where(m["r"].Type.Implements("io.Reader")).
		Report(`As of Go 1.16, this function simply calls io.ReadAll.`).
		Suggest(`io.ReadAll($r)`)
}

// ioutil.ReadDir => os.ReadDir
func ioutilReadDir(m dsl.Matcher) {
	m.Match(
		`ioutil.ReadDir($d)`,
	).Where(m["d"].Type.Is("string")).
		Report(`As of Go 1.16, os.ReadDir is a more efficient and correct choice: it returns a list of fs.DirEntry instead of fs.FileInfo, and it returns partial results in the case of an error midway through reading a directory.`).
		Suggest(`os.ReadDir($d)`)
}

// ioutil.ReadFile => os.ReadFile
func ioutilReadFile(m dsl.Matcher) {
	m.Match(
		`ioutil.ReadFile($f)`,
	).Where(m["f"].Type.Is("string")).
		Report(`As of Go 1.16, this function simply calls os.ReadFile.`).
		Suggest(`os.ReadFile($f)`)
}

// ioutil.TempDir => os.MkdirTemp
func ioutilTempDir(m dsl.Matcher) {
	m.Match(
		`ioutil.TempDir($d, $p)`,
	).Where(m["d"].Type.Is("string") && m["p"].Type.Is("string")).
		Report(`As of Go 1.17, this function simply calls os.MkdirTemp.`).
		Suggest(`os.MkdirTemp($d, $p)`)
}

// ioutil.TempFile => os.CreateTemp
func ioutilTempFile(m dsl.Matcher) {
	m.Match(
		`ioutil.TempFile($d, $p)`,
	).Where(m["d"].Type.Is("string") && m["p"].Type.Is("string")).
		Report(`As of Go 1.17, this function simply calls os.CreateTemp.`).
		Suggest(`os.CreateTemp($d, $p)`)
}

// ioutil.WriteFile => os.WriteFile
func ioutilWriteFile(m dsl.Matcher) {
	m.Import("io/fs")
	m.Match(
		`ioutil.WriteFile($f, $d, $p)`,
	).Where(m["f"].Type.Is("string") && m["d"].Type.Is("[]byte") && m["p"].Type.Is("fs.FileMode")).
		Report(`As of Go 1.16, this function simply calls os.WriteFile.`).
		Suggest(`os.WriteFile($f, $d, $p)`)
}

func ioWriterWriteMisuse(m dsl.Matcher) {
	// io.Writer.Write([]byte(string)) => io.WriteString(io.Writer, string)
	m.Match(`$w.Write([]byte($s))`).
		Where(m["s"].Type.Is("string") && m["w"].Type.HasMethod("io.Writer.Write") && !m["w"].Type.HasMethod("io.StringWriter.WriteString")).
		Report(`Use io.WriteString when writing a string to an io.Writer`).
		Suggest(`io.WriteString($w, $s)`)

	// interface{ io.Writer; io.StringWriter }.Write([]byte(string)) => interface{ io.Writer; io.StringWriter }.WriteString(string)
	m.Match(`$w.Write([]byte($s))`).
		Where(m["s"].Type.Is("string") && m["w"].Type.HasMethod("io.Writer.Write") && m["w"].Type.HasMethod("io.StringWriter.WriteString")).
		Report(`Use WriteString when writing a string to an io.StringWriter`).
		Suggest(`$w.WriteString($s)`)
}

func ioStringWriterWriteStringMisuse(m dsl.Matcher) {
	// interface{ io.Writer; io.StringWriter }.WriteString(string([]byte)) => interface{ io.Writer; io.StringWriter }.Write([]byte)
	m.Match(`$w.WriteString(string($b))`).
		Where(m["b"].Type.Is("[]byte") && m["w"].Type.HasMethod("io.Writer.Write") && m["w"].Type.HasMethod("io.StringWriter.WriteString")).
		Report(`Use Write when writing a []byte to an io.Writer`).
		Suggest(`$w.Write($b)`)
}
