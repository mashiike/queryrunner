package queryrunner

import (
	"errors"
	"fmt"
	"log"

	"github.com/agext/levenshtein"
	"github.com/hashicorp/hcl/v2"
)

var queryRunners = make(map[string]*QueryRunnerDefinition)

type QueryRunnerDefinition struct {
	TypeName             string
	BuildQueryRunnerFunc func(name string, body hcl.Body, ctx *hcl.EvalContext) (QueryRunner, hcl.Diagnostics)
}

type QueryRunner interface {
	Name() string
	Type() string
	Prepare(base *QueryBase) (PreparedQuery, hcl.Diagnostics)
}

type QueryRunners []QueryRunner

func (runners QueryRunners) Get(queryRunnerType string, name string) (QueryRunner, bool) {
	for _, runner := range runners {
		if runner.Type() != queryRunnerType {
			continue
		}
		if runner.Name() != name {
			continue
		}
		return runner, true
	}
	return nil, false
}

func Register(def *QueryRunnerDefinition) error {
	if def == nil {
		return errors.New("QueryRunnerDefinition is nil")
	}
	if def.TypeName == "" {
		return errors.New("TypeName is required")
	}
	if def.BuildQueryRunnerFunc == nil {
		return errors.New("BuildQueryRunnerFunc is required")
	}
	queryRunners[def.TypeName] = def
	return nil
}

func getQueryRunner(queryRunnerType string, body hcl.Body) (*QueryRunnerDefinition, hcl.Diagnostics) {
	def, ok := queryRunners[queryRunnerType]
	if !ok {
		for suggestion := range queryRunners {
			dist := levenshtein.Distance(queryRunnerType, suggestion, nil)
			if dist < 3 {
				return nil, hcl.Diagnostics([]*hcl.Diagnostic{
					{
						Severity: hcl.DiagError,
						Summary:  "Invalid query_runner type",
						Detail:   fmt.Sprintf(`The query runner type "%s" is invalid. Did you mean "%s"?`, queryRunnerType, suggestion),
						Subject:  body.MissingItemRange().Ptr(),
					},
				})
			}
		}
		return nil, hcl.Diagnostics([]*hcl.Diagnostic{
			{
				Severity: hcl.DiagError,
				Summary:  "Invalid query_runner type",
				Detail:   fmt.Sprintf(`The query runner type "%s" is invalid. maybe not implemented or typo`, queryRunnerType),
				Subject:  body.MissingItemRange().Ptr(),
			},
		})
	}
	return def, nil
}

func NewQueryRunner(queryRunnerType string, name string, body hcl.Body, ctx *hcl.EvalContext) (QueryRunner, hcl.Diagnostics) {
	def, diags := getQueryRunner(queryRunnerType, body)
	if diags.HasErrors() {
		return nil, diags
	}
	queryRunner, buildDiags := def.BuildQueryRunnerFunc(name, body, ctx)
	log.Printf("[debug] build query_runner `%s` as %T", queryRunnerType, queryRunner)
	diags = append(diags, buildDiags...)
	log.Printf("[debug] build query_runner `%s`, `%d` error diags", queryRunnerType, len(diags.Errs()))
	return queryRunner, diags
}
