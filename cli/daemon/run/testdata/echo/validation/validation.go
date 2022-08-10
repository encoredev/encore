package validation

import (
	"context"
	"errors"
)

type Request struct {
	Msg string
}

func (req *Request) Validate() error {
	if req.Msg == "fail" {
		return errors.New("bad message")
	}
	return nil
}

//encore:api public
func TestOne(ctx context.Context, msg *Request) error {
	return nil
}
