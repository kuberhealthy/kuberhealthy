resourceQuotaCheck: buildResourceQuotaCheck pushResourceQuotaCheck

buildResourceQuotaCheck:
	docker build -t quay.io/comcast/resource-quota-check:1.0.1 -f Dockerfile ../../

pushResourceQuotaCheck:
	docker push quay.io/comcast/resource-quota-check:1.0.1