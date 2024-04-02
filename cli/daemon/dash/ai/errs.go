package ai

import (
	"errors"
	"go/scanner"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"

	"encr.dev/pkg/fns"
	"encr.dev/v2/internals/perr"
)

type CodeType string

const (
	CodeTypeEndpoint CodeType = "endpoint"
	CodeTypeTypes    CodeType = "types"
)

type Pos struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

type ValidationError struct {
	Service  string   `json:"service"`
	Endpoint string   `json:"endpoint"`
	CodeType CodeType `json:"codeType"`
	Message  string   `json:"message"`
	Start    *Pos     `json:"start,omitempty"`
	End      *Pos     `json:"end,omitempty"`
}

func formatSrcErrList(overlays *overlays, list *perr.List) ([]ValidationError, error) {
	var rtn []ValidationError
	for i := 0; i < list.Len(); i++ {
		err := list.At(i)
		if err.Params.Locations == nil {
			return nil, err
		}
		rtn = append(rtn, overlays.validationError(err)...)
	}
	return rtn, nil
}

func formatError(info *overlay, err error) []ValidationError {
	if err == nil {
		return nil
	}
	var list scanner.ErrorList
	var pkgErr packages.Error
	if errors.As(err, &list) {
		return fns.Map(list, func(e *scanner.Error) ValidationError {
			return ValidationError{
				Service:  info.service.Name,
				Endpoint: info.endpoint.Name,
				CodeType: info.codeType,
				Message:  e.Msg,
				Start: &Pos{
					Line:   e.Pos.Line - info.headerOffset.Line,
					Column: e.Pos.Column - info.headerOffset.Column,
				},
			}
		})
	} else if errors.As(err, &pkgErr) {
		posParts := strings.Split(pkgErr.Pos, ":")
		var line, col int
		switch len(posParts) {
		case 2:
			line, _ = strconv.Atoi(posParts[1])
		case 3:
			line, _ = strconv.Atoi(posParts[1])
			col, _ = strconv.Atoi(posParts[2])
		}
		return []ValidationError{{
			Service:  info.service.Name,
			Endpoint: info.endpoint.Name,
			CodeType: info.codeType,
			Message:  pkgErr.Msg,
			Start: &Pos{
				Line:   line - info.headerOffset.Line,
				Column: col - info.headerOffset.Column,
			},
		}}

	} else {
		return []ValidationError{{Message: err.Error()}}
	}
}
