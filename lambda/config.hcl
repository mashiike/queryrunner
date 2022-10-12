query_runner "cloudwatch_logs_insights" "default" {
  region = "ap-northeast-1"
}

query "lambda_logs" {
  runner     = query_runner.cloudwatch_logs_insights.default
  start_time = strftime_in_zone("%Y-%m-%dT%H:%M:%S%z", "UTC", now() - duration("15m"))
  end_time   = strftime_in_zone("%Y-%m-%dT%H:%M:%S%z", "UTC", now())
  query      = <<EOT
fields @timestamp, @message
    | sort @timestamp desc
    | limit 5
EOT
  log_group_names = [
     "/aws/lambda/${var.function_name}",
  ]
}
