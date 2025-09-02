IMAGE ?= ruby-kh-check:latest

build:
	docker build -t \$(IMAGE) .

push:
	docker push \$(IMAGE)

check:
	ruby -c exampleCheck.rb
