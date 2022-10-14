query_runner "cloudwatch_logs_insights" "default" {
  region = "ap-northeast-1"
}

query "lambda_logs" {
  runner     = query_runner.cloudwatch_logs_insights.default
  start_time = now() - duration("15m")
  end_time   = now()
  query      = <<EOT
fields @timestamp, @message
    | sort @timestamp desc
    | limit 5
EOT
  log_group_names = [
    "/aws/lambda/${var.function_name}",
  ]
}
