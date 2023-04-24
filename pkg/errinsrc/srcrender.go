package errinsrc

import (
	"bufio"
	"bytes"
	"fmt"
	"math"
	"strings"
	"unicode"

	"github.com/alecthomas/chroma/quick"
	"github.com/jwalton/go-supportscolor"
	auroraPkg "github.com/logrusorgru/aurora/v3"
	"github.com/rs/zerolog/log"

	. "encr.dev/pkg/errinsrc/internal"
)

const grayLevelOnLineNumbers = 12
const endEscape = "\x1b[0m"

const tabSize = 4

var aurora auroraPkg.Aurora
var enableColors bool

func init() {
	ColoursInErrors(supportscolor.Stdout().SupportsColor)
}

func ColoursInErrors(enabled bool) {
	enableColors = enabled
	aurora = auroraPkg.NewAurora(enabled)
}

// renderSrc returns the lines of code surrounding the location with a pointer to the error on the error line
func renderSrc(builder *strings.Builder, causes SrcLocations) {
	const linesBeforeError = 2
	const linesAfterError = 2

	idx := 0
	currentCause := causes[idx]
	lastEnd := causes[len(causes)-1].End

	// Check if any of the causes are multiline
	multilineSpace := 0
	for _, cause := range causes {
		if cause.Start.Line != cause.End.Line {
			multilineSpace = 4
			break
		}
	}

	numDigitsInLineNumbers := int(math.Log10(float64(lastEnd.Line+linesAfterError+1))) + 1
	lineNumberFmt := fmt.Sprintf(" %%%dd %c ", numDigitsInLineNumbers, set.VerticalBar)

	// Render the filename
	builder.WriteString(strings.Repeat(" ", numDigitsInLineNumbers+1))
	builder.WriteString(aurora.Gray(grayLevelOnLineNumbers, fmt.Sprintf(" %c%c%c", set.LeftTop, set.HorizontalBar, set.LeftBracket)).String())
	// Note the space on both sides of this string is important
	// as it allows editors (such as GoLand) to pickup the filename in
	// terminal output and convert it into a clickable link into the code
	builder.WriteString(aurora.Cyan(fmt.Sprintf(" %s:%d:%d ",
		causes[0].File.RelPath,
		causes[0].Start.Line,
		causes[0].Start.Col,
	)).String())
	builder.WriteString(aurora.Gray(grayLevelOnLineNumbers, string(set.RightBracket)).String())
	builder.WriteRune('\n')
	builder.WriteString(strings.Repeat(" ", numDigitsInLineNumbers+2))
	builder.WriteString(aurora.Gray(grayLevelOnLineNumbers, fmt.Sprintf("%c", set.VerticalBar)).String())
	builder.WriteRune('\n')

	var currentLine int
	gapRenderedUntil := currentCause.Start.Line
	bBuffer := new(bytes.Buffer)
	bBuffer.Write(causes[0].File.Contents)
	sc := bufio.NewScanner(bBuffer)
linePrintLoop:
	for sc.Scan() {
		currentLine++

		if currentLine >= currentCause.Start.Line-linesBeforeError && currentLine <= currentCause.End.Line+linesAfterError {
			// Write the line number
			builder.WriteString(aurora.Gray(grayLevelOnLineNumbers, fmt.Sprintf(lineNumberFmt, currentLine)).String())

			// If this is a multiline error, then render the gutters
			if multilineSpace > 0 {
				if currentCause.Start.Line <= currentLine && currentLine <= currentCause.End.Line {
					line := fmt.Sprintf("%c   ", set.VerticalBar)

					switch currentLine {
					case currentCause.Start.Line:
						if currentCause.Start.Col == 1 {
							line = fmt.Sprintf("%c%c%c ", set.LeftTop, set.HorizontalBar, set.RightArrow)
						} else {
							line = strings.Repeat(" ", multilineSpace)
						}
					case currentCause.End.Line:
						if currentCause.End.Col == 1 {
							line = fmt.Sprintf("%c%c%c ", set.LeftCross, set.HorizontalBar, set.RightArrow)
						}
					}

					switch currentCause.Type {
					case LocError:
						builder.WriteString(aurora.BrightRed(line).String())
					case LocWarning:
						builder.WriteString(aurora.BrightYellow(line).String())
					case LocHelp:
						builder.WriteString(aurora.BrightBlue(line).String())
					}
				} else {
					builder.WriteString(strings.Repeat(" ", multilineSpace))
				}
			}

			unifiedLine := replaceTabsWithSpaces(sc.Text())

			// Then the line of code itself (attempting to highlight the syntax)
			// Note: we always use the "go" lexer, as it works nicely on CUE files too
			var subBuilder strings.Builder
			if err := quick.Highlight(&subBuilder, unifiedLine, "go", "terminal256", "monokai"); err != nil || !enableColors {
				if err != nil {
					log.Error().AnErr("error", err).Msg("Unable to highlight line of code")
				}
				builder.WriteString(unifiedLine)
			} else {
				syntaxHighlightedLine := subBuilder.String()

				// There's a bug in the quick.Highlight function that causes it sometimes add an extra line feed before the endEscape squence
				// so this if statement removes it
				if strings.HasSuffix(syntaxHighlightedLine, endEscape) {
					syntaxHighlightedLine = strings.TrimRight(syntaxHighlightedLine[:len(syntaxHighlightedLine)-len(endEscape)], " \n\r\t") + endEscape
				}
				builder.WriteString(syntaxHighlightedLine)
			}
			builder.WriteRune('\n')
		} else if currentLine >= gapRenderedUntil && idx < len(causes) {
			gapRenderedUntil = currentCause.Start.Line

			// render two spaces for a break in the file
			for i := 0; i < 2; i++ {
				lineNumberSpacer(builder, numDigitsInLineNumbers, set.VerticalBreak)
				builder.WriteString("\n")
			}
		}

		errorRendered := true
		colOffset := 0
		for errorRendered {
			errorRendered = false

			if currentCause.Start.Line < currentCause.End.Line {
				// If a multiline error then render the currentCause.Start() and currentCause.End() pointers
				color := aurora.BrightRed
				switch currentCause.Type {
				case LocWarning:
					color = aurora.BrightYellow
				case LocHelp:
					color = aurora.BrightBlue
				}
				switch currentLine {
				case currentCause.Start.Line:
					if currentCause.Start.Col > 1 {
						charCount := calcNumberCharactersForColumnNumber(sc.Text(), currentCause.Start.Col)

						lineNumberSpacer(builder, numDigitsInLineNumbers, set.VerticalGap)
						builder.WriteString(color(fmt.Sprintf("%s%c", strings.Repeat(" ", charCount+3), set.UpArrow)).String())
						builder.WriteString("\n")

						lineNumberSpacer(builder, numDigitsInLineNumbers, set.VerticalGap)
						builder.WriteString(color(fmt.Sprintf("%c%s%c", set.LeftTop, strings.Repeat(string(set.HorizontalBar), charCount+2), set.RightBottom)).String())
						builder.WriteString("\n")
					}
				case currentCause.End.Line:
					if currentCause.End.Col > 1 {
						charCount := calcNumberCharactersForColumnNumber(sc.Text(), currentCause.End.Col)

						lineNumberSpacer(builder, numDigitsInLineNumbers, set.VerticalGap)
						builder.WriteString(color(fmt.Sprintf("%c%s%c", set.VerticalBar, strings.Repeat(" ", charCount+2), set.UpArrow)).String())
						builder.WriteString("\n")

						lineNumberSpacer(builder, numDigitsInLineNumbers, set.VerticalGap)
						builder.WriteString(color(fmt.Sprintf("%c%s%c", set.LeftCross, strings.Repeat(string(set.HorizontalBar), charCount+2), set.RightBottom)).String())
						builder.WriteString("\n")
					}

					// And on the final line also render the error message
					renderErrorText(builder, 0, numDigitsInLineNumbers, "", 2, currentCause.Type, currentCause.Text, nil, true, true)
					errorRendered = true
				}
			} else if currentLine == currentCause.End.Line {
				var startCol, endCol int
				startCol = currentCause.Start.Col
				endCol = currentCause.End.Col

				// Try and guess the atom where the error is
				// if the currentCause.Start()/currentCause.End() point is the same position
				if endCol <= startCol {
					endCol = guessEndColumn(sc.Text(), startCol)
				}

				// Work out how long the indicator is
				indicatorLength := endCol - startCol
				if indicatorLength <= 1 {
					indicatorLength = 1
				}

				// Create out the error lines
				errorLines := []string{""}
				errorTextStart := indicatorLength + 1

				if indicatorLength >= 2 {
					half := float64(indicatorLength-1) / 2
					errorTextStart = int(math.Floor(half)) + 2
					if currentCause.Text != "" {
						errorLines[0] = fmt.Sprintf(
							"%s%c%s",
							strings.Repeat(string(set.HorizontalBar), int(math.Floor(half))),
							set.MiddleTop,
							strings.Repeat(string(set.HorizontalBar), int(math.Ceil(half))),
						)
					} else {
						errorLines[0] = strings.Repeat(string(set.HorizontalBar), indicatorLength)
					}
				} else {
					errorLines[0] = string(set.UpArrow)
				}

				renderNewLine := true
				if currentCause.Text == "" {
					if idx+1 < len(causes) {
						nextCause := causes[idx+1]
						if nextCause.Start.Line == currentLine && nextCause.End.Line == currentLine {
							renderNewLine = false
						}
					}
				}

				renderGutter := colOffset == 0

				renderErrorText(builder, startCol-colOffset, numDigitsInLineNumbers, sc.Text(), errorTextStart, currentCause.Type, currentCause.Text, errorLines, renderNewLine, renderGutter)
				errorRendered = true
				colOffset = endCol
			}

			if errorRendered {
				idx = idx + 1
				if idx < len(causes) {
					currentCause = causes[idx]
				} else {
					// stop looking for errors on this line
					break
				}
			}

			if currentLine > lastEnd.Line+linesAfterError {
				// stop printing al errors
				break linePrintLoop
			}
		}
	}

	builder.WriteString(aurora.Gray(grayLevelOnLineNumbers, strings.Repeat(string(set.HorizontalBar), numDigitsInLineNumbers+2)+string(set.RightBottom)).String())
	builder.WriteRune('\n')
}

