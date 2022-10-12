package errinsrc

// The current character set to use when rendering
var set CharacterSet = unicodeSet

type CharacterSet struct {
	HorizontalBar rune
	VerticalBar   rune
	CrossBar      rune
	VerticalBreak rune
	VerticalGap   rune
	UpArrow       rune
	RightArrow    rune
	LeftTop       rune
	MiddleTop     rune
	RightTop      rune
	LeftBottom    rune
	RightBottom   rune
	MiddleBottom  rune
	LeftBracket   rune
	RightBracket  rune
	LeftCross     rune
	RightCross    rune
	UnderBar      rune
	Underline     rune
}

var unicodeSet = CharacterSet{
	HorizontalBar: '─',
	VerticalBar:   '│',
	CrossBar:      '┼',
	VerticalBreak: '·',
	VerticalGap:   '⋮',
	UpArrow:       '▲',
	RightArrow:    '▶',
	LeftTop:       '╭',
	MiddleTop:     '┬',
	RightTop:      '╮',
	LeftBottom:    '╰',
	MiddleBottom:  '┴',
	RightBottom:   '╯',
	LeftBracket:   '[',
	RightBracket:  ']',
	LeftCross:     '├',
	RightCross:    '┤',
	UnderBar:      '┬',
	Underline:     '─',
}
var asciiSet = CharacterSet{
	HorizontalBar: '-',
	VerticalBar:   '|',
	CrossBar:      '+',
	VerticalBreak: '*',
	VerticalGap:   ':',
	UpArrow:       '^',
	RightArrow:    '>',
	LeftTop:       ',',
	MiddleTop:     'v',
	RightTop:      '.',
	LeftBottom:    '`',
	MiddleBottom:  '^',
	RightBottom:   '\'',
	LeftBracket:   '[',
	RightBracket:  ']',
	LeftCross:     '|',
	RightCross:    '|',
	UnderBar:      '|',
	Underline:     '^',
}
