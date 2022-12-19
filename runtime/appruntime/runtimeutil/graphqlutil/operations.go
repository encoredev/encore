package graphqlutil

import (
	"encoding/json"

	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"
)

type OpType string

const (
	Query        OpType = "query"
	Mutation     OpType = "mutation"
	Subscription OpType = "subscription"
)

type Op struct {
	Type OpType `json:"type"`
	Name string `json:"name"`
}

// GetOperations attempts to return the operation name for a given input to a
// GraphQL raw endpoint.
//
// This follows the behaviour for handling HTTP endpoints as specified here:
// https://graphql.org/learn/serving-over-http/#http-methods-headers-and-body
//
// If the input is not a valid GraphQL document, nil operations are returned
func GetOperations(httpMethod string, contentType string, possibleQueryDoc []byte) []*Op {
	var qryString string

	switch httpMethod {
	case "GET":
		// GET requests are always sent as query strings
		qryString = string(possibleQueryDoc)
	case "POST":
		// POST requests are sent as encoded JSON objects,
		// unless the Content-Type is application/graphql
		if contentType == "application/graphql" {
			qryString = string(possibleQueryDoc)
		} else {
			postedQueryDoc := &postedQuery{}
			if err := json.Unmarshal(possibleQueryDoc, postedQueryDoc); err != nil || postedQueryDoc.Query == "" {
				return nil
			}
			qryString = postedQueryDoc.Query
		}
	default:
		return nil
	}

	// Parse the query
	qry, err := parser.ParseQuery(&ast.Source{Input: qryString})
	if err != nil {
		return nil
	}

	// Return the operations listed
	ops := make([]*Op, 0, len(qry.Operations))
	for _, op := range qry.Operations {
		ops = append(ops, &Op{
			Type: OpType(op.Operation),
			Name: op.Name,
		})
	}
	return ops
}

type postedQuery struct {
	Query         string         `json:"query"`
	OperationName string         `json:"operationName,omitempty"`
	Vars          map[string]any `json:"variables,omitempty"`
}