func lineNumberSpacer(builder *strings.Builder, numDigitsInLineNumbers int, r rune) {
	builder.WriteString(strings.Repeat(" ", numDigitsInLineNumbers+1))
	builder.WriteString(aurora.Gray(grayLevelOnLineNumbers, fmt.Sprintf(" %c ", r)).String())
}

func replaceTabsWithSpaces(line string) string {
	var str strings.Builder
	var col int

	for _, r := range line {
		if r == '\t' {
			// Tabs always count as at least 1 space
			str.WriteRune(' ')
			col++

			// We then align onto the next tab column
			for col%tabSize != 0 {
				str.WriteRune(' ')
				col++
			}
		} else {
			str.WriteRune(r)
			col++
		}
	}

	return str.String()
}

// calcNumberCharactersForColumnNumber calculates the number of monospaced characters we need
// to render the given column number on the line - accounting for tab characters
func calcNumberCharactersForColumnNumber(line string, col int) int {
	count := 0
	for i, r := range line {
		if r == '\t' {
			count++
			for count%tabSize != 0 {
				count++
			}
		} else {
			count++
		}

		if i+1 >= col {
			break
		}

	}
	return count
}

func renderErrorText(builder *strings.Builder, startCol int, numDigitsInLineNumbers int, srcLine string, errorTextStart int, typ LocationType, text string, errorLines []string, renderNewLine bool, renderGutter bool) {
	if text != "" {
		lines := splitTextOnWords(text, errorTextStart)
		for i, line := range lines {
			prefix := strings.Repeat(" ", errorTextStart+1)
			if i == 0 {
				prefix = fmt.Sprintf("%s%c%c ", strings.Repeat(" ", errorTextStart-2), set.LeftBottom, set.HorizontalBar)
			}

			errorLines = append(errorLines, prefix+line)
		}
	}

	var prefixWhitespace string

	// It's possible the start column references generated code; in that case reset
	// the column information as a fallback to prevent panics below.
	if startCol > len(srcLine) {
		startCol = 0
	} else {
		// Compute the whitespace prefix we need on each line
		// (Note this will render tabs as tabs still if they are present)
		prefixWhitespace = strings.Repeat(" ", calcNumberCharactersForColumnNumber(srcLine, startCol-1))
	}

	// Now write the error lines
	for _, line := range errorLines {
		if renderGutter {
			lineNumberSpacer(builder, numDigitsInLineNumbers, set.VerticalGap)
		}
		builder.WriteString(prefixWhitespace)

		switch typ {
		case LocError:
			builder.WriteString(aurora.BrightRed(line).String())
		case LocWarning:
			builder.WriteString(aurora.BrightYellow(line).String())
		case LocHelp:
			builder.WriteString(aurora.BrightBlue(line).String())
		}

		if renderNewLine || len(errorLines) > 1 {
			builder.WriteString("\n")
		}
	}
}

