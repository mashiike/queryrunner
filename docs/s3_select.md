## Feature: S3 select Query runner

sample configuration

```hcl
query_runner "s3_select" "default" {
  region = "ap-northeast-1"
}

query "alb_5xx_logs" {
  runner            = query_runner.s3_select.default
  bucket_name       = "your-bucket"
  object_key_prefix = "alb/AWSLogs/0123456789012/elasticloadbalancing/ap-northeast-1/${strftime("%Y/%m/%d", now())}/"
  compression_type  = "GZIP"
  csv {
    field_delimiter  = " "
    record_delimiter = "\n"
  }
  expression = file("get_alb_5xx_log.sql")
}
```

### query runner block

aws region only: Required

### query block

When querying uncompressed json lines, the following is used

```hcl
query "logs" {
  runner            = query_runner.s3_select.default
  bucket_name       = "your-bucket"
  object_key_prefix = "application-logs/${strftime("%Y/%m/%d", now())}/"
  compression_type  = "NONE"
  json {
    type = "LINES"
  }
  expression = file("logs.sql")
}
```


in the case of Parquet

```hcl
query "logs" {
  runner            = query_runner.s3_select.default
  bucket_name       = "your-bucket"
  object_key_prefix = "application-logs/${strftime("%Y/%m/%d", now())}/"
  compression_type  = "NONE"
  parquet {}
  expression = file("logs.sql")
}
```
