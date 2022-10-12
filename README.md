# queryrunner
![Latest GitHub release](https://img.shields.io/github/release/mashiike/queryrunner.svg)
![Github Actions test](https://github.com/mashiike/queryrunner/workflows/Test/badge.svg?branch=main)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](https://github.com/mashiike/queryrunner/blob/master/LICENSE)

query-runner is a helper tool that makes querying several AWS services convenient

## Usage 

```
query-runner is a helper tool that makes querying several AWS services convenient

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
```

sample config and sample command is following

```
$ query-runner --variables '{"function_name": "helloworld"}' lambda_hello_world
```

~/.config/query-runner/config.hcl
```hcl
query_runner "cloudwatch_logs_insights" "default" {
  region = "ap-northeast-1"
}

query "lambda_logs" {
  runner     = query_runner.cloudwatch_logs_insights.default
  start_time = strftime_in_zone("%Y-%m-%dT%H:%M:%S%z", "UTC", now() - duration("15m"))
  end_time   = strftime_in_zone("%Y-%m-%dT%H:%M:%S%z", "UTC", now())
  query      = <<EOT
fields @timestamp, @message
    | parse @message "[*] *" as loggingType, loggingMessage
    | filter loggingType = "${var == null ? "info" : var.log_level}"
    | display @timestamp, loggingType, loggingMessage
    | sort @timestamp desc
    | limit 2000
EOT
  log_group_name = "/aws/lambda/${var.function_name}"
}
```

query with cloudwatch logs insights

For other query runner, please refer to [docs](docs/).

## Install 

#### Homebrew (macOS and Linux)

```console
$ brew install mashiike/tap/queryrunner
```

### Binary packages

[Releases](https://github.com/mashiike/queryrunner/releases)

### Usage as go Library

```go
package main 

import (
	"log"

	"github.com/mashiike/hclconfig"
	"github.com/mashiike/queryrunner"
)

func main() {
	var queries queryrunner.PreparedQueries
	if err := hclconfig.Load(&queries, "./"); err != nil {
		return err
	}
    query, ok := queries.Get("<your query name>")
	if !ok {
        log.Fatalln("query not found")
	}
	result, err := query.Run(egctx, p.MarshalCTYValues(), nil)
    if err != nil {
            log.Fatalln(err)
    }
	log.Println(result.ToTable())
}
```


## LICENSE

MIT License

Copyright (c) 2022 IKEDA Masashi