func splitTextOnWords(text string, startingCol int) (rtn []string) {
	text = strings.TrimSpace(text)
	maxLineLength := TerminalWidth - startingCol
	if maxLineLength < 20 {
		maxLineLength = 20
	}

	for _, line := range strings.Split(text, "\n") {
		if len(line) <= maxLineLength {
			rtn = append(rtn, line)
			continue
		}

		lineStart := 0
		lastSpace := 0
		for i := 0; i < len(line); i++ {
			if unicode.IsSpace(rune(line[i])) {
				if i-lineStart >= maxLineLength {
					rtn = append(rtn, line[lineStart:lastSpace])
					lineStart = lastSpace + 1
				}
				lastSpace = i
			}
		}

		if len(line)-lineStart >= maxLineLength {
			rtn = append(rtn, line[lineStart:lastSpace])
			lineStart = lastSpace + 1
		}
		if lineStart < len(line) {
			rtn = append(rtn, line[lineStart:])
		}

	}
	return
}

func wordWrap(text string, b *strings.Builder) {
	for _, line := range splitTextOnWords(text, 0) {
		b.WriteString(line)
		b.WriteString("\n")
	}
}

func guessEndColumn(line string, startColumn int) int {
	var params, brackets, braces int
	inBackticks := false

	for i := startColumn; i < len(line); i++ {
		switch line[i] {
		case '(':
			params++
		case '[':
			brackets++
		case '{':
			braces++
		case ')':
			params--
			if params <= 0 {
				return i + 1
			}
		case ']':
			brackets--
			if brackets <= 0 {
				return i + 1
			}
		case '}':
			braces--
			if braces <= 0 {
				return i + 1
			}
		case '`':
			inBackticks = !inBackticks
		case ';', ',', ':', '"', '\'':
			if !inBackticks && params == 0 && brackets == 0 && braces == 0 {
				return i + 1
			}
		case ' ', '\t', '\n', '\r':
			if params == 0 && brackets == 0 && braces == 0 {
				return i + 1
			}
		}
	}

	return len(line) + 1
}
