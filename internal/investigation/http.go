package investigation

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"OpenWAF/internal/api"
	"OpenWAF/internal/domain"
	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Handler struct{ service *Service }

func NewHandler(s *Service) *Handler { return &Handler{service: s} }

type idParams struct {
	ID string `path:"id" required:"true"`
}
type listParams struct {
	ServiceID  uint   `query:"serviceId"`
	State      string `query:"state"`
	Status     string `query:"status"`
	Severity   string `query:"severity"`
	Assessment string `query:"assessment"`
	From       string `query:"from"`
	To         string `query:"to"`
	Unread     bool   `query:"unread"`
	Page       int    `query:"page"`
	Limit      int    `query:"limit"`
}
type stateInput struct {
	State         string  `json:"state,omitempty"`
	Read          *bool   `json:"read,omitempty"`
	ResultVersion *uint64 `json:"resultVersion,omitempty"`
}
type ack struct {
	Success bool `json:"success"`
}
type InvestigationView struct {
	ID                    string                      `json:"id"`
	ServiceID             uint                        `json:"serviceId"`
	IP                    string                      `json:"ip,omitempty"`
	Title                 string                      `json:"title"`
	Summary               string                      `json:"summary"`
	Severity              string                      `json:"severity,omitempty"`
	Assessment            string                      `json:"assessment,omitempty"`
	State                 string                      `json:"state"`
	Status                string                      `json:"status"`
	Read                  bool                        `json:"read"`
	Trigger               domain.InvestigationTrigger `gorm:"serializer:json" json:"trigger"`
	FirstSeen             time.Time                   `json:"firstSeen"`
	LastSeen              time.Time                   `json:"lastSeen"`
	StartedAt             *time.Time                  `json:"startedAt,omitempty"`
	FinishedAt            *time.Time                  `json:"finishedAt,omitempty"`
	CancellationRequested bool                        `json:"cancellationRequested"`
	Error                 string                      `json:"error,omitempty"`
	ResultAvailable       bool                        `json:"resultAvailable"`
	ResultVersion         uint64                      `json:"resultVersion"`
}
type investigationList struct {
	Items       []InvestigationView `json:"items"`
	Total       int64               `json:"total"`
	Page        int                 `json:"page"`
	Limit       int                 `json:"limit"`
	LastEventID uint64              `json:"lastEventId"`
}
type evidenceView struct {
	Request   domain.RequestLog `json:"request"`
	Available bool              `json:"available"`
}
type resultView struct {
	Title           string                      `json:"title"`
	Summary         string                      `json:"summary"`
	Severity        string                      `json:"severity"`
	Assessment      string                      `json:"assessment"`
	Explanation     string                      `json:"explanation"`
	Patterns        []string                    `json:"patterns"`
	Recommendations []string                    `json:"recommendations"`
	Limitations     []string                    `json:"limitations"`
	Trigger         domain.InvestigationTrigger `json:"trigger"`
	PublishedAt     time.Time                   `json:"publishedAt"`
	Evidence        []evidenceView              `json:"evidence"`
}
type investigationDetail struct {
	InvestigationView
	Result          *resultView                 `json:"result"`
	Events          []domain.InvestigationEvent `json:"events"`
	LastEventID     uint64                      `json:"lastEventId"`
	EventsTruncated bool                        `json:"eventsTruncated"`
}
type investigationSummary struct {
	Total       int64            `json:"total"`
	Unread      int64            `json:"unread"`
	BySeverity  map[string]int64 `json:"bySeverity"`
	ByStatus    map[string]int64 `json:"byStatus"`
	LastEventID uint64           `json:"lastEventId"`
}
type settingsView struct {
	Policy    domain.AnomalySettings `json:"policy"`
	Inherited bool                   `json:"inherited"`
	AIEnabled bool                   `json:"aiEnabled"`
}

