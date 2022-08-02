package flakey_di

import (
	"context"
	"errors"
)

type Response struct {
	Msg string
}

//encore:service
type Service struct {
	Msg string
}

//encore:api public path=/di/flakey
func (s *Service) Flakey(ctx context.Context) (*Response, error) {
	return &Response{Msg: s.Msg}, nil
}

var ctr int

func InitService() (*Service, error) {
	ctr++
	if ctr == 1 {
		return nil, errors.New("temporary error")
	}
	return &Service{Msg: "Hello, Flakey World"}, nil
}
