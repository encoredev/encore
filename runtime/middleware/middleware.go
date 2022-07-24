package middleware

import (
	"context"
	"time"

	encore "encore.dev"
)

type Request struct {
	*encore.Request
	Ctx context.Context

	Next NextFunc
}

type NextFunc func(Request) Response

type Response struct {
	Payload  any
	Err      error
	Duration time.Duration
}

type Signature func(req Request, next NextFunc) Response
