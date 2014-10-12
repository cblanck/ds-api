all:
	go get "code.google.com/p/gcfg"
	go get "github.com/rschlaikjer/go-apache-logformat"
	go get "github.com/go-sql-driver/mysql"
	go build -ldflags "-X main.build_version ${VERSION}${VERSION_DIRTY}"
