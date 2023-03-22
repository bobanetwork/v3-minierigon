erigon:
	cd cmd/minierigon && go build
.PHONY: erigon

test:
	cd cmd/minierigon && ./minierigon 4
.PHONY: test