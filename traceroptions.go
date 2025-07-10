package gqlhive

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/domonda/go-types/nullable"
	"github.com/domonda/go-types/uu"
)

const defaultEndpoint = "https://app.graphql-hive.com/usage"

// WithEndpoint sets the endpoint to where the reports are sent.
// Defaults to "https://app.graphql-hive.com/usage".
func WithEndpoint(endpoint string) TracerOption {
	return tracerOptionFn(func(tracer *Tracer) {
		tracer.endpoint = endpoint
	})
}

func defaultGenerateID(operation string, operationName nullable.TrimmedString) string {
	return uu.IDv4().String()
}

// GenerateID creates unique operation IDs for the report.
type GenerateID func(operation string, operationName nullable.TrimmedString) string

// WithGenerateID sets the unique operation ID generator for the reports.
// Defaults to generating v4 UUIDs.
func WithGenerateID(fn GenerateID) TracerOption {
	return tracerOptionFn(func(tracer *Tracer) {
		tracer.generateID = fn
	})
}

var defaultSendReportTimeout time.Duration = 3 * time.Second

// WithSendReportTimeout sets the report sending debounce timeout.
// Executed operations will queue up and then be flushed/sent to GraphQL Hive after the timeout expires.
func WithSendReportTimeout(timeout time.Duration) TracerOption {
	return tracerOptionFn(func(tracer *Tracer) {
		tracer.sendReportTimeout = timeout
	})
}

func defaultSendReport(ctx context.Context, endpoint, target, token string, report *Report) error {
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(report)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint+"/"+target, &buf)
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Add("X-Usage-API-Version", "2")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("report sending failed with %d %s", res.StatusCode, res.Status)
	}

	return nil
}

// SendReport performs the actual report sending to GraphQL Hive.
type SendReport func(ctx context.Context, endpoint, target, token string, report *Report) error

// WithSendReport sets the report sender to GraphQL Hive.
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
