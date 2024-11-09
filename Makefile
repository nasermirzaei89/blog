CGO_ENABLED=1

default:
	echo "Hello Future"

.which-air:
	which air || go install github.com/air-verse/air@latest

run:
	air

format:
	gofmt -w -s .
