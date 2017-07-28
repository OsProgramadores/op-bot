build:
	date +'%N' >cachebuster
	docker build -t op-docker/op-bot .
	rm -f cachebuster

force:
	date +'%N' >cachebuster
	docker build --no-cache -t op-docker/op-bot .
	rm -f cachebuster

.PHONY: build
