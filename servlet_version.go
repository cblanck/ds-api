package main

import (
	"fmt"
	"net/http"
)

type VersionServlet struct {
}

func NewVersionServlet() *VersionServlet {
	t := new(VersionServlet)
	return t
}

func (t *VersionServlet) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "{ \"success\":1, \"return\":{ \"build\":\"%s\"} }", build_version)
}
