all:
	go get "code.google.com/p/gcfg"
	go build -ldflags "-X main.build_version ${VERSION}${VERSION_DIRTY}"
