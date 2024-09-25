.PHONY: build pack test run clean

build:
	GOARCH=arm64 GOOS=darwin go build -ldflags "-s -w" -o httpproxy-darwin-arm64
	GOARCH=amd64 GOOS=darwin go build -ldflags "-s -w" -o httpproxy-darwin-amd64
	GOARCH=amd64 GOOS=linux go build -ldflags "-s -w" -o httpproxy-linux-amd64
	GOARCH=amd64 GOOS=windows go build -ldflags "-s -w" -o httpproxy-windows-amd64.exe

transform = s/httpproxy[^\.]*/httpproxy/

pack: build
	tar --transform '$(transform)' -cvzf release-darwin-arm64.tar.gz httpproxy-darwin-arm64 *.default
	tar --transform '$(transform)' -cvzf release-darwin-amd64.tar.gz httpproxy-darwin-amd64 *.default
	tar --transform '$(transform)' -cvzf release-linux-amd64.tar.gz httpproxy-linux-amd64 *.default
	tar --transform '$(transform)' -cvzf release-windows-amd64.tar.gz httpproxy-windows-amd64.exe *.default

test:
	go test -v ./...
	go build -v ./...
	go clean

run:
	go run .

clean:
	go clean
	rm httpproxy*
