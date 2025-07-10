# gqlhive [![Go Report Card](https://goreportcard.com/badge/github.com/enisdenjo/go-gqlhive)](https://goreportcard.com/report/github.com/enisdenjo/go-gqlhive) [![Go Reference](https://pkg.go.dev/badge/github.com/enisdenjo/go-gqlhive.svg)](https://pkg.go.dev/github.com/enisdenjo/go-gqlhive)

Usage reporting to GraphQL Hive for [gqlgen](https://gqlgen.com/).

## Getting started

### Install

```sh
go get github.com/enisdenjo/go-gqlhive@v1
```

### Use

After [getting started with gqlgen](https://gqlgen.com/getting-started/) add the tracer to the server.

```go
package main

import (
	"log"
	"net/http"
	"os"

	"example.com/myrepo/mydata/graph"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/enisdenjo/go-gqlhive"
)

const defaultPort = "8080"

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	srv := handler.NewDefaultServer(graph.NewExecutableSchema(graph.Config{Resolvers: &graph.Resolver{}}))

	// 👇 use the gqlhive tracer with your token
	srv.Use(gqlhive.NewTracer("<your-graphql-hive-token>"))

	http.Handle("/", playground.Handler("GraphQL playground", "/query"))
	http.Handle("/query", srv)

	log.Printf("connect to http://localhost:%s/ for GraphQL playground", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
```

### Configure

See [traceroptions.go](/traceroptions.go) for configuring the tracer.

For example:

```go
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"example.com/myrepo/mydata/graph"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/domonda/go-types/nullable"
	"github.com/enisdenjo/go-gqlhive"
)

const defaultPort = "8080"

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	srv := handler.NewDefaultServer(graph.NewExecutableSchema(graph.Config{Resolvers: &graph.Resolver{}}))

	// 👇 use the gqlhive tracer with your token and custom options
	srv.Use(gqlhive.NewTracer("<your-graphql-hive-token>",
		gqlhive.WithEndpoint("http://localhost"),
		gqlhive.WithGenerateID(func(operation string, operationName nullable.TrimmedString) string {
			return "<custom unique ID generation for operations>"
		}),
		gqlhive.WithSendReportTimeout(5*time.Second),
		gqlhive.WithSendReport(func(ctx context.Context, endpoint, token string, report *gqlhive.Report) error {
			// custom report sender for queued reports
			return nil
		}),
	))

	http.Handle("/", playground.Handler("GraphQL playground", "/query"))
	http.Handle("/query", srv)

	log.Printf("connect to http://localhost:%s/ for GraphQL playground", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
```
