package rules

import (
	"errors"
	"gorm.io/gorm"
	"net/http"
	"strconv"

	"OpenWAF/internal/api"
	"OpenWAF/internal/domain"
	"github.com/gofiber/fiber/v3"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service} }

type scopeParameters struct {
	ServiceID uint `query:"serviceId" description:"Omit for global scope"`
}
type ruleParameters struct {
	ID uint `path:"id" minimum:"1"`
}

func (h *Handler) Register(router *api.Router, auth fiber.Handler) {
	r := router.Group("/rules", auth, func(c fiber.Ctx) error {
		user, ok := c.Locals("authUser").(domain.User)
		if !ok || user.Role != "admin" {
			return fiber.ErrForbidden
		}
		c.Set("Cache-Control", "no-store")
		return c.Next()
	})
	r.Handle(http.MethodGet, "/", api.Operation{ID: "listRules", Summary: "List custom rules in global or service scope", Parameters: scopeParameters{}, Response: []domain.Rule{}, Session: true, Errors: []int{400, 401, 403, 404}}, h.list)
	r.Handle(http.MethodPost, "/", api.Operation{ID: "createRule", Summary: "Create a global or service custom rule", Request: RuleInput{}, Response: domain.Rule{}, Status: 201, Session: true, Errors: []int{400, 401, 403, 404, 415}}, h.create)
	r.Handle(http.MethodGet, "/catalog", api.Operation{ID: "ruleCatalog", Summary: "List bundled CRS, engine and patch rules", Response: catalogResponse{}, Session: true, Errors: []int{401, 403}}, func(c fiber.Ctx) error {
		return c.JSON(catalogResponse{CRSVersion: CRSVersion, Rules: h.service.Catalog})
	})
	r.Handle(http.MethodGet, "/policy", api.Operation{ID: "getRulePolicy", Summary: "Get effective global or service policy", Parameters: scopeParameters{}, Response: PolicyView{}, Session: true, Errors: []int{400, 401, 403, 404}}, h.policy)
	r.Handle(http.MethodPut, "/policy", api.Operation{ID: "replaceRulePolicy", Summary: "Replace global policy or set service override", Parameters: scopeParameters{}, Request: PolicyInput{}, Response: PolicyView{}, Session: true, Errors: []int{400, 401, 403, 404, 415}}, h.savePolicy)
	r.Handle(http.MethodDelete, "/policy", api.Operation{ID: "resetRulePolicy", Summary: "Remove service policy override and inherit global", Parameters: scopeParameters{}, Status: 204, Session: true, Errors: []int{400, 401, 403, 404}}, h.resetPolicy)
	r.Handle(http.MethodGet, "/effective", api.Operation{ID: "effectiveRules", Summary: "Get merged rules and policy for a service", Parameters: scopeParameters{}, Response: effectiveResponse{}, Session: true, Errors: []int{400, 401, 403, 404}}, h.effective)
	r.Handle(http.MethodGet, "/:id", api.Operation{ID: "getRule", Summary: "Get custom rule", Parameters: ruleParameters{}, Response: domain.Rule{}, Session: true, Errors: []int{400, 401, 403, 404}}, h.get)
	r.Handle(http.MethodPut, "/:id", api.Operation{ID: "replaceRule", Summary: "Replace custom rule without changing scope", Parameters: ruleParameters{}, Request: RuleInput{}, Response: domain.Rule{}, Session: true, Errors: []int{400, 401, 403, 404, 415}}, h.update)
	r.Handle(http.MethodDelete, "/:id", api.Operation{ID: "deleteRule", Summary: "Delete custom rule", Parameters: ruleParameters{}, Status: 204, Session: true, Errors: []int{400, 401, 403, 404}}, h.delete)
}

type catalogResponse struct {
	CRSVersion string        `json:"crsVersion"`
	Rules      []CatalogRule `json:"rules"`
}
type effectiveResponse struct {
	PolicyView
	Rules      []domain.Rule `json:"rules"`
	CRSVersion string        `json:"crsVersion"`
}

func scope(c fiber.Ctx) (uint, error) {
	value := c.Query("serviceId")
	if value == "" {
		return 0, nil
	}
	id, err := strconv.ParseUint(value, 10, 32)
	if err != nil || id == 0 {
		return 0, domain.ValidationError("serviceId must be a positive integer")
	}
	return uint(id), nil
}
func ruleID(c fiber.Ctx) (uint, error) {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil || id == 0 {
		return 0, domain.ValidationError("Invalid rule ID")
	}
	return uint(id), nil
}
func (h *Handler) list(c fiber.Ctx) error {
	id, err := scope(c)
	if err != nil {
		return err
	}
	items, err := h.service.List(c.Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(items)
}
func (h *Handler) create(c fiber.Ctx) error {
	var in RuleInput
	if err := api.ReadJSON(c, &in); err != nil {
		return err
	}
	r, err := h.service.SaveRule(c.Context(), 0, in)
	if err != nil {
		return err
	}
	return c.Status(201).JSON(r)
}
func (h *Handler) get(c fiber.Ctx) error {
	id, err := ruleID(c)
	if err != nil {
		return err
	}
	var r domain.Rule
	err = h.service.db.WithContext(c.Context()).First(&r, id).Error
	if err != nil {
		return notFound(err)
	}
	return c.JSON(r)
}
func (h *Handler) update(c fiber.Ctx) error {
	id, err := ruleID(c)
	if err != nil {
		return err
	}
	var in RuleInput
	if err := api.ReadJSON(c, &in); err != nil {
		return err
	}
	r, err := h.service.SaveRule(c.Context(), id, in)
	if err != nil {
		return err
	}
	return c.JSON(r)
}
func (h *Handler) delete(c fiber.Ctx) error {
	id, err := ruleID(c)
	if err != nil {
		return err
	}
	if err := h.service.DeleteRule(c.Context(), id); err != nil {
		return err
	}
	return c.SendStatus(204)
}
func (h *Handler) policy(c fiber.Ctx) error {
	id, err := scope(c)
	if err != nil {
		return err
	}
	p, err := h.service.Policy(c.Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(p)
}
func (h *Handler) savePolicy(c fiber.Ctx) error {
	id, err := scope(c)
	if err != nil {
		return err
	}
	var in PolicyInput
	if err := api.ReadJSON(c, &in); err != nil {
		return err
	}
	p, err := h.service.SavePolicy(c.Context(), id, in)
	if err != nil {
		return err
	}
	return c.JSON(p)
}
func (h *Handler) resetPolicy(c fiber.Ctx) error {
	id, err := scope(c)
	if err != nil {
		return err
	}
	if err := h.service.ResetPolicy(c.Context(), id); err != nil {
		return err
	}
	return c.SendStatus(204)
}
func (h *Handler) effective(c fiber.Ctx) error {
	id, err := scope(c)
	if err != nil {
		return err
	}
	if err := h.service.checkScope(c.Context(), id); err != nil {
		return err
	}
	h.service.mu.RLock()
	defer h.service.mu.RUnlock()
	state := h.service.state
	raw, exists := state.policies[id]
	var configured *domain.RulePolicy
	if exists {
		configured = &raw
	}
	view := PolicyView{Policy: effectivePolicy(state, id), Inherited: !exists, ConfiguredPolicy: configured}
	items := []domain.Rule{}
	for _, r := range state.rules {
		if r.rule.ServiceID == nil || id != 0 && *r.rule.ServiceID == id {
			items = append(items, r.rule)
		}
	}
	return c.JSON(effectiveResponse{PolicyView: view, Rules: items, CRSVersion: CRSVersion})
}

func notFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return fiber.ErrNotFound
	}
	return err
}
