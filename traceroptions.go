package gqlhive

import (
	"context"

	"github.com/domonda/go-types/nullable"
	"github.com/domonda/go-types/uu"
)

const defaultEndpoint = "https://app.graphql-hive.com/usage"

func WithEndpoint(endpoint string) TracerOption {
	return tracerOptionFn(func(tracer *Tracer) {
		tracer.endpoint = endpoint
	})
}

func defaultGenerateID(operation string, operationName nullable.TrimmedString) string {
	return uu.IDv4().String()
}

type GenerateID func(operation string, operationName nullable.TrimmedString) string

func WithGenerateID(fn GenerateID) TracerOption {
	return tracerOptionFn(func(tracer *Tracer) {
		tracer.generateID = fn
	})
}

func defaultSendReport(ctx context.Context, endpoint, token string, report *Report) error {
	panic("TODO")
}

type SendReport func(ctx context.Context, endpoint, token string, report *Report) error

func WithSendReport(fn SendReport) TracerOption {
	return tracerOptionFn(func(tracer *Tracer) {
		tracer.sendReport = fn
	})
}

type TracerOption interface {
	set(*Tracer)
}

type tracerOptionFn func(*Tracer)

func (fn tracerOptionFn) set(config *Tracer) {
	fn(config)
}
