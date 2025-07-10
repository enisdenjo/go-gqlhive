package gqlhive

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/domonda/go-types/nullable"
	"github.com/domonda/go-types/uu"
	"github.com/vektah/gqlparser/v2/ast"
)

type Tracer struct {
	target            string
	token             string
	endpoint          string
	generateID        GenerateID
	sendReportTimeout time.Duration
	sendReport        SendReport
	log               Logger
}

var _ interface {
	graphql.HandlerExtension
	graphql.ResponseInterceptor
	graphql.FieldInterceptor
} = Tracer{}

// NewTracer creates a new Hive Console tracer with the given [target] and access [token].
// Read more about it here: https://the-guild.dev/graphql/hive/docs/schema-registry/usage-reporting.
//
//   - target: Is the slug as displayed on the Hive Console dashboard (e.g. `the-guild/graphql-hive/production`) or the UUID of the target (e.g. `a0f4c605-6541-4350-8cfe-b31f21a4bf80`). The UUID is more resilient to organization/project/target slug adjustments. You can find it within the target settings or by hovering over the target slug within the Hive dashboard.
//   - token: Is the access token for the given [target]. Instructions about setting up access tokens can be found in the Hive Console Access Tokens documentation: https://the-guild.dev/graphql/hive/docs/management/access-tokens.
func NewTracer(target, token string, opts ...TracerOption) *Tracer {
	tracer := &Tracer{
		target:            target,
		token:             token,
		endpoint:          defaultEndpoint,
		generateID:        defaultGenerateID,
		sendReportTimeout: defaultSendReportTimeout,
		sendReport:        defaultSendReport,
		log:               defaultLogger,
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
	invalidTargetErr := fmt.Errorf("invalid gqlhive tracer target %q, must be a valid pathname <ORGANIZATION>/<PROJECT>/<TARGET> or an UUID <TARGET_ID>", tracer.target)

	u, _ := url.Parse(tracer.target)
	if u.String() != tracer.target {
		// valid target wont change once url parsed
		return invalidTargetErr
	}

	if strings.Count(tracer.target, "/") != 0 {
		// probably a pathname
		if strings.HasPrefix(tracer.target, "/") {
			return fmt.Errorf("invalid gqlhive tracer target pathname %q, must not start with a slash", tracer.target)
		}
		if strings.Count(tracer.target, "/") != 2 {
			return fmt.Errorf("invalid gqlhive tracer target pathname %q, must contain 3 parts <ORGANIZATION>/<PROJECT>/<TARGET>", tracer.target)
		}
	} else {
		// probably an uuid
		if u := uu.IDFromStringOrNil(tracer.target); u.IsNil() {
			return invalidTargetErr
		}
	}

	if nullable.TrimmedStringFrom(tracer.token).IsNull() {
		return errors.New("gqlhive tracer token must not be empty")
	}

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

		err := queueOperation(operation)
		if err != nil {
			tracer.log.Printf("failed to queue operation %q: %v", operation.ID, err)
			return
		}

		// TODO: implement send retry

		doSend := func(ctx context.Context) error {
			queuedReportMtx.Lock()
			defer queuedReportMtx.Unlock()

			err := tracer.sendReport(ctx, tracer.endpoint, tracer.target, tracer.token, queuedReport)
			if err != nil {
				return err
			}

			// clear queued report
			queuedReport = nil
			return nil
		}

		// synchronous
		if tracer.sendReportTimeout == 0 {
			err := doSend(ctx)
			if err != nil {
				tracer.log.Printf("failed to send report for operation %q: %v", operation.ID, err)
			}
			return
		}

		// debounced
		if sendingQueued.CompareAndSwap(false, true) {
			go func() {
				defer sendingQueued.Store(false)
				time.Sleep(tracer.sendReportTimeout)

				err := doSend(
					// may time out and get cancelled
					// TODO: use a context with deadline
					context.TODO(),
				)
				if err != nil {
					tracer.log.Printf("failed to send queued report for operation %q: %v", operation.ID, err)
				}
			}()
		}
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
		for _, sel := range selSet {
			switch sel := sel.(type) {
			case *ast.Field:
				{
					fields = append(fields,
						fmt.Sprintf("%s.%s", sel.ObjectDefinition.Name, sel.Name),
					)
					for _, arg := range sel.Arguments {
						fields = append(fields, fmt.Sprintf("%s.%s.%s", sel.ObjectDefinition.Name, sel.Name, arg.Name))
						visitValue(arg.Value)
					}
					visitField(sel.SelectionSet)
				}
			case *ast.FragmentSpread:
				{
					// skip directly to the fields of the fragment
					visitField(sel.Definition.SelectionSet)
				}
			case *ast.InlineFragment:
				{
					// skip directly to the fields of the inline fragment
					visitField(sel.SelectionSet)
				}
			}
		}
	}
	visitValue = func(value *ast.Value) {
		if len(value.Children) == 0 {
			if value.Definition.Kind == ast.Enum {
				// single enum
				if value.Kind == ast.Variable {
					// variable
					for _, enum := range value.Definition.EnumValues {
						fields = append(fields,
							fmt.Sprintf("%s.%s", value.Definition.Name, enum.Name),
						)
					}
				} else {
					// hard-coded
					fields = append(fields,
						fmt.Sprintf("%s.%s", value.Definition.Name, value.Raw),
					)
				}
			} else {
				// single scalar
				fields = append(fields,
					value.Definition.Name,
				)
			}
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

			// other type of inputs
			fields = append(fields,
				fmt.Sprintf("%s.%s", value.Definition.Name, child.Name),
			)
			visitValue(child.Value)
		}
	}
	visitField(rootSelectionSet)
	return fields
}

var (
	queuedReport    *Report
	queuedReportMtx sync.Mutex
	sendingQueued   atomic.Bool
)

func queueOperation(operation *OperationWithInfo) error {
	queuedReportMtx.Lock()
	defer queuedReportMtx.Unlock()

	if queuedReport == nil {
		queuedReport = &Report{
			Operations: map[string]*Operation{},
		}
	}

	_, exists := queuedReport.Operations[operation.ID]
	if exists {
		return fmt.Errorf("operation with id %q already exists in report", operation.ID)
	}

	queuedReport.Size++
	queuedReport.Operations[operation.ID] = &operation.Operation
	queuedReport.OperationInfos = append(queuedReport.OperationInfos, &operation.OperationInfo)

	return nil
}
