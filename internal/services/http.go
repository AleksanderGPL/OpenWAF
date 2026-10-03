package services

import (
	"net/http"
	"strconv"

	"OpenWAF/internal/api"
	"OpenWAF/internal/domain"
	"github.com/gofiber/fiber/v3"
)

type serviceParameters struct {
	ID uint `path:"id" minimum:"1"`
}

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

func (h *Handler) Register(router *api.Router, requireAuth fiber.Handler) {
	routes := router.Group("/services", requireAuth, func(c fiber.Ctx) error {
		user, ok := c.Locals("authUser").(domain.User)
		if !ok || user.Role != "admin" {
			return fiber.ErrForbidden
		}
		c.Set("Cache-Control", "no-store")
		return c.Next()
	})
	routes.Handle(http.MethodGet, "/", api.Operation{
		ID: "listServices", Summary: "List services", Description: "Requires an admin session.",
		Response: []domain.Service{}, Session: true, Errors: []int{401, 403},
	}, h.list)
	routes.Handle(http.MethodPost, "/", api.Operation{
		ID: "createService", Summary: "Create a service", Description: "Requires an admin session. Configuration takes effect immediately.",
		Request: Input{}, Response: domain.Service{}, Status: 201, Session: true, Errors: []int{400, 401, 403, 409, 415},
	}, h.create)
	routes.Handle(http.MethodGet, "/:id", api.Operation{
		ID: "getService", Summary: "Get a service", Description: "Requires an admin session.",
		Parameters: serviceParameters{}, Response: domain.Service{}, Session: true, Errors: []int{400, 401, 403, 404},
	}, h.get)
	routes.Handle(http.MethodPut, "/:id", api.Operation{
		ID: "updateService", Summary: "Replace service configuration", Description: "Requires an admin session. Omitted enabled defaults to true; omitted skipTlsVerify defaults to false.",
		Parameters: serviceParameters{}, Request: Input{}, Response: domain.Service{}, Session: true, Errors: []int{400, 401, 403, 404, 409, 415},
	}, h.update)
	routes.Handle(http.MethodDelete, "/:id", api.Operation{
		ID: "deleteService", Summary: "Delete a service", Description: "Requires an admin session.",
		Parameters: serviceParameters{}, Status: 204, Session: true, Errors: []int{400, 401, 403, 404},
	}, h.delete)
}

func (h *Handler) list(c fiber.Ctx) error {
	services, err := h.service.List(c.Context())
	if err != nil {
		return err
	}
	return c.JSON(services)
}

func serviceID(c fiber.Ctx) (uint64, error) {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil || id == 0 {
		return 0, domain.ValidationError("Invalid service ID")
	}
	return id, nil
}

func (h *Handler) get(c fiber.Ctx) error {
	id, err := serviceID(c)
	if err != nil {
		return err
	}
	service, err := h.service.Get(c.Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(service)
}

func (h *Handler) create(c fiber.Ctx) error {
	var input Input
	if err := api.ReadJSON(c, &input); err != nil {
		return err
	}
	service, err := h.service.Create(c.Context(), input)
	if err != nil {
		return err
	}
	return c.Status(http.StatusCreated).JSON(service)
}

func (h *Handler) update(c fiber.Ctx) error {
	id, err := serviceID(c)
	if err != nil {
		return err
	}
	existing, err := h.service.Get(c.Context(), id)
	if err != nil {
		return err
	}
	var input Input
	if err := api.ReadJSON(c, &input); err != nil {
		return err
	}
	service, err := h.service.Update(c.Context(), existing, input)
	if err != nil {
		return err
	}
	return c.JSON(service)
}

func (h *Handler) delete(c fiber.Ctx) error {
	id, err := serviceID(c)
	if err != nil {
		return err
	}
	if err := h.service.Delete(c.Context(), id); err != nil {
		return err
	}
	return c.SendStatus(http.StatusNoContent)
}
