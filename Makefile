build:
	go build ./...

test:
	go test ./...

install-local:
	go build -o "$(HOME)/.local/bin/deployctl" ./cmd/deployctl
	go build -o "$(HOME)/.local/bin/deployd" ./cmd/deployd
