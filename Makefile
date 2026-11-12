.PHONY: build test smoke up down seed run

build:
	go build ./...

test:
	go test ./...

smoke:
	./scripts/smoke.sh

up:
	docker compose up -d --build

down:
	docker compose down -v

seed:
	go run ./cmd/seed

run:
	go run ./cmd/server
