//go:build dev

package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/swaggest/openapi-go"
	"github.com/swaggest/openapi-go/openapi3"
)

type document struct {
	reflector openapi3.Reflector
	err       error
}

func newDocument() *document {
	d := &document{}
	d.reflector.SpecEns().Info.WithTitle("OpenWAF API").WithVersion("0.1.0")
	d.reflector.Spec.SetAPIKeySecurity("session", "session", openapi.InCookie, "Session cookie obtained from sign-in")
	return d
}

func (d *document) add(method, path string, operation Operation) {
	if d.err != nil {
		return
	}
	path = strings.TrimSuffix(path, "/")
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if strings.HasPrefix(part, ":") {
			parts[i] = "{" + part[1:] + "}"
		}
	}
	path = strings.Join(parts, "/")
	context, err := d.reflector.NewOperationContext(method, path)
	if err != nil {
		d.err = err
		return
	}
	context.SetID(operation.ID)
	context.SetSummary(operation.Summary)
	context.SetDescription(operation.Description)
	if operation.Parameters != nil {
		context.AddReqStructure(operation.Parameters)
	}
	if operation.Request != nil {
		context.AddReqStructure(operation.Request, func(cu *openapi.ContentUnit) {
			cu.ContentType = "application/json"
			cu.Customize = func(content openapi.ContentOrReference) {
				content.(*openapi3.RequestBodyOrRef).RequestBody.WithRequired(true)
			}
		})
	}
	status := operation.Status
	if status == 0 {
		status = http.StatusOK
	}
	context.AddRespStructure(operation.Response, func(cu *openapi.ContentUnit) { cu.HTTPStatus = status })
	if operation.Session {
		context.AddSecurity("session")
	}
	for _, code := range append(operation.Errors, http.StatusInternalServerError) {
		context.AddRespStructure(ErrorResponse{}, func(cu *openapi.ContentUnit) { cu.HTTPStatus = code })
	}
	if err := d.reflector.AddOperation(context); err != nil {
		d.err = fmt.Errorf("document %s %s: %w", method, path, err)
	}
}

func (r *Router) Schema() ([]byte, error) {
	if r.document.err != nil {
		return nil, r.document.err
	}
	return json.MarshalIndent(r.document.reflector.Spec, "", "  ")
}

func (r *Router) Publish(path string) error {
	schema, err := r.Schema()
	if err != nil {
		return err
	}
	r.Get(path, func(c fiber.Ctx) error {
		c.Set("Cache-Control", "no-store")
		c.Type("json")
		return c.Send(schema)
	})
	return nil
}
