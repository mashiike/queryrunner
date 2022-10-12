
resource "aws_iam_role" "query_runner" {
  name = "query_runner_lambda"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Sid    = ""
        Principal = {
          Service = "lambda.amazonaws.com"
        }
      }
    ]
  })
}

resource "aws_iam_policy" "query_runner" {
  name   = "query_runner"
  path   = "/"
  policy = data.aws_iam_policy_document.query_runner.json
}

resource "aws_iam_role_policy_attachment" "query_runner" {
  role       = aws_iam_role.query_runner.name
  policy_arn = aws_iam_policy.query_runner.arn
}

data "aws_iam_policy_document" "query_runner" {
  statement {
    actions = [
      "ssm:GetParameter*",
      "ssm:DescribeParameters",
      "ssm:List*",
      "logs:GetQueryResults",
      "logs:StartQuery",
      "logs:StopQuery",
      "logs:GetLog*",
      "logs:CreateLogGroup",
      "logs:CreateLogStream",
      "logs:PutLogEvents",
    ]
    resources = ["*"]
  }
}
