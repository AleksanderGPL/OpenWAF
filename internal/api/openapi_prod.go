//go:build !dev

package api

type document struct{}

func newDocument() *document { return &document{} }

func (*document) add(string, string, Operation) {}
