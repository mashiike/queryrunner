## Feature: Cloudwatch logs insights Query runner

sample configuration

```
query_runner "cloudwatch_logs_insights" "default" {
  region = "ap-northeast-1"
}

query "cw_logs" {
  runner = query_runner.cloudwatch_logs_insights.default
  start_time = now() - duration("15m")
  query  = <<EOT
fields @timestamp, @message
| sort @timestamp desc
| limit 20
EOT
  log_group_names = [
    "<your log group name>"
  ]
}
```

