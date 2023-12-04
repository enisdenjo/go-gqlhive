package gqlhive

import (
	"context"
	"os"
	"testing"

	"github.com/99designs/gqlgen/client"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/domonda/go-types/nullable"
	"github.com/enisdenjo/go-gqlhive/fixtures/todos/graph"
	"github.com/gkampitakis/go-snaps/snaps"
)

func TestMain(t *testing.M) {
	v := t.Run()
	snaps.Clean(t)
	os.Exit(v)
}

func TestCreatedReports(t *testing.T) {
	var queries = []string{
		"{ todos { id } }",
		`{
			todos {
				id
				user {
					name
					id
				}
				text
				done
			}
		}`,
	}

	for _, query := range queries {
		t.Run(query, func(t *testing.T) {
			server := handler.NewDefaultServer(graph.NewExecutableSchema(graph.Config{Resolvers: &graph.Resolver{}}))

			var sentReport *Report
			server.Use(NewTracer(
				"<token>",
				WithGenerateID(func(operation string, operationName nullable.TrimmedString) string {
					return "id"
				}),
				WithSendReport(func(ctx context.Context, endpoint, token string, report *Report) error {
					for _, info := range report.OperationInfos {
						info.Timestamp = -1
						info.Execution.Duration = -1
					}
					sentReport = report
					return nil
				}),
			))

			res := map[string]any{}
			client.New(server).MustPost(query, &res)

			snaps.MatchJSON(t, sentReport)
		})
	}
}
