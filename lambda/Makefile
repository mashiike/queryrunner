export AWS_ACCOUNT_ID := $(shell aws sts get-caller-identity --query 'Account' --output text)

bootstrap: ../cmd/query-runner/*.go ../*.go ../go.*
	GOARCH=amd64 GOOS=linux go build -o bootstrap ../cmd/query-runner/*.go

clean:
	rm -f bootstrap
	rm -f *_dummy.zip

deploy: bootstrap
	lambroll --log-level debug deploy --tfstate .terraform/terraform.tfstate

invoke:
	cat params.json | lambroll --log-level debug invoke --tfstate .terraform/terraform.tfstate

logs:
	lambroll logs --follow --format=short --tfstate .terraform/terraform.tfstate

.PHONY: terraform/init
terraform/init:
	terraform providers lock \
		-platform=darwin_amd64 \
		-platform=darwin_arm64 \
		-platform=linux_amd64 \
		-platform=linux_arm64 \
		-platform=windows_amd64
	terraform init


.PHONY: terraform/plan
terraform/plan:
	terraform plan


.PHONY: terraform/apply
terraform/apply:
	terraform apply
