package cloudwatchlogsinsights

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/dustin/go-humanize"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/mashiike/queryrunner"
	"github.com/samber/lo"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

const TypeName = "cloudwatch_logs_insights"

func init() {
	err := queryrunner.Register(&queryrunner.QueryRunnerDefinition{
		TypeName:             TypeName,
		BuildQueryRunnerFunc: BuildQueryRunner,
	})
	if err != nil {
		panic(fmt.Errorf("register cloudwatch_logs_insights query runner:%w", err))
	}
}

func BuildQueryRunner(name string, body hcl.Body, ctx *hcl.EvalContext) (queryrunner.QueryRunner, hcl.Diagnostics) {
	queryRunner := &QueryRunner{
		name: name,
	}
	diags := gohcl.DecodeBody(body, ctx, queryRunner)
	if diags.HasErrors() {
		return nil, diags
	}
	optFns := make([]func(*config.LoadOptions) error, 0)
	if queryRunner.Region != nil {
		optFns = append(optFns, config.WithRegion(*queryRunner.Region))
	}
	awsCfg, err := config.LoadDefaultConfig(context.Background(), optFns...)
	if err != nil {

		return nil, diags
	}
	queryRunner.client = cloudwatchlogs.NewFromConfig(awsCfg)
	return queryRunner, diags
}

func (r *QueryRunner) Name() string {
	return r.name
}

func (r *QueryRunner) Type() string {
	return TypeName
}

type QueryRunner struct {
	client *cloudwatchlogs.Client
	name   string
	Region *string `hcl:"region"`
}

type PreparedQuery struct {
	*queryrunner.QueryBase
	runner *QueryRunner

	StartTime hcl.Expression `hcl:"start_time"`
	EndTime   hcl.Expression `hcl:"end_time"`
	Query     hcl.Expression `hcl:"query"`
	Limit     *int32         `hcl:"limit"`

	LogGroupNames hcl.Expression `hcl:"log_group_names,optional"`
	IgnoreFields  []string       `hcl:"ignore_fields,optional"`
}

func (r *QueryRunner) Prepare(base *queryrunner.QueryBase) (queryrunner.PreparedQuery, hcl.Diagnostics) {
	log.Printf("[debug] prepare `%s` with cloudwatch_logs_insights query_runner", base.Name())
	q := &PreparedQuery{
		QueryBase: base,
		runner:    r,
	}
	body := base.Remain()
	ctx := base.NewEvalContext(nil, nil)
	diags := gohcl.DecodeBody(body, ctx, q)
	if diags.HasErrors() {
		return nil, diags
	}
	queryValue, _ := q.Query.Value(ctx)
	if queryValue.IsKnown() && queryValue.IsNull() {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid query template",
			Detail:   "required attribute query",
			Subject:  q.Query.Range().Ptr(),
		})
	}
	log.Printf("[debug] end cloudwatch_logs_insights query block %d error diags", len(diags.Errs()))
	logGroupNamesValue, _ := q.LogGroupNames.Value(ctx)
	if logGroupNamesValue.IsKnown() && logGroupNamesValue.IsNull() {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid log_group_names",
			Detail:   "required attribute log_group_names",
			Subject:  q.LogGroupNames.Range().Ptr(),
		})
	}
	startTimeValue, _ := q.StartTime.Value(ctx)
	if startTimeValue.IsKnown() && startTimeValue.IsNull() {
		var parseDiags hcl.Diagnostics
		q.StartTime, parseDiags = hclsyntax.ParseExpression([]byte(`now() - duration("15m")`), "default_start_time.hcl", hcl.InitialPos)
		diags = append(diags, parseDiags...)
	}
	endTimeValue, _ := q.EndTime.Value(ctx)
	if endTimeValue.IsKnown() && endTimeValue.IsNull() {
		var parseDiags hcl.Diagnostics
		q.EndTime, parseDiags = hclsyntax.ParseExpression([]byte(`now()`), "default_end_time.hcl", hcl.InitialPos)
		diags = append(diags, parseDiags...)
	}
	return q, diags
}

