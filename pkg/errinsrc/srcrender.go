package errinsrc

import (
	"bufio"
	"bytes"
	"fmt"
	"math"
	"strings"
	"unicode"

	"github.com/alecthomas/chroma/quick"
	auroraPkg "github.com/logrusorgru/aurora/v3"
	"github.com/rs/zerolog/log"

	. "encr.dev/pkg/errinsrc/internal"
)

const grayLevelOnLineNumbers = 12
const endEscape = "\x1b[0m"

var aurora = auroraPkg.NewAurora(true)
var enableColors = true

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
	builder.WriteString(aurora.Cyan(fmt.Sprintf("%s:%d:%d",
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

			// Then the line of code itself (attempting to highlight the syntax)
			// Note: we always use the "go" lexer, as it works nicely on CUE files too
			var subBuilder strings.Builder
			if err := quick.Highlight(&subBuilder, sc.Text(), "go", "terminal256", "monokai"); err != nil || !enableColors {
				if err != nil {
					log.Error().AnErr("error", err).Msg("Unable to highlight line of code")
				}
				builder.WriteString(sc.Text())
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

		errorRendered := false
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
					lineNumberSpacer(builder, numDigitsInLineNumbers, set.VerticalGap)
					builder.WriteString(color(fmt.Sprintf("%s%c", strings.Repeat(" ", currentCause.Start.Col+3), set.UpArrow)).String())
					builder.WriteString("\n")

					lineNumberSpacer(builder, numDigitsInLineNumbers, set.VerticalGap)
					builder.WriteString(color(fmt.Sprintf("%c%s%c", set.LeftTop, strings.Repeat(string(set.HorizontalBar), currentCause.Start.Col+2), set.RightBottom)).String())
					builder.WriteString("\n")
				}
			case currentCause.End.Line:
				if currentCause.End.Col > 1 {
					lineNumberSpacer(builder, numDigitsInLineNumbers, set.VerticalGap)
					builder.WriteString(color(fmt.Sprintf("%c%s%c", set.VerticalBar, strings.Repeat(" ", currentCause.End.Col+2), set.UpArrow)).String())
					builder.WriteString("\n")

					lineNumberSpacer(builder, numDigitsInLineNumbers, set.VerticalGap)
					builder.WriteString(color(fmt.Sprintf("%c%s%c", set.LeftCross, strings.Repeat(string(set.HorizontalBar), currentCause.End.Col+2), set.RightBottom)).String())
					builder.WriteString("\n")
				}

				// And on the final line also render the error message
				renderErrorText(builder, 1, numDigitsInLineNumbers, sc.Text(), 2, currentCause.Type, currentCause.Text, nil)
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

			renderErrorText(builder, startCol, numDigitsInLineNumbers, sc.Text(), errorTextStart, currentCause.Type, currentCause.Text, errorLines)
			errorRendered = true
		}

		if errorRendered {
			idx = idx + 1
			if idx < len(causes) {
				currentCause = causes[idx]
			}
		}

		if currentLine > lastEnd.Line+linesAfterError {
			break
		}
	}

	builder.WriteString(aurora.Gray(grayLevelOnLineNumbers, strings.Repeat(string(set.HorizontalBar), numDigitsInLineNumbers+2)+string(set.RightBottom)).String())
	builder.WriteRune('\n')
}

func lineNumberSpacer(builder *strings.Builder, numDigitsInLineNumbers int, r rune) {
	builder.WriteString(strings.Repeat(" ", numDigitsInLineNumbers+1))
	builder.WriteString(aurora.Gray(grayLevelOnLineNumbers, fmt.Sprintf(" %c ", r)).String())
}

func renderErrorText(builder *strings.Builder, startCol int, numDigitsInLineNumbers int, srcLine string, errorTextStart int, typ LocationType, text string, errorLines []string) {
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

	// Compute the whitespace prefix we need on each line
	// (Note this will render tabs as tabs still if they are present)
	prefixWhitespace := ""
	for i := 0; i < startCol-1; i++ {
		switch {
		case unicode.IsSpace(rune(srcLine[i])):
			prefixWhitespace += string(srcLine[i])
		default:
			prefixWhitespace += " "
		}
	}

	// Now write the error lines
	for _, line := range errorLines {
		lineNumberSpacer(builder, numDigitsInLineNumbers, set.VerticalGap)
		builder.WriteString(prefixWhitespace)

		switch typ {
		case LocError:
			builder.WriteString(aurora.BrightRed(line).String())
		case LocWarning:
			builder.WriteString(aurora.BrightYellow(line).String())
		case LocHelp:
			builder.WriteString(aurora.BrightBlue(line).String())
		}

		builder.WriteString("\n")
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
			if !inBackticks {
				return i + 1
			}
		case ' ', '\t', '\n', '\r':
			return i + 1
		}
	}

	return len(line) + 1
}
