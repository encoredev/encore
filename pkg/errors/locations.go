package errors

import (
	goAst "go/ast"
	goToken "go/token"
)

// LocationType represents if the locaton is the source of the error, a warning or a helpful hint
type LocationType uint8

const (
	LocError LocationType = iota
	LocWarning
	LocHelp
)

// LocationKind tells us what language and position markers we're using to identify the source
type LocationKind uint8

const (
	LocFile LocationKind = iota
	LocGoNode
	LocGoPos
	LocGoPositions
)

// SrcLocation tells us where in the code base caused the error, warning or is a hint to the end
// user.
type SrcLocation struct {
	Kind            LocationKind
	LocType         LocationType
	Text            string
	Filepath        string
	GoNode          goAst.Node
	GoStartPos      goToken.Pos
	GoEndPos        goToken.Pos
	GoStartPosition goToken.Position
	GoEndPosition   goToken.Position
}

type LocationOption func(*SrcLocation)

// AsError allows you to set a SrcLocation's error text
//
// Pass this option in when you give the error [Template] a src location
func AsError(errorText string) func(loc *SrcLocation) {
	return func(loc *SrcLocation) {
		loc.Text = errorText
		loc.LocType = LocError
	}
}

// AsWarning allows you to set a SrcLocation's text and mark it as a warning
//
// Pass this option in when you give the error [Template] a src location
func AsWarning(warningText string) func(loc *SrcLocation) {
	return func(loc *SrcLocation) {
		loc.Text = warningText
		loc.LocType = LocWarning
	}
}

// AsHelp allows you to set a SrcLocation's text and mark it as a a helpful hint
//
// Pass this option in when you give the error [Template] a src location
func AsHelp(helpText string) func(loc *SrcLocation) {
	return func(loc *SrcLocation) {
		loc.LocType = LocHelp
		loc.Text = helpText
	}
}
