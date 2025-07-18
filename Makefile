export HOST := 0.0.0.0:9090
export DEBUG := false

.PHONY: vulncheck fmtcheck all run up down logs

all: up

up:
	docker-compose up -d

down:
	docker-compose down

logs:
	docker-compose logs -f

vulncheck:
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

fmtcheck:
	@unformatted=$$(go run mvdan.cc/gofumpt@latest -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "Files not gofumpt formatted:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi