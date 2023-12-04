package gqlhive

import (
	"context"
	"fmt"

	"github.com/domonda/go-types/nullable"
)

const (
	CLIENT_NAME    = "go-gqlhive"
	CLIENT_VERSION = "0.0.0"
)

type Report struct {
	// Number of operations being reported
	Size uint `json:"size"`
	// The executed operations
	Operations map[string]*Operation `json:"map"`
	// Info about each operation's execution
	OperationInfos []*OperationInfo `json:"operations"`
}

type Operation struct {
	// Operation's body
	// e.g. "query me { me { id name } }"
	Operation string `json:"operation"`
	// Name of the operation
	// e.g. "me"
	OperationName nullable.TrimmedString `json:"operationName,omitempty"`
	// Schema coordinates
	// e.g. ["Query", "Query.me", "User", "User.id", "User.name"]
	Fields []string `json:"fields"`
}

type OperationInfo struct {
	// The ID of the operation in the operations map
	ID string `json:"operationMapKey"`
	// UNIX time in miliseconds of the operation's execution start
	Timestamp int64     `json:"timestamp"`
	Execution Execution `json:"execution"`
	Metadata  Metadata  `json:"metadata"`
}

type Execution struct {
	// Was the execution successful?
	Ok bool `json:"ok"`
	// Duration of the entire operation in nanoseconds
	Duration int64 `json:"duration"`
	// Total number of occured GraphQL errors
	ErrorsTotal int `json:"errorsTotal"`
}

type Metadata struct {
	Client Client `json:"client"`
}

type Client struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type OperationWithInfo struct {
	Operation
	OperationInfo
}

var operationCtxKey int

func ContextWithOperation(ctx context.Context, operation *OperationWithInfo) context.Context {
	return context.WithValue(ctx, &operationCtxKey, operation)
}

func OperationFromContext(ctx context.Context) (operation *OperationWithInfo, exists bool) {
	operationVal := ctx.Value(&operationCtxKey)
	if operationVal == nil {
		return nil, false
	}
	return operationVal.(*OperationWithInfo), true
}

func AddOperationToReport(report *Report, operation *OperationWithInfo) error {
	if report.Operations == nil {
		report.Operations = map[string]*Operation{}
	}

	_, exists := report.Operations[operation.ID]
	if exists {
		return fmt.Errorf("operation with id %q already exists in report", operation.ID)
	}

	report.Size++
	report.Operations[operation.ID] = &operation.Operation
	report.OperationInfos = append(report.OperationInfos, &operation.OperationInfo)

	return nil

}
