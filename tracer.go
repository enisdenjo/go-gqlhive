package gqlhive

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/domonda/go-types/nullable"
	"github.com/vektah/gqlparser/v2/ast"
)

type Tracer struct{}

var _ interface {
	graphql.HandlerExtension
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

// InterceptResponse intercepts the incoming request.
func (a Tracer) InterceptResponse(ctx context.Context, next graphql.ResponseHandler) *graphql.Response {
	// Some errors can happen outside of an operation so we need to check whether an operation was executed
	if !graphql.HasOperationContext(ctx) {
		return next(ctx)
	}
	operationCtx := graphql.GetOperationContext(ctx)

	operationStart := operationCtx.Stats.OperationStart
	operation := NewOperationWithInfo(operationCtx.RawQuery, nullable.TrimmedStringFrom(operationCtx.OperationName), operationStart, createFieldsForOperation(operationCtx.Operation.SelectionSet))
	defer func() {
		operation.Execution.Duration = time.Since(operationStart).Nanoseconds()

		// TODO: queue operation for reporting
	}()

	// we assume there are no errors, error checks will happen while intercepting fields
	operation.Execution.Ok = true
	operation.Execution.ErrorsTotal = 0

	return next(ContextWithOperation(ctx, operation))
}

func (a Tracer) InterceptField(ctx context.Context, next graphql.Resolver) (interface{}, error) {
	fieldCtx := graphql.GetFieldContext(ctx)

	operation, exists := OperationFromContext(ctx)
	if !exists {
		return nil, errors.New("operation doesn't exist in context")
	}

	res, err := next(ctx)
	if err != nil {
		operation.Execution.Ok = false
		operation.Execution.ErrorsTotal += 1
	}

	errList := graphql.GetFieldErrors(ctx, fieldCtx)
	if len(errList) != 0 {
		operation.Execution.Ok = false
		operation.Execution.ErrorsTotal += len(errList)
	}

	return res, err
}

func createFieldsForOperation(rootSelectionSet ast.SelectionSet) (fields []string) {
	var visit func(selSet ast.SelectionSet)
	visit = func(selSet ast.SelectionSet) {
		first := true
		for _, sel := range selSet {
			field := sel.(*ast.Field)
			if first {
				fields = append(fields, field.ObjectDefinition.Name)
				first = false
			}
			fields = append(fields, fmt.Sprintf("%s.%s", field.ObjectDefinition.Name, field.Name))
			visit(field.SelectionSet)
		}
	}
	visit(rootSelectionSet)
	return fields
}
