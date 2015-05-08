# DegreeSheep API Component
## API that powers DegreeSheep

# Structural overview

The API for degree sheep is comrpised of a generic HTTP request handling
system, and a series of servlets that handle a group of tasks. These servlets
are loaded into a map at runtime, and when a request comes in a servlet that
matches it is looked for. If one is found, reflection is used to see if it has
the method requested. If the method exists and is exported, it is called. The
result of the API call is then optionally cached (if the method name,
internally, is prefixed with Cacheable) and returned. Database interaction
occurs in Schema.go, which contains methods for fully retrieving each datatype.

It is recommended that for a production system this code is deployed through
git hooks, an example of which can be found in the
[dockerfile](https://github.com/rschlaikjer/ds-docker/) that builds this
system.

# Usage

Use either `make` or `go get . && go build` to fetch dependencies and build the
api binary.

When deployed, the binary must have a server.gcfg file in the same directory,
from which it will read the SQL, SMTP, Memcached and other settings.
