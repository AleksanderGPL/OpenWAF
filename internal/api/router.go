package api

import "github.com/gofiber/fiber/v3"

type Operation struct {
	ID                  string
	Summary             string
	Description         string
	Request             any
	Parameters          any
	ResponseContentType string
	Response            any
	Status              int
	Errors              []int
	Session             bool
}

type ErrorResponse struct {
	Message string `json:"message" required:"true"`
}

type Router struct {
	fiber.Router
	prefix   string
	document *document
}

func New(router fiber.Router) *Router {
	return &Router{Router: router, document: newDocument()}
}

func (r *Router) Group(prefix string, handlers ...any) *Router {
	return &Router{Router: r.Router.Group(prefix, handlers...), prefix: r.prefix + prefix, document: r.document}
}

func (r *Router) Handle(method, path string, operation Operation, handler any, handlers ...any) {
	r.document.add(method, r.prefix+path, operation)
	r.Router.Add([]string{method}, path, handler, handlers...)
}
