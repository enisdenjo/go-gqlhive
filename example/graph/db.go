package graph

import "github.com/enisdenjo/go-gqlhive/example/graph/model"

// In-Memory database for the graph.

var (
	todos []*model.Todo
	users []*model.User
)

// Seed the database.
func init() {
	john := &model.User{
		ID:   "u0",
		Name: "John",
	}
	users = []*model.User{john}

	todos = []*model.Todo{
		{
			ID:   "t0",
			Text: "Buy Milk",
			Done: true,
			User: john,
		},
		{
			ID:   "t1",
			Text: "Make Pancakes",
			Done: false,
			User: john,
		},
	}
}
