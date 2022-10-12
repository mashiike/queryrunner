package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/fatih/color"
	"github.com/fujiwara/logutils"
	"github.com/handlename/ssmwrap"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/ken39arg/go-flagx"
	"github.com/mashiike/hclconfig"
	"github.com/mashiike/queryrunner"
	_ "github.com/mashiike/queryrunner/cloudwatchlogsinsights"
	_ "github.com/mashiike/queryrunner/redshiftdata"
	_ "github.com/mashiike/queryrunner/s3select"
	"github.com/zclconf/go-cty/cty"
	"golang.org/x/sync/errgroup"
)

var filter = &logutils.LevelFilter{
	Levels: []logutils.LogLevel{"debug", "info", "notice", "warn", "error"},
	ModifierFuncs: []logutils.ModifierFunc{
		logutils.Color(color.FgHiBlack),
		nil,
		logutils.Color(color.FgHiBlue),
		logutils.Color(color.FgYellow),
		logutils.Color(color.FgRed, color.BgBlack),
	},
	MinLevel: "info",
	Writer:   os.Stderr,
}

func main() {
	if err := _main(); err != nil {
		log.Fatalln("[error]", err)
	}
}

const usage = `query-runner is a helper tool that makes querying several AWS services convenient

  usages:
    query-runner -l
    query-runner [options] <query_name1> <query_name2> ...
    cat params.json | query-runner [options]

  options:
    -c, --config        config dir, config format is HCL (defualt: ~/.config/query-runner/)
    -l, --list          displays a list of formats
	-o, --output        output format [json|table|markdown|borderless|vertical] (default:json)
	-v, --variables     variables json
    -h, --help          prints help information
        --log-level     log output level (default: info)
`

