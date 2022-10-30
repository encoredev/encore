// Package ansi provides helper functions for writing ANSI terminal escape codes.
package ansi

import "fmt"

// SetCursorPosition returns the ANSI escape code for setting the cursor position.
// The rows and columns are one-based. If <=0 they default to the first row/column.
func SetCursorPosition(row, col int) string {
	if row <= 0 {
		row = 1
	}
	if col <= 0 {
		col = 1
	}
	return fmt.Sprintf("\u001b[%d;%dH", row, col)
}

type ClearScreenMethod int

const (
	CursorToBottom           ClearScreenMethod = 0
	CursorToTop              ClearScreenMethod = 1
	WholeScreen              ClearScreenMethod = 2
	WholeScreenAndScrollback ClearScreenMethod = 3
)

// ClearScreen clears the screen according to the given method.
func ClearScreen(method ClearScreenMethod) string {
	return fmt.Sprintf("\u001b[%dJ", method)
}

type ClearLineMethod int

const (
	CursorToEnd   ClearLineMethod = 0 // cursor to end of line
	CursorToStart ClearLineMethod = 1 // cursor to start of line
	WholeLine     ClearLineMethod = 2
)

// ClearLine clears the current line according to the given method.
// The cursor position within the line does not change.
func ClearLine(method ClearLineMethod) string {
	return fmt.Sprintf("\u001b[%dK", method)
}

const (
	// SaveCursorPosition saves the current cursor position.
	SaveCursorPosition = "\u001b7"
	// RestoreCursorPosition restores the cursor position to the saved position.
	RestoreCursorPosition = "\u001b8"
)

// MoveCursorLeft moves the cursor left n cells.
// If the cursor is already at the edge of the screen it has no effect.
// If n is negative it moves to the right instead.
func MoveCursorLeft(n int) string {
	if n < 0 {
		return MoveCursorRight(-n)
	}
	return fmt.Sprintf("\u001b[%dD", n)
}

// MoveCursorRight moves the cursor right n cells.
// If the cursor is already at the edge of the screen it has no effect.
// If n is negative it moves to the left instead.
func MoveCursorRight(n int) string {
	if n < 0 {
		return MoveCursorLeft(-n)
	}
	return fmt.Sprintf("\u001b[%dC", n)
}
