all:
	go get "code.google.com/p/gcfg"
	go get "github.com/rschlaikjer/go-apache-logformat"
	go get "github.com/go-sql-driver/mysql"
	go get "code.google.com/p/go-uuid/uuid"
	go get "code.google.com/p/go.crypto/pbkdf2"
	go get "github.com/bradfitz/gomemcache/memcache"
	go build -o degreed -ldflags "-X main.build_version ${VERSION}${VERSION_DIRTY}"