func (h *Handler) Register(router *api.Router, requireAuth fiber.Handler) {
	r := router.Group("")
	admin := func(c fiber.Ctx) error {
		user, ok := c.Locals("authUser").(domain.User)
		if !ok || user.Role != "admin" {
			return fiber.ErrForbidden
		}
		c.Set("Cache-Control", "no-store")
		return c.Next()
	}
	register := func(method, path, id, summary string, request, response, params any, status int, handler fiber.Handler) {
		r.Handle(method, path, api.Operation{ID: id, Summary: summary, Request: request, Response: response, Parameters: params, Status: status, Session: true, Errors: []int{400, 401, 403, 404, 409, 415, 429, 503}}, requireAuth, admin, handler)
	}
	for _, path := range []string{"/investigations/events", "/investigations/:id/events"} {
		p := path
		var params any
		if strings.Contains(p, ":id") {
			params = idParams{}
		}
		r.Handle(http.MethodGet, p, api.Operation{ID: strings.NewReplacer("/", "_", ":", "").Replace(p) + "Stream", Summary: "Stream investigation progress and result updates", Description: "Durable SSE updates use the investigation ID. Reconnect with Last-Event-ID or after. A disconnect does not cancel execution.", Response: domain.InvestigationEvent{}, ResponseContentType: "text/event-stream", Parameters: params, Session: true, Errors: []int{400, 401, 403, 404}}, requireAuth, admin, func(c fiber.Ctx) error { return h.stream(c, p) })
	}
	register("GET", "/investigations", "listInvestigations", "List investigations with current status and result summaries", nil, investigationList{}, listParams{}, 200, h.list)
	register("GET", "/investigations/summary", "getInvestigationsSummary", "Get unread, severity and execution counts", nil, investigationSummary{}, nil, 200, h.summary)
	register("GET", "/investigations/:id", "getInvestigation", "Get trigger, execution progress and investigation result", nil, investigationDetail{}, idParams{}, 200, h.get)
	register("PATCH", "/investigations/:id", "updateInvestigation", "Mark an investigation read, acknowledge, resolve or dismiss it", stateInput{}, InvestigationView{}, idParams{}, 200, h.update)
	register("POST", "/investigations/:id/retry", "retryInvestigation", "Queue another execution of the investigation", nil, investigationDetail{}, idParams{}, 202, h.retry)
	register("POST", "/investigations/:id/cancel", "cancelInvestigation", "Cancel the investigation's queued or active execution", nil, ack{}, idParams{}, 200, h.cancel)
	register("POST", "/investigations/:id/follow-up", "createInvestigationFollowUp", "Create or get your investigation follow-up conversation", nil, domain.AssistantConversation{}, idParams{}, 201, h.followUp)
	register("GET", "/anomaly-settings", "getAnomalySettings", "Get global or service anomaly settings", nil, settingsView{}, listParams{}, 200, h.settings)
	register("PUT", "/anomaly-settings", "updateAnomalySettings", "Set global or service anomaly settings", domain.AnomalySettings{}, settingsView{}, listParams{}, 200, h.updateSettings)
	register("DELETE", "/anomaly-settings", "resetAnomalySettings", "Restore service settings inheritance", nil, ack{}, listParams{}, 200, h.resetSettings)
}
func operator(c fiber.Ctx) uint { return c.Locals("authUser").(domain.User).ID }

const readExpression = "EXISTS (SELECT 1 FROM investigation_reads AS seen WHERE seen.investigation_id = i.id AND seen.user_id = ? AND seen.version >= COALESCE(i.result_version,0))"
const cardColumns = `i.id, i.service_id, i.ip, i.state, COALESCE(execution.trigger, i.trigger) AS trigger, i.first_seen, i.last_seen,
 COALESCE(json_extract(i.result, '$.title'), 'Traffic investigation') AS title,
 COALESCE(json_extract(i.result, '$.summary'), json_extract(i.trigger, '$.explanation'), '') AS summary,
 COALESCE(json_extract(i.result, '$.severity'), '') AS severity,
 COALESCE(json_extract(i.result, '$.assessment'), '') AS assessment,
 COALESCE(execution.status, 'detected') AS status,
 execution.started_at, execution.finished_at,
 COALESCE(execution.cancel_requested, 0) AS cancellation_requested,
 COALESCE(execution.error, '') AS error,
 i.result IS NOT NULL AND i.result != 'null' AS result_available,
 COALESCE(i.result_version,0) AS result_version, ` + readExpression + " AS read"

