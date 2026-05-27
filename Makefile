dev:
	@export $$(grep -v '^#' .env | xargs) && go run github.com/cosmtrek/air@v1.43.0 \
		--build.cmd "make build" --build.bin "./clear-renovate-notifications" --build.delay "100" \
		--build.exclude_dir "" \
		--build.include_ext "go" \
		--misc.clean_on_exit "true"

build:
	@go build -o ./clear-renovate-notifications .

run: build
	@export $$(grep -v '^#' .env | xargs) && ./clear-renovate-notifications

clean:
	@rm -f clear-renovate-notifications*

test:
	@go test ./...

format:
	@go mod tidy -v
	@go fmt ./...

vet:
	@go vet ./...

push:
	@make format
	@make vet
	@make test
	@git add -A
	@git commit
	@git push --no-verify
