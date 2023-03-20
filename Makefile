default: help
SHELL := /bin/bash
IS_RUNNING=`docker compose ls | grep estoraje-go.*running | wc -l`

.PHONY: help
help: # Show help for each of the Makefile recipes.
	@grep -E '^[a-zA-Z0-9 -]+:.*#'  Makefile | sort | while read -r l; do printf "\033[1;32m$$(echo $$l | cut -f 1 -d':')\033[00m:$$(echo $$l | cut -f 2- -d'#')\n"; done

.PHONY: run
run: # Run estoraje cluster for development.
	docker compose --profile cluster up --remove-orphans

.PHONY: run-alone
run-alone: # Run one estoraje node alone for development.
	docker compose --profile alone up

.PHONY: stop
stop: # Run estoraje cluster for development.
	docker compose stop

.PHONY: restart
restart: | stop run # Run estoraje cluster for development.

.PHONY: build
build: # Build docker images
	 docker compose --profile cluster build --force-rm --progress=plain

.PHONY: test
test: # Run unit tets.
	go test

.PHONY: bench
bench: # Run unit tets.
	go test -bench .

.PHONY: acceptance
acceptance: # Run acceptance tests.
	test/acceptance.sh

.PHONY: acceptance-cluster
acceptance-cluster: # Run acceptance tests on cluster.
	test/acceptance.sh http://localhost:7000

.PHONY: all-tests
all-tests: | acceptance test # Run acceptance and unit tests.

.PHONY: cluster-status
cluster-status: # Show cluster status
	curl localhost:7000/_cluster_status | jq

.PHONY: add-samples
add-samples: # Add a number of random values
	test/add-samples.sh http://localhost:7000 $(number)

.PHONY: clear
clear: # Clear data from nodes and etcd cluster info
	rm -rf data etcd_conf tmp/{1,2,3,4,5}/*
