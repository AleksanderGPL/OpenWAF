package telemetry

import (
	"encoding/csv"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"OpenWAF/internal/api"
	"OpenWAF/internal/domain"
	"github.com/gofiber/fiber/v3"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

type statsParameters struct {
	Range     string `query:"range" enum:"24h,7d,30d" default:"24h"`
	From      string `query:"from" format:"date-time"`
	To        string `query:"to" format:"date-time"`
	ServiceID uint64 `query:"serviceId" minimum:"1"`
}

type logParameters struct {
	statsParameters
	Action string `query:"action" enum:"allowed,blocked"`
	IP     string `query:"ip"`
	Method string `query:"method"`
	Status int    `query:"status" minimum:"100" maximum:"599"`
	RuleID string `query:"ruleId"`
	Search string `query:"search" maxLength:"256"`
	Page   int    `query:"page" minimum:"1" maximum:"100000" default:"1"`
	Limit  int    `query:"limit" minimum:"1" maximum:"100" default:"50"`
}

type logIDParameters struct {
	ID uint64 `path:"id" minimum:"1"`
}

func (h *Handler) Register(router *api.Router, requireAuth fiber.Handler) {
	admin := func(c fiber.Ctx) error {
		user, ok := c.Locals("authUser").(domain.User)
		if !ok || user.Role != "admin" {
			return fiber.ErrForbidden
		}
		c.Set("Cache-Control", "no-store")
		return c.Next()
	}
	routes := router.Group("/stats", requireAuth, admin)
	logRoutes := router.Group("/logs", requireAuth, admin)
	settingsRoutes := router.Group("/settings", requireAuth, admin)
	description := "Requires an admin session. UTC time window includes from and excludes to. Defaults to the last 24 hours; explicit from/to must be supplied together, without range, and span at most 366 days. Statistics cover retained logs only. Request bytes count body bytes read by the proxy; response bytes count body bytes written. HTTP headers and upgraded connection traffic are excluded. Latency measures request handling duration, excluding log persistence. Collection failures count failed writes since collectionStartedAt."
	for _, route := range []struct {
		path, id, summary string
		response          any
		handler           fiber.Handler
	}{
		{"/", "getStats", "Get request statistics and previous period", Summary{}, h.summary},
		{"/traffic", "getTraffic", "Get hourly or daily traffic buckets", Traffic{}, h.traffic},
		{"/threats", "getThreats", "Get blocked requests grouped by rule", ThreatsResponse{}, h.threats},
		{"/blocked-sources", "getBlockedSources", "Get the ten most blocked client IPs", SourcesResponse{}, h.sources},
	} {
		routes.Handle(http.MethodGet, route.path, api.Operation{ID: route.id, Summary: route.summary, Description: description, Parameters: statsParameters{}, Response: route.response, Session: true, Errors: []int{400, 401, 403}}, route.handler)
	}
	logRoutes.Handle(http.MethodGet, "/", api.Operation{ID: "listRequestLogs", Summary: "List request logs", Description: description + " Search matches path, hostname, IP, reason, or exact request ID. Results ordered by timestamp and ID descending.", Parameters: logParameters{}, Response: LogPage{}, Session: true, Errors: []int{400, 401, 403}}, h.logs)
	logRoutes.Handle(http.MethodGet, "/export", api.Operation{ID: "exportRequestLogs", Summary: "Export filtered request logs as CSV", Description: description + " Maximum 10000 matching rows; returns 413 when exceeded. Page and limit are ignored. Body bytes exclude HTTP headers and upgraded connection traffic.", Parameters: logParameters{}, Response: "", ResponseContentType: "text/csv", Session: true, Errors: []int{400, 401, 403, 413}}, h.export)
	logRoutes.Handle(http.MethodGet, "/:id", api.Operation{ID: "getRequestLog", Summary: "Get request log details", Parameters: logIDParameters{}, Response: domain.RequestLog{}, Session: true, Errors: []int{400, 401, 403, 404}}, h.get)
	settingsRoutes.Handle(http.MethodGet, "/", api.Operation{ID: "getSettings", Summary: "Get log retention settings", Response: domain.Settings{}, Session: true, Errors: []int{401, 403}}, h.settings)
	settingsRoutes.Handle(http.MethodPut, "/", api.Operation{ID: "updateSettings", Summary: "Update log retention settings", Description: "Requires an admin session. Retention is 1–3650 days, default 30. Saving immediately removes expired logs. Cleanup also runs at startup and hourly. Statistics use retained logs only.", Request: domain.Settings{}, Response: domain.Settings{}, Session: true, Errors: []int{400, 401, 403, 415}}, h.updateSettings)
}

func parseFilter(c fiber.Ctx, logs bool) (Filter, error) {
	now := time.Now().UTC()
	f := Filter{Window: Window{From: now.Add(-24 * time.Hour), To: now}, Page: 1, Limit: 50}
	from, to, period := c.Query("from"), c.Query("to"), c.Query("range")
	if from != "" || to != "" {
		if from == "" || to == "" || period != "" {
			return f, domain.ValidationError("Supply both from and to, without range")
		}
		var err error
		f.From, err = time.Parse(time.RFC3339Nano, from)
		if err != nil {
			return f, domain.ValidationError("Invalid from timestamp")
		}
		f.To, err = time.Parse(time.RFC3339Nano, to)
		if err != nil {
			return f, domain.ValidationError("Invalid to timestamp")
		}
		f.From, f.To = f.From.UTC(), f.To.UTC()
		if !f.From.Before(f.To) || f.To.Sub(f.From) > 366*24*time.Hour {
			return f, domain.ValidationError("Time window must be positive and at most 366 days")
		}
	} else {
		switch period {
		case "", "24h":
		case "7d":
			f.From = now.Add(-7 * 24 * time.Hour)
		case "30d":
			f.From = now.Add(-30 * 24 * time.Hour)
		default:
			return f, domain.ValidationError("Invalid range")
		}
	}
	if value := c.Query("serviceId"); value != "" {
		id, err := strconv.ParseUint(value, 10, 64)
		if err != nil || id == 0 {
			return f, domain.ValidationError("Invalid serviceId")
		}
		f.ServiceID = id
	}
	if !logs {
		return f, nil
	}
	f.Action, f.IP, f.Method, f.RuleID, f.Search = c.Query("action"), c.Query("ip"), strings.ToUpper(c.Query("method")), c.Query("ruleId"), strings.TrimSpace(c.Query("search"))
	if f.Action != "" && f.Action != "allowed" && f.Action != "blocked" {
		return f, domain.ValidationError("Invalid action")
	}
	if f.IP != "" {
		ip, err := netip.ParseAddr(f.IP)
		if err != nil || ip.Zone() != "" {
			return f, domain.ValidationError("Invalid IP address")
		}
		f.IP = ip.Unmap().String()
	}
	if len(f.Method) > 32 || len(f.RuleID) > 128 || len(f.Search) > 256 {
		return f, domain.ValidationError("Filter is too long")
	}
	for _, field := range []struct {
		name     string
		target   *int
		min, max int
	}{{"page", &f.Page, 1, 100000}, {"limit", &f.Limit, 1, 100}, {"status", &f.Status, 100, 599}} {
		if value := c.Query(field.name); value != "" {
			n, err := strconv.Atoi(value)
			if err != nil || n < field.min || n > field.max {
				return f, domain.ValidationError("Invalid " + field.name)
			}
			*field.target = n
		}
	}
	return f, nil
}

func (h *Handler) summary(c fiber.Ctx) error {
	f, err := parseFilter(c, false)
	if err != nil {
		return err
	}
	result, err := h.service.Summary(c.Context(), f)
	if err != nil {
		return err
	}
	return c.JSON(result)
}
func (h *Handler) traffic(c fiber.Ctx) error {
	f, err := parseFilter(c, false)
	if err != nil {
		return err
	}
	result, err := h.service.Traffic(c.Context(), f)
	if err != nil {
		return err
	}
	return c.JSON(result)
}
func (h *Handler) threats(c fiber.Ctx) error {
	f, err := parseFilter(c, false)
	if err != nil {
		return err
	}
	result, err := h.service.Threats(c.Context(), f)
	if err != nil {
		return err
	}
	return c.JSON(result)
}
func (h *Handler) sources(c fiber.Ctx) error {
	f, err := parseFilter(c, false)
	if err != nil {
		return err
	}
	result, err := h.service.BlockedSources(c.Context(), f)
	if err != nil {
		return err
	}
	return c.JSON(result)
}
func (h *Handler) logs(c fiber.Ctx) error {
	f, err := parseFilter(c, true)
	if err != nil {
		return err
	}
	result, err := h.service.Logs(c.Context(), f)
	if err != nil {
		return err
	}
	return c.JSON(result)
}
func (h *Handler) get(c fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil || id == 0 {
		return domain.ValidationError("Invalid request ID")
	}
	event, err := h.service.RequestLog(c.Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(event)
}
func (h *Handler) settings(c fiber.Ctx) error {
	settings, err := h.service.Settings(c.Context())
	if err != nil {
		return err
	}
	return c.JSON(settings)
}
func (h *Handler) updateSettings(c fiber.Ctx) error {
	var settings domain.Settings
	if err := api.ReadJSON(c, &settings); err != nil {
		return err
	}
	result, err := h.service.UpdateSettings(c.Context(), settings)
	if err != nil {
		return err
	}
	return c.JSON(result)
}

func csvCell(value string) string {
	trimmed := strings.TrimLeft(value, " \t\r\n")
	if trimmed != "" && strings.ContainsAny(trimmed[:1], "=+-@") {
		return "'" + value
	}
	return value
}
func (h *Handler) export(c fiber.Ctx) error {
	f, err := parseFilter(c, true)
	if err != nil {
		return err
	}
	f.Page, f.Limit, f.MaxTotal = 1, 10000, 10000
	result, err := h.service.Logs(c.Context(), f)
	if err != nil {
		return err
	}
	if result.Total > int64(f.Limit) {
		return fiber.NewError(http.StatusRequestEntityTooLarge, "Export exceeds 10000 rows; narrow the filters")
	}
	var body strings.Builder
	writer := csv.NewWriter(&body)
	if err := writer.Write([]string{"ID", "Timestamp", "Service ID", "Hostname", "IP", "Country code", "Method", "Path", "Action", "Rule ID", "Reason", "Status", "Error category", "Duration ms", "Request bytes", "Response bytes"}); err != nil {
		return err
	}
	for _, event := range result.Items {
		serviceID, country := "", ""
		if event.ServiceID != nil {
			serviceID = strconv.FormatUint(uint64(*event.ServiceID), 10)
		}
		if event.CountryCode != nil {
			country = *event.CountryCode
		}
		row := []string{strconv.FormatUint(event.ID, 10), event.Timestamp.Format(time.RFC3339Nano), serviceID, event.Hostname, event.IP, country, event.Method, event.Path, event.Action, event.RuleID, event.Reason, strconv.Itoa(event.Status), event.ErrorCategory, strconv.FormatFloat(event.DurationMs, 'f', 3, 64), strconv.FormatInt(event.RequestBytes, 10), strconv.FormatInt(event.ResponseBytes, 10)}
		for i := range row {
			row[i] = csvCell(row[i])
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return err
	}
	c.Set("Content-Type", "text/csv; charset=utf-8")
	c.Set("Content-Disposition", `attachment; filename="openwaf-request-logs.csv"`)
	return c.SendString(body.String())
}
