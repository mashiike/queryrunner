package queryrunner

import (
	"context"
	"fmt"
	"log"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

type PreparedQuery interface {
	Name() string
	Description() string
	RunnerType() string
	Run(ctx context.Context, variables map[string]cty.Value, functions map[string]function.Function) (*QueryResult, error)
}

type PreparedQueries []PreparedQuery

func (queries PreparedQueries) Get(name string) (PreparedQuery, bool) {
	for _, query := range queries {
		if query.Name() != name {
			continue
		}
		return query, true
	}
	return nil, false
}

func (queries *PreparedQueries) DecodeBody(body hcl.Body, ctx *hcl.EvalContext) hcl.Diagnostics {
	var diags hcl.Diagnostics
	if queries == nil {
		panic("queries is nil")
	}
	*queries, _, diags = DecodeBody(body, ctx)
	return diags
}

type QueryBase struct {
	name        string
	description string
	runner      QueryRunner
	body        hcl.Body
	remain      hcl.Body
	evalCtx     *hcl.EvalContext
}

func (q *QueryBase) Name() string {
	return q.name
}

func (q *QueryBase) Description() string {
	return q.description
}

func (q *QueryBase) Runner() QueryRunner {
	return q.runner
}

func (q *QueryBase) RunnerType() string {
	return q.runner.Type()
}

func (q *QueryBase) NewEvalContext(variables map[string]cty.Value, functions map[string]function.Function) *hcl.EvalContext {
	ctx := q.evalCtx.NewChild()
	ctx.Variables = variables
	ctx.Functions = functions
	return ctx
}

func (q *QueryBase) Body() hcl.Body {
	return q.body
}

func (q *QueryBase) Remain() hcl.Body {
	return q.remain
}

func (q *QueryBase) DecodeBody(body hcl.Body, ctx *hcl.EvalContext, queryRunners QueryRunners) (PreparedQuery, hcl.Diagnostics) {
	q.body = body
	q.evalCtx = ctx

	schema := &hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{
			{
				Name: "description",
			},
			{
				Name:     "runner",
				Required: true,
			},
		},
	}
	content, remain, diags := body.PartialContent(schema)
	q.remain = remain
	for _, attr := range content.Attributes {
		switch attr.Name {
		case "description":
			value, valueDiags := attr.Expr.Value(ctx)
			diags = append(diags, valueDiags...)
			if !value.IsKnown() {
				diags = append(diags, &hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  "Invalid description",
					Detail:   `description is unknown"`,
					Subject:  attr.Expr.Range().Ptr(),
				})
				continue
			}
			if value.Type() != cty.String {
				diags = append(diags, &hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  "Invalid description",
					Detail:   `description is not string"`,
					Subject:  attr.Expr.Range().Ptr(),
				})
				continue
			}
			q.description = value.AsString()
		case "runner":
			variables := attr.Expr.Variables()
			if len(variables) == 0 {
				diags = append(diags, &hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  "Invalid Query Runner",
					Detail:   `can not set constant value. please write as runner = "query_runner.type.name"`,
					Subject:  attr.Expr.Range().Ptr(),
				})
				continue
			}
			if len(variables) != 1 {
				diags = append(diags, &hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  "Invalid Query Runner",
					Detail:   `can not set multiple query runners. please write as runner = "query_runner.type.name"`,
					Subject:  attr.Expr.Range().Ptr(),
				})
				continue
			}
			traversal := variables[0]
			if traversal.IsRelative() {
				diags = append(diags, &hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  "Invalid Relation",
					Detail:   `traversal is relative, query.runner depends on "qurey_runner" block,  please write as runner = "query_runner.type.name"`,
					Subject:  traversal.SourceRange().Ptr(),
				})
				continue
			}
			if rootName := traversal.RootName(); rootName != "query_runner" {
				diags = append(diags, &hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  "Invalid Relation",
					Detail:   fmt.Sprintf(`invalid refarence "%s.*", query.runner depends on "qurey_runner" block,  please write as runner = "query_runner.type.name"`, rootName),
					Subject:  traversal.SourceRange().Ptr(),
				})
				continue
			}
			if len(traversal) != 3 {
				diags = append(diags, &hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  "Invalid Relation",
					Detail:   `query.runner depends on "qurey_runner" block,  please write as runner = "query_runner.type.name"`,
					Subject:  traversal.SourceRange().Ptr(),
				})
				continue
			}
			typeAttr, ok := traversal[1].(hcl.TraverseAttr)
			if !ok {
				diags = append(diags, &hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  "Invalid Relation",
					Detail:   `query.runner depends on "qurey_runner" block,  please write as runner = "query_runner.type.name"`,
					Subject:  traversal.SourceRange().Ptr(),
				})
				continue
			}
			nameAttr, ok := traversal[2].(hcl.TraverseAttr)
			if !ok {
				diags = append(diags, &hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  "Invalid Relation",
					Detail:   `query.runner depends on "qurey_runner" block,  please write as runner = "query_runner.type.name"`,
					Subject:  traversal.SourceRange().Ptr(),
				})
				continue
			}
			log.Printf("[debug] try runner type `%s` restriction", typeAttr.Name)
			q.runner, ok = queryRunners.Get(typeAttr.Name, nameAttr.Name)
			if !ok {
				diags = append(diags, &hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  "Invalid Relation",
					Detail:   fmt.Sprintf(`query_runner "%s.%s" is not found`, typeAttr.Name, nameAttr.Name),
					Subject:  variables[0].SourceRange().Ptr(),
				})
				continue
			}
		}
	}
	if diags.HasErrors() {
		return nil, diags
	}

	preparedQuery, prepareDiags := q.runner.Prepare(q)
	diags = append(diags, prepareDiags...)
	return preparedQuery, diags
}
