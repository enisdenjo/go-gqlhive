package gqlhive

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/99designs/gqlgen/client"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/domonda/go-types/nullable"
	"github.com/enisdenjo/go-gqlhive/internal/fixtures/todos/graph"
	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/stretchr/testify/require"
)

func TestMain(t *testing.M) {
	v := t.Run()
	snaps.Clean(t)
	os.Exit(v)
}

// TODO: test errored fields

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
		`mutation CreateTodo {
			createTodo(input: { text: "Check Mail", userId: "u0" }) {
				id
				text
				user {
					name
				}
				done
			}
		}`,
		`mutation CreateTodo {
			createTodo(input: { userId: "u0", text: "Check Mail" }) {
				id
			}
		}`,
		`{
			todos(sortBy: NAME_DESC, condition: { searchText: "test" }) {
				id
			}
		}`,
		`{
			todos(condition: { statuses: [DONE, ASSIGNED] }) {
				id
			}
		}`,
		`{
			todos(condition: { userStatus: AVAILABLE }) {
				id
			}
		}`,
		`{
			todos(condition: { user: { name: "deep" } }) {
				id
			}
		}`,
		`{
			todos {
				...TodoFragment
			}
		}
		fragment TodoFragment on Todo {
			id
			text
			user {
				... on User {
					id
					name
				}
			}
			done
		}
		`,
		`query Todos($searchText: String) {
			todos(condition: { searchText: $searchText }) {
				id
			}
		}`,
		`query Todos($userStatus: TodosConditionUserStatus) {
			todos(condition: { userStatus: $userStatus }) {
				id
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
				WithSendReportTimeout(0),
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

func TestSendingQueuedReports(t *testing.T) {
	server := handler.NewDefaultServer(graph.NewExecutableSchema(graph.Config{Resolvers: &graph.Resolver{}}))

	var wg sync.WaitGroup
	wg.Add(1)
	var sentReport *Report
	server.Use(NewTracer(
		"<token>",
		WithGenerateID(func(operation string, operationName nullable.TrimmedString) string {
			return operation
		}),
		WithSendReportTimeout(1*time.Second),
		WithSendReport(func(ctx context.Context, endpoint, token string, report *Report) error {
			for _, info := range report.OperationInfos {
				info.Timestamp = -1
				info.Execution.Duration = -1
			}
			sentReport = report
			wg.Done() // we also test the amount of SendReport calles here because calling wg.Done too many times will panic
			return nil
		}),
	))

	res := map[string]any{}
	client.New(server).MustPost("{ todos { id } } #1", &res)
	client.New(server).MustPost("{ todos { id } } #2", &res)
	client.New(server).MustPost("{ todos { id } } #3", &res)
	client.New(server).MustPost("{ todos { id } } #4", &res)

	wg.Wait()

	snaps.MatchJSON(t, sentReport)
}

func TestSendingReportsOverHTTP(t *testing.T) {
	token := "sometoken123"
	tserver := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.WriteHeader(http.StatusOK)

		require.Equal(t, "POST", req.Method)
		require.Equal(t, "application/json", req.Header.Get("Content-Type"))
		require.Equal(t, fmt.Sprintf("Bearer %s", token), req.Header.Get("Authorization"))

		report := &Report{}
		err := json.NewDecoder(req.Body).Decode(&report)
		require.NoError(t, err)

		for _, info := range report.OperationInfos {
			info.Timestamp = -1
			info.Execution.Duration = -1
		}

		snaps.MatchJSON(t, report)
	}))
	defer tserver.Close()

	server := handler.NewDefaultServer(graph.NewExecutableSchema(graph.Config{Resolvers: &graph.Resolver{}}))

	server.Use(NewTracer(
		token,
		WithEndpoint(tserver.URL),
		WithGenerateID(func(operation string, operationName nullable.TrimmedString) string {
			return "id"
		}),
		WithSendReportTimeout(0),
	))

	res := map[string]any{}
	client.New(server).MustPost("{ todos { id } }", &res)
}
