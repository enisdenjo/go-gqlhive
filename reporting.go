package gqlhive

import (
	"context"

	"github.com/99designs/gqlgen/graphql"
)

type Tracer struct {
	OperationName string
}

var _ interface {
	graphql.HandlerExtension
	graphql.OperationInterceptor
	graphql.ResponseInterceptor
	graphql.FieldInterceptor
} = Tracer{}

func NewTracer() Tracer {
	return Tracer{}
}

func (a Tracer) ExtensionName() string {
	return "GraphQLHive"
}

func (a Tracer) Validate(schema graphql.ExecutableSchema) error {
	// nothing to validate
	return nil
}

func (a Tracer) InterceptOperation(ctx context.Context, next graphql.OperationHandler) graphql.ResponseHandler {
	// oc := graphql.GetOperationContext(ctx)

	return next(ctx)
}

func (a Tracer) InterceptField(ctx context.Context, next graphql.Resolver) (interface{}, error) {
	// fc := graphql.GetFieldContext(ctx)

	return next(ctx)
}

// InterceptResponse intercepts the incoming request.
func (a Tracer) InterceptResponse(ctx context.Context, next graphql.ResponseHandler) *graphql.Response {
	// Some errors can happen outside of an operation so we need to check whether an operation was executed
	if !graphql.HasOperationContext(ctx) {
		return next(ctx)
	}

	// oc := graphql.GetOperationContext(ctx)

	return next(ctx)
}
