package di

import (
	"context"
	"errors"
)

type Response struct {
	Msg string
}

type None struct{}

//encore:api public path=/di/one
func (s *None) One(ctx context.Context) error {
	return nil
}

type Simple struct {
	Msg string
}

//encore:api public path=/di/two
func (s *Simple) Two(ctx context.Context) (*Response, error) {
	return &Response{Msg: s.Msg}, nil
}

func NewSimple() *Simple {
	return &Simple{Msg: "simple"}
}

type Complex struct {
	Msg string
}

//encore:api public path=/di/three
func (s *Complex) Three(ctx context.Context) (*Response, error) {
	return &Response{Msg: s.Msg}, nil
}

var ctr int

func NewComplex() (*Complex, error) {
	ctr++
	if ctr == 1 {
		return nil, errors.New("temporary error")
	}
	return &Complex{Msg: "complex"}, nil
}
