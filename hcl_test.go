package queryrunner_test

import (
	"context"
	"log"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/mashiike/hclconfig"
	"github.com/mashiike/queryrunner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

type dummyQueryRunner struct {
	name    string
	Columns []string `hcl:"columns"`
}

func (r *dummyQueryRunner) Name() string {
	return r.name
}

func (r *dummyQueryRunner) Type() string {
	return "dummy"
}

func (r *dummyQueryRunner) Prepare(base *queryrunner.QueryBase) (queryrunner.PreparedQuery, hcl.Diagnostics) {
	log.Printf("[debug] prepare `%s` with dummy query_runner", base.Name())
	q := &dummyPreparedQuery{
		QueryBase: base,
		columns:   r.Columns,
	}
	diags := gohcl.DecodeBody(base.Remain(), base.NewEvalContext(nil, nil), q)
	return q, diags

}

type dummyPreparedQuery struct {
	*queryrunner.QueryBase
	columns []string

	Rows hcl.Expression `hcl:"rows"`
}

func (q *dummyPreparedQuery) Run(ctx context.Context, v map[string]cty.Value, f map[string]function.Function) (*queryrunner.QueryResult, error) {
	var rows [][]string
	diags := gohcl.DecodeExpression(q.Rows, q.NewEvalContext(v, f), &rows)
	if diags.HasErrors() {
		return nil, diags
	}
	return queryrunner.NewQueryResult(q.Name(), "", q.columns, rows), nil
}

func TestDecodeBody(t *testing.T) {
	err := queryrunner.Register(&queryrunner.QueryRunnerDefinition{
		TypeName: "dummy",
		BuildQueryRunnerFunc: func(name string, body hcl.Body, ctx *hcl.EvalContext) (queryrunner.QueryRunner, hcl.Diagnostics) {
			runner := &dummyQueryRunner{
				name: name,
			}
			diags := gohcl.DecodeBody(body, ctx, runner)
			return runner, diags
		},
	})
	require.NoError(t, err)

	parser := hclparse.NewParser()
	src := []byte(`
	query_runner "dummy" "default" {
		columns = ["id", "name", "age"]
	}

	query "default" {
		runner = query_runner.dummy.default
		rows = [
			[ "1", "hoge", "13"],
			[ "2", "fuga", "26"],
		]
	}

	extra = "hoge"
	`)
	file, diags := parser.ParseHCL(src, "config.hcl")
	require.False(t, diags.HasErrors())
	queries, remain, diags := queryrunner.DecodeBody(file.Body, &hcl.EvalContext{})
	if !assert.False(t, diags.HasErrors()) {
		var builder strings.Builder
		w := hcl.NewDiagnosticTextWriter(&builder, parser.Files(), 400, false)
		w.WriteDiagnostics(diags)
		t.Log(builder.String())
		t.FailNow()
	}
	attrs, _ := remain.JustAttributes()
	require.Equal(t, 1, len(attrs))
	query, ok := queries.Get("default")
	require.True(t, ok)
	result, err := query.Run(context.Background(), nil, nil)
	require.NoError(t, err)
	expected := strings.TrimSpace(`
+----+------+-----+
| ID | NAME | AGE |
+----+------+-----+
|  1 | hoge |  13 |
|  2 | fuga |  26 |
+----+------+-----+
`)
	require.Equal(t, expected, strings.TrimSpace(result.ToTable()))
}

func TestDecodeBodyRequireQueryRunner(t *testing.T) {
	parser := hclparse.NewParser()
	src := []byte(`
	query "default" {}
	`)
	file, diags := parser.ParseHCL(src, "config.hcl")
	require.False(t, diags.HasErrors())
	_, _, diags = queryrunner.DecodeBody(file.Body, &hcl.EvalContext{})
	require.True(t, diags.HasErrors(), "has errors")

	var builder strings.Builder
	w := hcl.NewDiagnosticTextWriter(&builder, parser.Files(), 400, false)
	w.WriteDiagnostics(diags)
	expected := `
Error: Missing required argument

  on config.hcl line 2, in query "default":
   2: 	query "default" {}

The argument "runner" is required, but no definition was found.`
	require.EqualValues(t, strings.TrimSpace(expected), strings.TrimSpace(builder.String()))
}

func TestDecodeBodyMissingQueryRunner(t *testing.T) {
	parser := hclparse.NewParser()
	src := []byte(`
	query_runner "invalid" "default" {
		columns = ["id", "name", "age"]
	}
	`)
	file, diags := parser.ParseHCL(src, "config.hcl")
	require.False(t, diags.HasErrors())
	_, _, diags = queryrunner.DecodeBody(file.Body, &hcl.EvalContext{})
	require.True(t, diags.HasErrors(), "has errors")

	var builder strings.Builder
	w := hcl.NewDiagnosticTextWriter(&builder, parser.Files(), 400, false)
	w.WriteDiagnostics(diags)
	expected := `
Error: Invalid query_runner type

  on config.hcl line 2, in query_runner "invalid" "default":
   2: 	query_runner "invalid" "default" {

The query runner type "invalid" is invalid. maybe not implemented or typo
`
	require.EqualValues(t, strings.TrimSpace(expected), strings.TrimSpace(builder.String()))
}

func TestDecodeBodyDuplicateQueryRunner(t *testing.T) {
	err := queryrunner.Register(&queryrunner.QueryRunnerDefinition{
		TypeName: "dummy",
		BuildQueryRunnerFunc: func(name string, body hcl.Body, ctx *hcl.EvalContext) (queryrunner.QueryRunner, hcl.Diagnostics) {
			runner := &dummyQueryRunner{
				name: name,
			}
			diags := gohcl.DecodeBody(body, ctx, runner)
			return runner, diags
		},
	})
	require.NoError(t, err)
	parser := hclparse.NewParser()
	src := []byte(`
	query_runner "dummy" "default" {
		columns = ["id", "name", "age"]
	}
	query_runner "dummy" "default" {
		columns = ["id", "name", "age"]
	}
	`)
	file, diags := parser.ParseHCL(src, "config.hcl")
	require.False(t, diags.HasErrors())
	_, _, diags = queryrunner.DecodeBody(file.Body, &hcl.EvalContext{})
	require.True(t, diags.HasErrors(), "has errors")

	var builder strings.Builder
	w := hcl.NewDiagnosticTextWriter(&builder, parser.Files(), 400, false)
	w.WriteDiagnostics(diags)
	expected := `
Error: Duplicate query_runner "dummy" configuration

  on config.hcl line 5, in query_runner "dummy" "default":
   5: 	query_runner "dummy" "default" {

A dummy query_runner named "default" was already declared at config.hcl:2,2-32. query_runner names must unique per type in a configuration`
	require.EqualValues(t, strings.TrimSpace(expected), strings.TrimSpace(builder.String()))
}

func TestDecodeBodyDuplicateQuery(t *testing.T) {
	err := queryrunner.Register(&queryrunner.QueryRunnerDefinition{
		TypeName: "dummy",
		BuildQueryRunnerFunc: func(name string, body hcl.Body, ctx *hcl.EvalContext) (queryrunner.QueryRunner, hcl.Diagnostics) {
			runner := &dummyQueryRunner{
				name: name,
			}
			diags := gohcl.DecodeBody(body, ctx, runner)
			return runner, diags
		},
	})
	require.NoError(t, err)
	parser := hclparse.NewParser()
	src := []byte(`
	query_runner "dummy" "default" {
		columns = ["id", "name", "age"]
	}
	query "default" {
		runner = query_runner.dummy.default
		rows = [
			[ "1", "hoge", "13"],
			[ "2", "fuga", "26"],
		]
	}
	query "default" {
		runner = query_runner.dummy.default
		rows = [
			[ "1", "hoge", "13"],
			[ "2", "fuga", "26"],
		]
	}
	`)
	file, diags := parser.ParseHCL(src, "config.hcl")
	require.False(t, diags.HasErrors())
	_, _, diags = queryrunner.DecodeBody(file.Body, &hcl.EvalContext{})
	require.True(t, diags.HasErrors(), "has errors")

	var builder strings.Builder
	w := hcl.NewDiagnosticTextWriter(&builder, parser.Files(), 400, false)
	w.WriteDiagnostics(diags)
	expected := `
Error: Duplicate query declaration

  on config.hcl line 12, in query "default":
  12: 	query "default" {

A query named "default" was already declared at config.hcl:5,2-17. query names must unique within a configuration`
	require.EqualValues(t, strings.TrimSpace(expected), strings.TrimSpace(builder.String()))
}

func TestDecodeBodyQueryRunnerNotFound(t *testing.T) {
	parser := hclparse.NewParser()
	src := []byte(`
	query "default" {
		runner = query_runner.not_found.default
	}
	`)
	file, diags := parser.ParseHCL(src, "config.hcl")
	require.False(t, diags.HasErrors())
	_, _, diags = queryrunner.DecodeBody(file.Body, &hcl.EvalContext{})
	require.True(t, diags.HasErrors(), "has errors")

	var builder strings.Builder
	w := hcl.NewDiagnosticTextWriter(&builder, parser.Files(), 400, false)
	w.WriteDiagnostics(diags)
	expected := `
Error: Invalid Relation

  on config.hcl line 3, in query "default":
   3: 		runner = query_runner.not_found.default

query_runner "not_found.default" is not found`
	require.EqualValues(t, strings.TrimSpace(expected), strings.TrimSpace(builder.String()))
}

func TestDecodeBodyFunctionValueChain(t *testing.T) {
	err := queryrunner.Register(&queryrunner.QueryRunnerDefinition{
		TypeName: "dummy",
		BuildQueryRunnerFunc: func(name string, body hcl.Body, ctx *hcl.EvalContext) (queryrunner.QueryRunner, hcl.Diagnostics) {
			runner := &dummyQueryRunner{
				name: name,
			}
			diags := gohcl.DecodeBody(body, ctx, runner)
			return runner, diags
		},
	})
	require.NoError(t, err)

	parser := hclparse.NewParser()
	src := []byte(`
	query_runner "dummy" "default" {
		columns = ["id", "name", "age"]
	}

	query "default" {
		runner = query_runner.dummy.default
		rows = jsondecode(
			templatefile("testdata/rows.json", {
				start = var.start
			})
		)
	}

	extra = "hoge"
	`)
	file, diags := parser.ParseHCL(src, "config.hcl")
	if !assert.False(t, diags.HasErrors()) {
		var builder strings.Builder
		w := hcl.NewDiagnosticTextWriter(&builder, parser.Files(), 400, false)
		w.WriteDiagnostics(diags)
		t.Log(builder.String())
		t.FailNow()
	}
	queries, remain, diags := queryrunner.DecodeBody(file.Body, hclconfig.NewEvalContext("./"))
	if !assert.False(t, diags.HasErrors()) {
		var builder strings.Builder
		w := hcl.NewDiagnosticTextWriter(&builder, parser.Files(), 400, false)
		w.WriteDiagnostics(diags)
		t.Log(builder.String())
		t.FailNow()
	}
	attrs, _ := remain.JustAttributes()
	require.Equal(t, 1, len(attrs))
	query, ok := queries.Get("default")
	require.True(t, ok)
	result, err := query.Run(context.Background(), map[string]cty.Value{
		"var": cty.ObjectVal(map[string]cty.Value{
			"start": cty.NumberIntVal(1),
		}),
	}, nil)
	require.NoError(t, err)
	expected := strings.TrimSpace(`
+----+------+-----+
| ID | NAME | AGE |
+----+------+-----+
|  1 | hoge |  13 |
|  2 | fuga |  26 |
+----+------+-----+
`)
	require.Equal(t, expected, strings.TrimSpace(result.ToTable()))
}
