# gqlhive [![Go Report Card](https://goreportcard.com/badge/github.com/enisdenjo/go-gqlhive)](https://goreportcard.com/report/github.com/enisdenjo/go-gqlhive) [![Go Reference](https://pkg.go.dev/badge/github.com/enisdenjo/go-gqlhive.svg)](https://pkg.go.dev/github.com/enisdenjo/go-gqlhive)

Usage reporting to GraphQL Hive for [gqlgen](https://gqlgen.com/).

## Getting started

### Install

```sh
go get github.com/enisdenjo/go-gqlhive@v2
```

### Set up usage reporting in Hive Console

First you have to set up [usage reporting and monitoring Hive Console](https://the-guild.dev/graphql/hive/docs/schema-registry/usage-reporting), define your target and acquire the access token.

### Use

Then, after [getting started with gqlgen](https://gqlgen.com/getting-started/), add the tracer to the server.

```go
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/enisdenjo/go-gqlhive"
)

const defaultPort = "8080"

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	srv := handler.New(NewExecutableSchema(graph.Config{Resolvers: &resolvers{}}))
	srv.AddTransport(transport.POST{})

	// 👇 use the gqlhive tracer with your token
	srv.Use(gqlhive.NewTracer(
		"<TARGET_ID> or <ORGANIZATION>/<PROJECT>/<TARGET>",
		"<ACCESS_TOKEN>",
	))

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

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"
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

	srv := handler.New(NewExecutableSchema(graph.Config{Resolvers: &resolvers{}}))
	srv.AddTransport(transport.POST{})

	srv.Use(gqlhive.NewTracer(
		"<TARGET_ID> or <ORGANIZATION>/<PROJECT>/<TARGET>",
		"<ACCESS_TOKEN>",
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

## Migrating from v1 to v2

The only breaking change in v2 is the move from registry tokens to access tokens. You can read more about the necessary steps in Hive in the [related migration guide](https://the-guild.dev/graphql/hive/docs/migration-guides/organization-access-tokens).

After acquiring the new access token, provide it alongside the target when setting up the tracer:

```diff
gqlhive.NewTracer(
-	"<REGISTRY_TOKEN>",
+	"<TARGET_ID> or <ORGANIZATION>/<PROJECT>/<TARGET>",
+	"<ACCESS_TOKEN>",
	...opts,
)
```
