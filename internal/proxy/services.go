package proxy

import (
	"errors"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"OpenWAF/internal/api"
	"OpenWAF/internal/database"
	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"
)

type serviceInput struct {
	Name          string `json:"name" required:"true" description:"Trimmed nonblank name, at most 255 bytes"`
	Hostname      string `json:"hostname" required:"true" description:"Hostname or IP address without a port"`
	UpstreamURL   string `json:"upstreamUrl" required:"true" format:"uri" description:"HTTP or HTTPS origin, optionally with a port"`
	SkipTLSVerify bool   `json:"skipTlsVerify" default:"false"`
	Enabled       *bool  `json:"enabled" default:"true"`
}

type serviceParameters struct {
	ID uint `path:"id" minimum:"1"`
}

func (s *Service) Register(router *api.Router, requireAuth fiber.Handler) {
	routes := router.Group("/services", requireAuth, func(c fiber.Ctx) error {
		user, ok := c.Locals("authUser").(database.User)
		if !ok || user.Role != "admin" {
			return fiber.ErrForbidden
		}
		c.Set("Cache-Control", "no-store")
		return c.Next()
	})
	routes.Handle(http.MethodGet, "/", api.Operation{
		ID: "listServices", Summary: "List services", Description: "Requires an admin session.",
		Response: []database.Service{}, Session: true, Errors: []int{401, 403},
	}, s.list)
	routes.Handle(http.MethodPost, "/", api.Operation{
		ID: "createService", Summary: "Create a service", Description: "Requires an admin session. Configuration takes effect immediately.",
		Request: serviceInput{}, Response: database.Service{}, Status: 201, Session: true, Errors: []int{400, 401, 403, 409, 415},
	}, s.create)
	routes.Handle(http.MethodGet, "/:id", api.Operation{
		ID: "getService", Summary: "Get a service", Description: "Requires an admin session.",
		Parameters: serviceParameters{}, Response: database.Service{}, Session: true, Errors: []int{400, 401, 403, 404},
	}, s.get)
	routes.Handle(http.MethodPut, "/:id", api.Operation{
		ID: "updateService", Summary: "Replace service configuration", Description: "Requires an admin session. Omitted enabled defaults to true; omitted skipTlsVerify defaults to false.",
		Parameters: serviceParameters{}, Request: serviceInput{}, Response: database.Service{}, Session: true, Errors: []int{400, 401, 403, 404, 409, 415},
	}, s.update)
	routes.Handle(http.MethodDelete, "/:id", api.Operation{
		ID: "deleteService", Summary: "Delete a service", Description: "Requires an admin session.",
		Parameters: serviceParameters{}, Status: 204, Session: true, Errors: []int{400, 401, 403, 404},
	}, s.delete)
}

func validHostname(host string) bool {
	if net.ParseIP(host) != nil {
		return true
	}
	if len(host) == 0 || len(host) > 253 {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, char := range label {
			if !(char >= 'a' && char <= 'z' || char >= '0' && char <= '9' || char == '-') {
				return false
			}
		}
	}
	return true
}

func upstreamURL(value string) (*url.URL, error) {
	target, err := url.Parse(value)
	if err != nil || target == nil || (target.Scheme != "http" && target.Scheme != "https") || target.Host == "" || target.User != nil || (target.Path != "" && target.Path != "/") || target.RawQuery != "" || target.ForceQuery || target.Fragment != "" || !validHostname(strings.ToLower(target.Hostname())) {
		return nil, errors.New("Upstream URL must be an HTTP or HTTPS origin without credentials, a path, query, or fragment")
	}
	if port := target.Port(); port != "" {
		number, err := strconv.Atoi(port)
		if err != nil || number < 1 || number > 65535 {
			return nil, errors.New("Upstream port must be between 1 and 65535")
		}
	}
	if strings.HasSuffix(target.Host, ":") {
		return nil, errors.New("Upstream port cannot be empty")
	}
	target.Path = ""
	return target, nil
}

func readService(c fiber.Ctx) (database.Service, error) {
	mediaType, _, err := mime.ParseMediaType(c.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return database.Service{}, fiber.NewError(415, "Expected application/json")
	}
	var input serviceInput
	if err := c.Bind().JSON(&input); err != nil {
		return database.Service{}, fiber.NewError(400, "Invalid JSON body")
	}
	input.Name = strings.TrimSpace(input.Name)
	input.Hostname = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(input.Hostname)), ".")
	if input.Name == "" || len(input.Name) > 255 || !validHostname(input.Hostname) {
		return database.Service{}, fiber.NewError(400, "A name (up to 255 bytes) and a valid hostname or IP address without a port are required")
	}
	target, err := upstreamURL(strings.TrimSpace(input.UpstreamURL))
	if err != nil {
		return database.Service{}, fiber.NewError(400, err.Error())
	}
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	return database.Service{Name: input.Name, Hostname: input.Hostname, UpstreamURL: target.String(), SkipTLSVerify: input.SkipTLSVerify, Enabled: enabled}, nil
}

func serviceError(err error) error {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return fiber.NewError(409, "A service with this hostname already exists")
	}
	return err
}

func (s *Service) list(c fiber.Ctx) error {
	services := make([]database.Service, 0)
	if err := s.db.Order("id").Find(&services).Error; err != nil {
		return err
	}
	return c.JSON(services)
}

func (s *Service) find(c fiber.Ctx) (database.Service, error) {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil || id == 0 {
		return database.Service{}, fiber.NewError(400, "Invalid service ID")
	}
	var service database.Service
	err = s.db.Where("id = ?", id).First(&service).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return service, fiber.ErrNotFound
	}
	return service, err
}

func (s *Service) get(c fiber.Ctx) error {
	service, err := s.find(c)
	if err != nil {
		return err
	}
	return c.JSON(service)
}

func (s *Service) create(c fiber.Ctx) error {
	service, err := readService(c)
	if err != nil {
		return err
	}
	if err := s.db.Create(&service).Error; err != nil {
		return serviceError(err)
	}
	return c.Status(201).JSON(service)
}

func (s *Service) update(c fiber.Ctx) error {
	existing, err := s.find(c)
	if err != nil {
		return err
	}
	service, err := readService(c)
	if err != nil {
		return err
	}
	service.ID, service.CreatedAt = existing.ID, existing.CreatedAt
	result := s.db.Model(&database.Service{}).Where("id = ?", existing.ID).Select("Name", "Hostname", "UpstreamURL", "SkipTLSVerify", "Enabled", "UpdatedAt").Updates(&service)
	if result.Error != nil {
		return serviceError(result.Error)
	}
	if result.RowsAffected == 0 {
		return fiber.ErrNotFound
	}
	return c.JSON(service)
}

func (s *Service) delete(c fiber.Ctx) error {
	service, err := s.find(c)
	if err != nil {
		return err
	}
	if err := s.db.Delete(&service).Error; err != nil {
		return err
	}
	return c.SendStatus(204)
}
