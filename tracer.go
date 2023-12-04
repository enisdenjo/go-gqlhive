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

type Tracer struct {
	token      string
	endpoint   string
	generateID GenerateID
	sendReport SendReport
}

var _ interface {
	graphql.HandlerExtension
	graphql.ResponseInterceptor
	graphql.FieldInterceptor
} = Tracer{}

func NewTracer(token string, opts ...TracerOption) *Tracer {
	tracer := &Tracer{
		token:      token,
		endpoint:   defaultEndpoint,
		generateID: defaultGenerateID,
		sendReport: defaultSendReport,
	}
	for _, opt := range opts {
		opt.set(tracer)
	}
	return tracer
}

func (tracer Tracer) ExtensionName() string {
	return "GraphQLHive"
}

func (tracer Tracer) Validate(schema graphql.ExecutableSchema) error {
	// nothing to validate
	return nil
}

// InterceptResponse intercepts the incoming request.
func (tracer Tracer) InterceptResponse(ctx context.Context, next graphql.ResponseHandler) *graphql.Response {
	if !graphql.HasOperationContext(ctx) {
		return next(ctx)
	}
	operationCtx := graphql.GetOperationContext(ctx)
	if operationCtx.Operation == nil {
		return next(ctx)
	}

	operationStart := operationCtx.Stats.OperationStart
	operation := &OperationWithInfo{
		Operation: Operation{
			Operation:     operationCtx.RawQuery,
			OperationName: nullable.TrimmedStringFrom(operationCtx.OperationName),
			Fields:        createFieldsForOperation(operationCtx.Operation.SelectionSet),
		},
		OperationInfo: OperationInfo{
			ID:        tracer.generateID(operationCtx.RawQuery, nullable.TrimmedStringFrom(operationCtx.OperationName)),
			Timestamp: operationStart.UnixMilli(),
			Execution: Execution{
				// we assume there are no errors, error checks will happen while intercepting fields
				Ok:          true,
				ErrorsTotal: 0,
			},
			Metadata: Metadata{
				Client: Client{
					Name:    CLIENT_NAME,
					Version: CLIENT_VERSION,
				},
			},
		},
	}
	defer func() {
		operation.Execution.Duration = time.Since(operationStart).Nanoseconds()

		report := &Report{}
		err := AddOperationToReport(report, operation)
		if err != nil {
			// TODO: report gracefully
			panic(err)
		}

		tracer.sendReport(ctx, tracer.endpoint, tracer.token, report)
	}()

	return next(ContextWithOperation(ctx, operation))
}

func (tracer Tracer) InterceptField(ctx context.Context, next graphql.Resolver) (interface{}, error) {
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
	var visitField func(selSet ast.SelectionSet)
	var visitValue func(value *ast.Value)
	visitField = func(selSet ast.SelectionSet) {
		first := true
		for _, sel := range selSet {
			field := sel.(*ast.Field)
			if first {
				fields = append(fields, field.ObjectDefinition.Name)
				first = false
			}
			fields = append(fields,
				fmt.Sprintf("%s.%s", field.ObjectDefinition.Name, field.Name),
			)
			for _, arg := range field.Arguments {
				fields = append(fields,
					fmt.Sprintf("%s.%s.%s", field.ObjectDefinition.Name, field.Name, arg.Name),
					arg.Value.Definition.Name,
				)
				visitValue(arg.Value)
			}
			visitField(field.SelectionSet)
		}
	}
	visitValue = func(value *ast.Value) {
		if len(value.Children) == 0 && value.Definition.Kind == ast.Enum {
			// single enum
			fields = append(fields,
				fmt.Sprintf("%s.%s", value.Definition.Name, value.Raw),
			)
			return
		}

		for _, child := range value.Children {
			if value.Definition.Kind == ast.Enum {
				// list of enums
				fields = append(fields,
					fmt.Sprintf("%s.%s", value.Definition.Name, child.Value.Raw),
				)
				continue
			}

			fields = append(fields,
				fmt.Sprintf("%s.%s", value.Definition.Name, child.Name),
			)
			visitValue(child.Value)
		}
	}
	visitField(rootSelectionSet)
	return fields
}
