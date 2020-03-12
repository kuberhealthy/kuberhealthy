resourceQuotaCheck: buildResourceQuotaCheck pushResourceQuotaCheck

buildResourceQuotaCheck:
	docker build -t quay.io/comcast/resource-quota-check:1.0.0 -f cmd/resource-quota-check/Dockerfile .

pushResourceQuotaCheck:
	docker push quay.io/comcast/resource-quota-check:1.0.0