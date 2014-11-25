package main

import (
	"net/http"
)

type VersionServlet struct {
}

func NewVersionServlet() *VersionServlet {
	t := new(VersionServlet)
	return t
}

func (t *VersionServlet) ServeHTTP(w http.ResponseWriter, r *http.Request) interface{} {
	return build_version
}