func _main() error {
	log.SetOutput(filter)
	ssmwrapPaths := os.Getenv("SSMWRAP_PATHS")
	paths := strings.Split(ssmwrapPaths, ",")
	if ssmwrapPaths != "" && len(paths) > 0 {
		err := ssmwrap.Export(ssmwrap.ExportOptions{
			Paths:   paths,
			Retries: 3,
		})
		if err != nil {
			return err
		}
	}
	ssmwrapNames := os.Getenv("SSMWRAP_NAMES")
	names := strings.Split(ssmwrapNames, ",")
	if ssmwrapNames != "" && len(names) > 0 {
		err := ssmwrap.Export(ssmwrap.ExportOptions{
			Names:   names,
			Retries: 3,
		})
		if err != nil {
			return err
		}
	}

	var (
		config    string
		logLevel  string
		showList  bool
		variables string
		output    string
	)
	flag.Usage = func() { fmt.Print(usage) }
	flag.StringVar(&config, "config", "", "")
	flag.StringVar(&config, "c", "", "")
	flag.BoolVar(&showList, "list", false, "")
	flag.BoolVar(&showList, "l", false, "")
	flag.StringVar(&output, "output", "", "")
	flag.StringVar(&output, "o", "", "")
	flag.StringVar(&variables, "variables", "", "")
	flag.StringVar(&variables, "v", "", "")
	flag.StringVar(&logLevel, "log-level", "info", "")
	flag.VisitAll(flagFilter(flagx.EnvToFlag))
	flag.VisitAll(flagFilter(flagx.EnvToFlagWithPrefix("QUERY_RUNNER_")))
	flag.Parse()
	filter.SetMinLevel(logutils.LogLevel(strings.ToLower(logLevel)))
	if config == "" {
		config = "~/.config/query-runner/"
	}
	var queries queryrunner.PreparedQueries
	if err := hclconfig.Load(&queries, config); err != nil {
		return err
	}
	if showList {
		fmt.Println("query list:")
		for _, query := range queries {
			fmt.Printf("\t%s\t%s\t%s\n", query.Name(), query.RunnerType(), query.Description())
		}
		return nil
	}
	if strings.HasPrefix(os.Getenv("AWS_EXECUTION_ENV"), "AWS_Lambda") || os.Getenv("AWS_LAMBDA_RUNTIME_API") != "" {
		log.Println("[info] run on AWS Lambda runtime")
		lambda.Start(func(ctx context.Context, p *params) (*response, error) {
			resp := &response{
				Results: make([]*queryrunner.QueryResult, len(p.Queries)),
			}
			eg, egctx := errgroup.WithContext(ctx)
			for i, queryName := range p.Queries {
				query, ok := queries.Get(queryName)
				if !ok {
					return nil, fmt.Errorf("query `%s` is not found, skip this query", queryName)
				}
				index := i
				eg.Go(func() error {
					log.Printf("[debug] start run `%s` runner type `%s`", query.Name(), query.RunnerType())
					result, err := query.Run(egctx, p.MarshalCTYValues(), nil)
					if err != nil {
						return err
					}
					log.Printf("[debug] finish run `%s` runner type `%s`", query.Name(), query.RunnerType())
					resp.Results[index] = result
					return nil
				})
			}
			if err := eg.Wait(); err != nil {
				return nil, err
			}
			return resp, nil
		})
	}
	var p params
	if variables != "" {
		decoder := json.NewDecoder(strings.NewReader(variables))
		if err := decoder.Decode(&p.Variables); err != nil {
			return err
		}
	}
	if flag.NArg() > 0 {
		p.Queries = flag.Args()
	} else {
		decoder := json.NewDecoder(os.Stdin)
		if err := decoder.Decode(&p); err != nil {
			return err
		}
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	defer cancel()
	eg, egctx := errgroup.WithContext(ctx)
	for _, queryName := range p.Queries {
		query, ok := queries.Get(queryName)
		if !ok {
			log.Printf("[warn] query `%s` is not found, skip this query", queryName)
		}
		eg.Go(func() error {
			log.Printf("[debug] start run `%s` runner type `%s`", query.Name(), query.RunnerType())
			result, err := query.Run(egctx, p.MarshalCTYValues(), nil)
			if err != nil {
				return err
			}
			log.Printf("[debug] finish run `%s` runner type `%s`", query.Name(), query.RunnerType())
			switch output {
			case "table":
				io.WriteString(os.Stdout, result.ToTable())
			case "markdown":
				io.WriteString(os.Stdout, result.ToMarkdownTable())
			case "borderless":
				io.WriteString(os.Stdout, result.ToBorderlessTable())
			case "vertical":
				io.WriteString(os.Stdout, result.ToVertical())
			default:
				io.WriteString(os.Stdout, result.ToJSON())
			}
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return err
	}
	return nil
}

func flagFilter(visitFunc func(f *flag.Flag)) func(f *flag.Flag) {
	return func(f *flag.Flag) {
		if len(f.Name) <= 1 {
			return
		}
		visitFunc(f)
	}
}

type params struct {
	Queries   []string        `json:"queries"`
	Variables json.RawMessage `json:"variables,omitempty"`

	once  sync.Once
	cache map[string]cty.Value
}

func (p *params) MarshalCTYValues() map[string]cty.Value {
	p.once.Do(func() {
		if p.Variables == nil {
			p.cache = map[string]cty.Value{
				"var": cty.NullVal(cty.DynamicPseudoType),
			}
			return
		}
		ctx := hclconfig.NewEvalContext()
		src := []byte(`jsondecode(`)
		bs, _ := json.Marshal(string(p.Variables))
		src = append(src, bs...)
		src = append(src, []byte(`)`)...)
		expr, diags := hclsyntax.ParseExpression(src, "", hcl.InitialPos)
		if diags.HasErrors() {
			log.Println("[warn] params variables parse expression:", diags.Error())
			return
		}
		value, diags := expr.Value(ctx)
		if diags.HasErrors() {
			log.Println("[warn] params variables eval value:", diags.Error())
			return
		}
		p.cache = map[string]cty.Value{
			"var": value,
		}
	})
	return p.cache
}

type response struct {
	Results []*queryrunner.QueryResult `json:"results"`
}
