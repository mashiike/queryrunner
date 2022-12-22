query_runner "redshift_data" "provisioned" {
  cluster_identifier = "warehouse"
  database           = "dev"
  db_user            = "admin"
}

query "error_logs" {
  runner = query_runner.redshift_data.provisioned
  sql    = "SELECT * FROM hoge"
}
