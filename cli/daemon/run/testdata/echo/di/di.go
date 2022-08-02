package di

import (
	"context"
)

type Response struct {
	Msg string
}

//encore:service
type Service struct {
	Msg string
}

//encore:api public path=/di/one
func (s *Service) One(ctx context.Context) error {
	return nil
}

//encore:api public path=/di/two
func (s *Service) Two(ctx context.Context) (*Response, error) {
	return &Response{Msg: s.Msg}, nil
}

func InitService() (*Service, error) {
	return &Service{Msg: "Hello World"}, nil
}
