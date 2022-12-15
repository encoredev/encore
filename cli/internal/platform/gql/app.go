package gql

import (
	"encoding/json"
	"fmt"
)

type App struct {
	ID   string
	Slug string
}

type Error struct {
	Message    string                     `json:"message"`
	Path       []string                   `json:"path"`
	Extensions map[string]json.RawMessage `json:"extensions"`
}

func (e *Error) Error() string {
	return e.Message
}

type ErrorList []*Error

func (err ErrorList) Error() string {
	if len(err) == 0 {
		return "no errors"
	} else if len(err) == 1 {
		return err[0].Error()
	}
	return fmt.Sprintf("%s (and %d more errors)", err[0].Error(), len(err)-1)
}
