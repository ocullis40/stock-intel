.PHONY: build server cli analyze clean

# Build both binaries
build: server cli

server:
	go build -o stock-intel-server ./cmd/server

cli:
	go build -o stock-intel-cli ./cmd/cli

# Run the dashboard server
run: server
	./stock-intel-server

# Run headless analysis
analyze: cli
	./stock-intel-cli

clean:
	rm -f stock-intel-server stock-intel-cli