func investigationQuery(db *gorm.DB) *gorm.DB {
	return db.Table("investigations AS i").Joins("LEFT JOIN investigation_runs AS execution ON execution.id = (SELECT newest.id FROM investigation_runs AS newest WHERE newest.incident_id = i.id ORDER BY newest.created_at DESC, newest.id DESC LIMIT 1)")
}
func filter(c fiber.Ctx, q *gorm.DB) (*gorm.DB, error) {
	service, err := scope(c)
	if err != nil {
		return q, err
	}
	if service != 0 {
		q = q.Where("i.service_id = ?", service)
	}
	if state := c.Query("state"); state != "" {
		if !oneOf(state, "open", "acknowledged", "resolved", "dismissed") {
			return q, fiber.ErrBadRequest
		}
		q = q.Where("i.state = ?", state)
	}
	if status := c.Query("status"); status != "" {
		if !oneOf(status, "detected", "queued", "running", "completed", "failed", "cancelled") {
			return q, fiber.ErrBadRequest
		}
		q = q.Where("COALESCE(execution.status, 'detected') = ?", status)
	}
	if v := c.Query("severity"); v != "" {
		if !oneOf(v, "info", "low", "medium", "high", "critical") {
			return q, fiber.ErrBadRequest
		}
		q = q.Where("json_extract(i.result, '$.severity') = ?", v)
	}
	if v := c.Query("assessment"); v != "" {
		if !oneOf(v, "likely_malicious", "likely_benign", "inconclusive") {
			return q, fiber.ErrBadRequest
		}
		q = q.Where("json_extract(i.result, '$.assessment') = ?", v)
	}
	for _, name := range []string{"from", "to"} {
		if v := c.Query(name); v != "" {
			t, err := time.Parse(time.RFC3339, v)
			if err != nil {
				return q, domain.ValidationError(name + " must be RFC3339")
			}
			op := ">="
			if name == "to" {
				op = "<"
			}
			q = q.Where("i.last_seen "+op+" ?", t)
		}
	}
	if v := c.Query("unread"); v != "" {
		unread, err := strconv.ParseBool(v)
		if err != nil {
			return q, fiber.ErrBadRequest
		}
		expr := readExpression
		if unread {
			expr = "NOT " + expr
		}
		q = q.Where(expr, operator(c))
	}
	return q, nil
}
func (h *Handler) list(c fiber.Ctx) error {
	page, limit, err := pagination(c)
	if err != nil {
		return err
	}
	q, err := filter(c, investigationQuery(h.service.db.WithContext(c.Context())))
	if err != nil {
		return err
	}
	result := investigationList{Items: []InvestigationView{}, Page: page, Limit: limit}
	if err := h.service.db.WithContext(c.Context()).Model(&domain.InvestigationEvent{}).Select("COALESCE(MAX(id),0)").Scan(&result.LastEventID).Error; err != nil {
		return err
	}
	if err := q.Count(&result.Total).Error; err != nil {
		return err
	}
	if err := q.Select(cardColumns, operator(c)).Order("i.last_seen DESC, i.id ASC").Limit(limit).Offset((page - 1) * limit).Scan(&result.Items).Error; err != nil {
		return err
	}
	return c.JSON(result)
}
func (h *Handler) detail(ctx context.Context, user uint, id string) (investigationDetail, error) {
	result := investigationDetail{Events: []domain.InvestigationEvent{}}
	err := h.service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var item domain.Investigation
		if err := tx.First(&item, "id = ?", id).Error; err != nil {
			return notFound(err)
		}
		if err := investigationQuery(tx).Where("i.id = ?", id).Select(cardColumns, user).Scan(&result.InvestigationView).Error; err != nil {
			return err
		}

		if item.Result != nil {
			value := item.Result
			result.Result = &resultView{Title: value.Title, Summary: value.Summary, Severity: value.Severity, Assessment: value.Assessment, Explanation: value.Explanation, Patterns: append([]string{}, value.Patterns...), Recommendations: append([]string{}, value.Recommendations...), Limitations: append([]string{}, value.Limitations...), Trigger: value.Trigger, PublishedAt: value.PublishedAt, Evidence: []evidenceView{}}
			var ids []uint64
			for _, snapshot := range value.Evidence {
				ids = append(ids, snapshot.ID)
			}
			var available []domain.RequestLog
			if len(ids) > 0 {
				if err := tx.Select("id").Where("id IN ?", ids).Find(&available).Error; err != nil {
					return err
				}
			}
			exists := map[uint64]bool{}
			for _, item := range available {
				exists[item.ID] = true
			}
			for _, snapshot := range value.Evidence {
				result.Result.Evidence = append(result.Result.Evidence, evidenceView{snapshot, exists[snapshot.ID]})
			}
		}
		if err := tx.Where("incident_id = ?", id).Order("id DESC").Limit(201).Find(&result.Events).Error; err != nil {
			return err
		}
		if len(result.Events) > 200 {
			result.Events = result.Events[:200]
			result.EventsTruncated = true
		}
		if len(result.Events) > 0 {
			result.LastEventID = result.Events[0].ID
		}
		for i, j := 0, len(result.Events)-1; i < j; i, j = i+1, j-1 {
			result.Events[i], result.Events[j] = result.Events[j], result.Events[i]
		}
		for i := range result.Events {
			result.Events[i].Type = eventType(result.Events[i].Type)
		}
		return nil
	})
	return result, err
}
func (h *Handler) get(c fiber.Ctx) error {
	result, err := h.detail(c.Context(), operator(c), c.Params("id"))
	if err != nil {
		return err
	}
	return c.JSON(result)
}
func (h *Handler) update(c fiber.Ctx) error {
	var input stateInput
	if err := api.ReadJSON(c, &input); err != nil {
		return err
	}
	if input.State != "" && !oneOf(input.State, "open", "acknowledged", "resolved", "dismissed") {
		return fiber.ErrBadRequest
	}
	if input.State == "" && input.Read == nil {
		return fiber.ErrBadRequest
	}
	err := h.service.db.WithContext(c.Context()).Transaction(func(tx *gorm.DB) error {
		var item domain.Investigation
		if err := tx.First(&item, "id = ?", c.Params("id")).Error; err != nil {
			return notFound(err)
		}
		if input.Read != nil {
			if *input.Read {
				version := item.ResultVersion
				if input.ResultVersion != nil {
					version = *input.ResultVersion
				}
				if version > item.ResultVersion {
					return fiber.NewError(409, "The result version is no longer available")
				}
				read := domain.InvestigationRead{InvestigationID: item.ID, UserID: operator(c), Version: version, ReadAt: time.Now().UTC()}
				if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "investigation_id"}, {Name: "user_id"}}, DoUpdates: clause.Assignments(map[string]any{"version": gorm.Expr("MAX(investigation_reads.version, excluded.version)"), "read_at": read.ReadAt})}).Create(&read).Error; err != nil {
					return err
				}
			} else {
				if err := tx.Where("investigation_id = ? AND user_id = ?", item.ID, operator(c)).Delete(&domain.InvestigationRead{}).Error; err != nil {
					return err
				}
			}
		}
		if input.State != "" {
			if err := tx.Model(&item).Update("state", input.State).Error; err != nil {
				return err
			}
			return event(tx, domain.InvestigationRun{InvestigationID: item.ID}, "investigation.updated", input.State, "")
		}
		return nil
	})
	if err != nil {
		return err
	}
	result, err := h.detail(c.Context(), operator(c), c.Params("id"))
	if err != nil {
		return err
	}
	return c.JSON(result.InvestigationView)
}
func (h *Handler) retry(c fiber.Ctx) error {
	if !h.service.assistant.Enabled() {
		return fiber.NewError(503, "AI investigation is not configured")
	}
	var item domain.Investigation
	if err := h.service.db.WithContext(c.Context()).First(&item, "id = ?", c.Params("id")).Error; err != nil {
		return notFound(err)
	}
	p, err := h.service.policy(c.Context(), item.ServiceID)
	if err != nil {
		return err
	}
	err = h.service.db.WithContext(c.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&item, "id = ?", item.ID).Error; err != nil {
			return err
		}
		if oneOf(item.State, "resolved", "dismissed") {
			item.State = "open"
			if err := tx.Model(&item).Update("state", "open").Error; err != nil {
				return err
			}
		}
		_, err := queue(tx, &item, item.Trigger, time.Now().UTC(), p.HourlyRunLimit)
		return err
	})
	if err != nil {
		return err
	}
	c.Status(202)
	return h.get(c)
}
func (h *Handler) cancel(c fiber.Ctx) error {
	if err := h.service.Cancel(c.Context(), c.Params("id")); err != nil {
		return err
	}
	return c.JSON(ack{true})
}
func (h *Handler) summary(c fiber.Ctx) error {
	result := investigationSummary{BySeverity: map[string]int64{"info": 0, "low": 0, "medium": 0, "high": 0, "critical": 0}, ByStatus: map[string]int64{"detected": 0, "queued": 0, "running": 0, "completed": 0, "failed": 0, "cancelled": 0}}
	db := h.service.db.WithContext(c.Context())
	if err := db.Model(&domain.InvestigationEvent{}).Select("COALESCE(MAX(id),0)").Scan(&result.LastEventID).Error; err != nil {
		return err
	}
	if err := db.Model(&domain.Investigation{}).Count(&result.Total).Error; err != nil {
		return err
	}
	if err := db.Table("investigations AS i").Where("NOT "+readExpression, operator(c)).Count(&result.Unread).Error; err != nil {
		return err
	}
	var severities []struct {
		Severity string
		Count    int64
	}
	if err := db.Table("investigations AS i").Select("json_extract(i.result,'$.severity') AS severity, COUNT(*) AS count").Where("i.result IS NOT NULL AND i.result != 'null'").Group("json_extract(i.result,'$.severity')").Scan(&severities).Error; err != nil {
		return err
	}
	for _, row := range severities {
		result.BySeverity[row.Severity] = row.Count
	}
	var statuses []struct {
		Status string
		Count  int64
	}
	if err := investigationQuery(db).Select("COALESCE(execution.status, 'detected') AS status, COUNT(*) AS count").Group("COALESCE(execution.status, 'detected')").Scan(&statuses).Error; err != nil {
		return err
	}
	for _, row := range statuses {
		result.ByStatus[row.Status] = row.Count
	}
	return c.JSON(result)
}
func (h *Handler) followUp(c fiber.Ctx) error {
	s := h.service
	s.mu.Lock()
	defer s.mu.Unlock()
	detail, err := h.detail(c.Context(), operator(c), c.Params("id"))
	if err != nil {
		return err
	}
	if oneOf(detail.Status, "queued", "running") {
		return fiber.NewError(409, "Wait for the investigation to finish or cancel it before following up")
	}
	evidence := []domain.RequestLog{}
	if detail.Result == nil {
		for _, id := range detail.Trigger.RequestIDs {
			var item domain.RequestLog
			if err := s.db.WithContext(c.Context()).First(&item, "id = ?", id).Error; err == nil {
				evidence = append(evidence, boundedSnapshot(item))
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}
	}
	if len(detail.Events) > 50 {
		detail.Events = detail.Events[len(detail.Events)-50:]
	}
	for i := range detail.Events {
		detail.Events[i].Delta = bound(detail.Events[i].Delta, 256)
	}
	seed, err := json.Marshal(struct {
		Investigation   investigationDetail `json:"investigation"`
		InitialEvidence []domain.RequestLog `json:"initialEvidence"`
	}{detail, evidence})
	if err != nil {
		return err
	}
	if len(seed) > 192*1024 {
		return fiber.NewError(413, "Investigation context is too large")
	}
	user := operator(c)
	var link domain.InvestigationFollowUp
	linkErr := s.db.WithContext(c.Context()).First(&link, "run_id = ? AND user_id = ?", detail.ID, user).Error
	if errors.Is(linkErr, gorm.ErrRecordNotFound) {
		var latest domain.InvestigationRun
		if err := s.db.WithContext(c.Context()).Select("id").Where("incident_id = ?", detail.ID).Order("created_at DESC, id DESC").First(&latest).Error; err == nil {
			linkErr = s.db.WithContext(c.Context()).First(&link, "run_id = ? AND user_id = ?", latest.ID, user).Error
			if linkErr == nil {
				link.InvestigationID = detail.ID
				if err := s.db.WithContext(c.Context()).Clauses(clause.OnConflict{UpdateAll: true}).Create(&link).Error; err != nil {
					return err
				}
			}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
	}
	if linkErr == nil {
		var conversation domain.AssistantConversation
		err := s.db.WithContext(c.Context()).First(&conversation, "id = ? AND user_id = ?", link.ConversationID, user).Error
		if err == nil {
			if err := s.db.WithContext(c.Context()).Model(&conversation).Update("context", string(seed)).Error; err != nil {
				return err
			}
			return c.JSON(conversation)
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
	} else if !errors.Is(linkErr, gorm.ErrRecordNotFound) {
		return linkErr
	}
	conversation, err := s.assistant.CreateFollowUp(c.Context(), user, "Investigation follow-up", string(seed))
	if err != nil {
		return err
	}
	link = domain.InvestigationFollowUp{InvestigationID: detail.ID, UserID: user, ConversationID: conversation.ID}
	if err := s.db.WithContext(c.Context()).Clauses(clause.OnConflict{UpdateAll: true}).Create(&link).Error; err != nil {
		return err
	}
	return c.Status(201).JSON(conversation)
}

func eventType(kind string) string {
	switch kind {
	case "incident.created":
		return "investigation.created"
	case "incident.updated", "finding.updated":
		return "investigation.updated"
	case "finding.published":
		return "investigation.result_published"
	default:
		return kind
	}
}
func (h *Handler) stream(c fiber.Ctx, path string) error {
	investigationID := ""
	if strings.Contains(path, ":id") {
		var item domain.Investigation
		if err := h.service.db.WithContext(c.Context()).Select("id").First(&item, "id = ?", c.Params("id")).Error; err != nil {
			return notFound(err)
		}
		investigationID = item.ID
	}
	after := uint64(0)
	cursor := c.Get("Last-Event-ID")
	if cursor == "" {
		cursor = c.Query("after")
	}
	if cursor != "" {
		value, err := strconv.ParseUint(cursor, 10, 64)
		if err != nil {
			return fiber.ErrBadRequest
		}
		after = value
	}
	db := h.service.db
	if cursor == "" && investigationID == "" {
		if err := db.WithContext(c.Context()).Model(&domain.InvestigationEvent{}).Select("COALESCE(MAX(id),0)").Scan(&after).Error; err != nil {
			return err
		}
	}
	user := operator(c)
	sessionHash := sha256.Sum256([]byte(c.Cookies("session")))
	tokenHash := hex.EncodeToString(sessionHash[:])
	c.Set("Content-Type", "text/event-stream; charset=utf-8")
	c.Set("X-Accel-Buffering", "no")
	return c.SendStreamWriter(func(w *bufio.Writer) {
		tick := time.NewTicker(time.Second)
		defer tick.Stop()
		heartbeat := time.NewTicker(15 * time.Second)
		defer heartbeat.Stop()
		expiry := time.NewTimer(15 * time.Minute)
		defer expiry.Stop()
		for {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			var current domain.User
			err := db.WithContext(ctx).First(&current, user).Error
			var sessions int64
			if err == nil {
				err = db.WithContext(ctx).Model(&domain.UserSession{}).Where("user_id = ? AND token_hash = ? AND expires_at > ?", user, tokenHash, time.Now().UTC()).Count(&sessions).Error
			}
			if err != nil || current.Role != "admin" || sessions == 0 {
				cancel()
				return
			}
			q := db.WithContext(ctx).Where("id > ?", after)
			if investigationID != "" {
				q = q.Where("incident_id = ?", investigationID)
			}
			var events []domain.InvestigationEvent
			err = q.Order("id ASC").Limit(100).Find(&events).Error
			cancel()
			if err != nil {
				return
			}
			for _, e := range events {
				e.Type = eventType(e.Type)
				payload, err := json.Marshal(e)
				if err != nil {
					return
				}
				if _, err = fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", e.ID, e.Type, payload); err != nil {
					return
				}
				if w.Flush() != nil {
					return
				}
				after = e.ID
			}
			if len(events) == 100 {
				continue
			}
			select {
			case <-expiry.C:
				return
			case <-tick.C:
			case <-heartbeat.C:
				if _, err := w.WriteString(": heartbeat\n\n"); err != nil {
					return
				}
				if w.Flush() != nil {
					return
				}
			}
		}
	})
}
func pagination(c fiber.Ctx) (int, int, error) {
	page, limit := 1, 25
	var err error
	if v := c.Query("page"); v != "" {
		page, err = strconv.Atoi(v)
		if err != nil {
			return 0, 0, fiber.ErrBadRequest
		}
	}
	if v := c.Query("limit"); v != "" {
		limit, err = strconv.Atoi(v)
		if err != nil {
			return 0, 0, fiber.ErrBadRequest
		}
	}
	if page < 1 || page > 10000 || limit < 1 || limit > 100 {
		return 0, 0, domain.ValidationError("page must be 1–10000 and limit 1–100")
	}
	return page, limit, nil
}
func scope(c fiber.Ctx) (uint, error) {
	v := c.Query("serviceId")
	if v == "" {
		return 0, nil
	}
	n, err := strconv.ParseUint(v, 10, 32)
	if err != nil || n == 0 {
		return 0, domain.ValidationError("serviceId must be a positive integer")
	}
	return uint(n), nil
}

func (h *Handler) settings(c fiber.Ctx) error {
	id, err := scope(c)
	if err != nil {
		return err
	}
	if err := h.service.exists(c.Context(), id); err != nil {
		return err
	}
	p, err := h.service.policy(c.Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(settingsView{p, p.Scope != id, h.service.assistant.Enabled()})
}
func (s *Service) exists(ctx context.Context, id uint) error {
	if id == 0 {
		return nil
	}
	var item domain.Service
	return notFound(s.db.WithContext(ctx).First(&item, id).Error)
}
func (h *Handler) updateSettings(c fiber.Ctx) error {
	id, err := scope(c)
	if err != nil {
		return err
	}
	if err := h.service.exists(c.Context(), id); err != nil {
		return err
	}
	var p domain.AnomalySettings
	if err := api.ReadJSON(c, &p); err != nil {
		return err
	}
	p.Scope = id
	for _, n := range []int{p.BlockedThreshold, p.SuspiciousThreshold, p.ScanRequests, p.ScanPaths, p.ErrorThreshold, p.SurgeMinimum} {
		if n < 1 || n > 1000000 {
			return domain.ValidationError("thresholds must be between 1 and 1000000")
		}
	}
	if p.ScanPaths > 100 || p.ErrorPercent < 1 || p.ErrorPercent > 100 || p.SurgeMultiplier < 1.5 || p.SurgeMultiplier > 100 || p.CooldownMinutes < 1 || p.CooldownMinutes > 1440 || p.HourlyRunLimit < 1 || p.HourlyRunLimit > 1000 {
		return domain.ValidationError("invalid anomaly settings")
	}
	if err := h.service.db.WithContext(c.Context()).Clauses(clause.OnConflict{UpdateAll: true}).Create(&p).Error; err != nil {
		return err
	}
	return c.JSON(settingsView{p, false, h.service.assistant.Enabled()})
}
func (h *Handler) resetSettings(c fiber.Ctx) error {
	id, err := scope(c)
	if err != nil {
		return err
	}
	if id == 0 {
		return domain.ValidationError("global settings cannot be deleted")
	}
	if err := h.service.exists(c.Context(), id); err != nil {
		return err
	}
	if err := h.service.db.WithContext(c.Context()).Delete(&domain.AnomalySettings{}, "scope = ?", id).Error; err != nil {
		return err
	}
	return c.JSON(ack{true})
}
