deploy: build push

build: 
	docker build -t chrishirsch/kuberhealthy-storage-check:v0.0.1 -f Dockerfile ../../

push:
	docker push chrishirsch/kuberhealthy-storage-check:v0.0.1

apply:
	kubectl apply -f storage-check.yaml  

clean:
	rm -f ./storage-check
	kubectl delete -f storage-check.yaml
