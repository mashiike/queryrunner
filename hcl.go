package queryrunner

import (
	"errors"
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/mashiike/hclconfig"
)

func DecodeBody(body hcl.Body, ctx *hcl.EvalContext) (PreparedQueries, hcl.Body, hcl.Diagnostics) {
	schema := &hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{
				Type:       "query_runner",
				LabelNames: []string{"type", "name"},
			},
			{
				Type:       "query",
				LabelNames: []string{"name"},
			},
		},
	}
	content, remain, diags := body.PartialContent(schema)
	diags = append(diags, hclconfig.RestrictUniqueBlockLabels(content)...)

	queryRunnerBlocks := make(hcl.Blocks, 0)
	queryBlocks := make(hcl.Blocks, 0)
	for _, block := range content.Blocks {
		switch block.Type {
		case "query_runner":
			queryRunnerBlocks = append(queryRunnerBlocks, block)
		case "query":
			queryBlocks = append(queryBlocks, block)
		}
	}
	runners := make(QueryRunners, 0, len(queryRunnerBlocks))
	for _, block := range queryRunnerBlocks {
		runnerType := block.Labels[0]
		runnerName := block.Labels[1]
		query, buildDiags := NewQueryRunner(runnerType, runnerName, block.Body, ctx)
		diags = append(diags, buildDiags...)
		runners = append(runners, query)
	}
	queries := make(PreparedQueries, 0, len(queryBlocks))
	for _, block := range queryBlocks {
		base := &QueryBase{
			name: block.Labels[0],
		}
		query, decodeDiags := base.DecodeBody(block.Body, ctx, runners)
		diags = append(diags, decodeDiags...)
		if decodeDiags.HasErrors() {
			continue
		}
		queries = append(queries, query)
	}

	return queries, remain, diags
}

func TraversalQuery(traversal hcl.Traversal, queries PreparedQueries) (PreparedQuery, error) {
	if traversal.IsRelative() {
		return nil, errors.New("traversal is relative")
	}
	if traversal.RootName() != "query" {
		return nil, fmt.Errorf("expected root name is `query` s, actual %s", traversal.RootName())
	}
	if len(traversal) < 2 {
		return nil, errors.New("traversal length < 2")
	}
	attr, ok := traversal[1].(hcl.TraverseAttr)
	if !ok {
		return nil, errors.New("traversal[1] is not TraverseAttr")
	}
	query, ok := queries.Get(attr.Name)
	if !ok {
		return nil, fmt.Errorf("query.%s is not found", attr.Name)
	}
	return query, nil
}
