## Feature: Redshift data Query runner

sample configuration

```hcl
query_runner "redshift_data" "default" {
    cluster_identifier = "warehouse"
    database           = "dev"
    db_user            = "admin"
}

query "alb_target_5xx_info" {
    runner = query_runner.redshift_data.default
    sql = <<EOQ
SELECT *
FROM access_logs
WHERE status BETWEEN 500 AND 599
    AND "time" BETWEEN 
        getdate() - interval '15 minutes'
        AND getdate()
LIMIT 200
EOQ
}
```

### query_runner block

Specify the target Redshift to query.
There are several ways to specify.

#### with secrets_arn 

```hcl
query_runner "redshift_data" "default" {
    secrets_arn = "arn:aws:secretsmanager:ap-northeast-1:xxxxxxxxxxxx:secret:test-1O5wUG"
}
```

#### with cluster_identifier, db_user, database

only provisioned cluster

```hcl
query_runner "redshift_data" "default" {
    cluster_identifier = "warehouse"
    database           = "dev"
    db_user            = "admin"
}
```

#### with workgroup_name, database

only serverless workgroup

```hcl
query_runner "redshift_data" "default" {
    workgroup_name = "default"
    database       = "dev"
}
```