func (q *PreparedQuery) Run(ctx context.Context, variables map[string]cty.Value, functions map[string]function.Function) (*queryrunner.QueryResult, error) {
	evalCtx := q.NewEvalContext(variables, functions)
	queryValue, diags := q.Query.Value(evalCtx)
	if diags.HasErrors() {
		return nil, diags
	}
	if !queryValue.IsKnown() {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid query template",
			Detail:   "query is unknown",
			Subject:  q.Query.Range().Ptr(),
		})
		return nil, diags
	}
	query := queryValue.AsString()
	if query == "" {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid query template",
			Detail:   "query is empty",
			Subject:  q.Query.Range().Ptr(),
		})
		return nil, diags
	}
	if queryValue.Type() != cty.String {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid query template",
			Detail:   "query is not string",
			Subject:  q.Query.Range().Ptr(),
		})
		return nil, diags
	}

	startTimeValue, diags := q.StartTime.Value(evalCtx)
	if diags.HasErrors() {
		return nil, diags
	}
	if !startTimeValue.IsKnown() {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid start_time template",
			Detail:   "start_time is unknown",
			Subject:  q.StartTime.Range().Ptr(),
		})
		return nil, diags
	}
	if startTimeValue.Type() != cty.Number {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid start_time template",
			Detail:   "start_time is not number",
			Subject:  q.StartTime.Range().Ptr(),
		})
		return nil, diags
	}
	startTimeEpoch, _ := startTimeValue.AsBigFloat().Float64()
	startTime := time.Unix(0, int64(startTimeEpoch*float64(time.Second)))

	endTimeValue, diags := q.EndTime.Value(evalCtx)
	if diags.HasErrors() {
		return nil, diags
	}
	if !endTimeValue.IsKnown() {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid end_time template",
			Detail:   "end_time is unknown",
			Subject:  q.EndTime.Range().Ptr(),
		})
		return nil, diags
	}
	if endTimeValue.Type() != cty.Number {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid end_time template",
			Detail:   "end_time is not number",
			Subject:  q.EndTime.Range().Ptr(),
		})
		return nil, diags
	}
	endTimeEpoch, _ := endTimeValue.AsBigFloat().Float64()
	endTime := time.Unix(0, int64(endTimeEpoch*float64(time.Second)))

	params := &cloudwatchlogs.StartQueryInput{
		StartTime:   aws.Int64(startTime.Unix()),
		EndTime:     aws.Int64(endTime.Unix()),
		QueryString: aws.String(query),
		Limit:       q.Limit,
	}

	logGroupNamesValue, diags := q.LogGroupNames.Value(evalCtx)
	if diags.HasErrors() {
		return nil, diags
	}
	if !logGroupNamesValue.IsKnown() {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid log_group_names template",
			Detail:   "log_group_names is unknown",
			Subject:  q.LogGroupNames.Range().Ptr(),
		})
		return nil, diags
	}
	logGroupNameValues := logGroupNamesValue.AsValueSlice()
	if !logGroupNamesValue.Type().IsListType() && !logGroupNamesValue.Type().IsTupleType() {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid log_group_names",
			Detail:   "log_group_names is must string list",
			Subject:  q.LogGroupNames.Range().Ptr(),
		})
		return nil, diags
	}
	if len(logGroupNameValues) == 0 {
		return nil, errors.New("missing log_group_names")
	}
	if len(logGroupNameValues) == 1 {
		params.LogGroupName = aws.String(logGroupNameValues[0].AsString())
	} else {
		params.LogGroupNames = lo.Map(logGroupNameValues, func(v cty.Value, _ int) string {
			return v.AsString()
		})
	}

	return q.runner.RunQuery(ctx, q.Name(), params, q.IgnoreFields)
}

func (r *QueryRunner) RunQuery(ctx context.Context, name string, params *cloudwatchlogs.StartQueryInput, ignoreFields []string) (*queryrunner.QueryResult, error) {
	reqID := queryrunner.GetRequestID(ctx)
	startQueryOutput, err := r.client.StartQuery(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("start_query: %w", err)
	}
	var logGroupNames string
	if params.LogGroupName != nil {
		logGroupNames = *params.LogGroupName
	}
	if params.LogGroupNames != nil {
		logGroupNames = "[" + strings.Join(params.LogGroupNames, ",") + "]"
	}
	log.Printf("[info][%s] start cloudwatch logs insights query to %s", reqID, logGroupNames)
	log.Printf("[info][%s] time range: %s ~ %s", reqID, time.Unix(*params.StartTime, 0).In(time.Local), time.Unix(*params.EndTime, 0).In(time.Local))
	log.Printf("[debug][%s] query string: %s", reqID, *params.QueryString)
	queryStart := time.Now()
	getQueryResultOutput, err := r.waitQueryResult(ctx, queryStart, &cloudwatchlogs.GetQueryResultsInput{
		QueryId: startQueryOutput.QueryId,
	})
	if err != nil {
		return nil, err
	}
	log.Printf("[debug][%s] query result: %d results, %s scanned, %f records matched, %f recoreds scanned",
		reqID,
		len(getQueryResultOutput.Results),
		humanize.Bytes(uint64(getQueryResultOutput.Statistics.BytesScanned)),
		getQueryResultOutput.Statistics.RecordsMatched,
		getQueryResultOutput.Statistics.RecordsScanned,
	)
	columnsMap := make(map[string]int)
	rowsMap := make([]map[string]interface{}, 0, len(getQueryResultOutput.Results))
	ignoreFields = append([]string{"@ptr"}, ignoreFields...)
	for _, results := range getQueryResultOutput.Results {
		row := make(map[string]interface{}, len(results))
		for _, result := range results {
			if lo.Contains(ignoreFields, *result.Field) {
				continue
			}
			if _, ok := columnsMap[*result.Field]; !ok {
				columnsMap[*result.Field] = len(columnsMap)
			}
			if result.Value == nil {
				row[*result.Field] = ""
			} else {
				row[*result.Field] = *result.Value
			}
		}
		rowsMap = append(rowsMap, row)
	}
	return queryrunner.NewQueryResultWithRowsMap(name, *params.QueryString, columnsMap, rowsMap), nil
}

func (r *QueryRunner) waitQueryResult(ctx context.Context, queryStart time.Time, params *cloudwatchlogs.GetQueryResultsInput) (*cloudwatchlogs.GetQueryResultsOutput, error) {
	reqID := queryrunner.GetRequestID(ctx)
	waiter := &queryrunner.Waiter{
		StartTime: queryStart,
		MinDelay:  100 * time.Microsecond,
		MaxDelay:  5 * time.Second,
		Timeout:   15 * time.Minute,
		Jitter:    200 * time.Millisecond,
	}
	for waiter.Continue(ctx) {
		elapsedTime := time.Since(queryStart)
		log.Printf("[debug][%s] wating cloudwatch logs insights query elapsed_time=%s", reqID, elapsedTime)
		getQueryResultOutput, err := r.client.GetQueryResults(ctx, params)
		if err != nil {
			return nil, fmt.Errorf("get query results:%w", err)
		}

		switch getQueryResultOutput.Status {
		case types.QueryStatusRunning, types.QueryStatusScheduled:
		case types.QueryStatusComplete:
			return getQueryResultOutput, nil
		default:
			return nil, errors.New("get query result unknown status ")
		}
	}
	return nil, errors.New("wait query result timeout")
}
