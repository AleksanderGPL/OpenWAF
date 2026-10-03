package assistant

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"OpenWAF/internal/api"
	"OpenWAF/internal/domain"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type Handler struct{ service *Service }

func NewHandler(s *Service) *Handler { return &Handler{service: s} }

type createRequest struct {
	Title string `json:"title" maxLength:"120"`
}
type messageRequest struct {
	Content string `json:"content" required:"true" minLength:"1" maxLength:"8000"`
}
type conversationParams struct {
	ID string `path:"id" required:"true" format:"uuid"`
}
type conversationList struct {
	Items []domain.AssistantConversation `json:"items" required:"true"`
}
type messageList struct {
	Items []domain.AssistantMessage `json:"items" required:"true"`
}
type statusResponse struct {
	Enabled bool   `json:"enabled" required:"true"`
	Model   string `json:"model" required:"true"`
}
type acknowledgement struct {
	Success bool `json:"success" required:"true"`
}

func (h *Handler) Register(router *api.Router, requireAuth fiber.Handler) {
	routes := router.Group("/assistant", requireAuth, func(c fiber.Ctx) error {
		user, ok := c.Locals("authUser").(domain.User)
		if !ok || user.Role != "admin" {
			return fiber.ErrForbidden
		}
		c.Set("Cache-Control", "no-store")
		return c.Next()
	})
	routes.Handle(http.MethodGet, "/status", api.Operation{ID: "getAssistantStatus", Summary: "Get assistant configuration status", Response: statusResponse{}, Session: true, Errors: []int{401, 403}}, h.status)
	routes.Handle(http.MethodPost, "/conversations", api.Operation{ID: "createAssistantConversation", Summary: "Create an assistant conversation", Request: createRequest{}, Response: domain.AssistantConversation{}, Status: 201, Session: true, Errors: []int{400, 401, 403, 415}}, h.create)
	routes.Handle(http.MethodGet, "/conversations", api.Operation{ID: "listAssistantConversations", Summary: "List your 100 most recently updated conversations", Response: conversationList{}, Session: true, Errors: []int{401, 403}}, h.list)
	routes.Handle(http.MethodGet, "/conversations/:id/messages", api.Operation{ID: "listAssistantMessages", Summary: "Get conversation messages", Parameters: conversationParams{}, Response: messageList{}, Session: true, Errors: []int{401, 403, 404}}, h.messages)
	routes.Handle(http.MethodPost, "/conversations/:id/messages", api.Operation{ID: "sendAssistantMessage", Summary: "Send a message and stream the investigation", Description: "Admin session required. JSON content input; response is SSE with JSON Event payloads. Events: run_started, text_delta, tool_started, tool_finished, run_completed, run_failed. Heartbeat comments every 15 seconds. Consume with streaming fetch, not EventSource. One active run per conversation; four globally. Runs time out after three minutes. Disconnect cancels the run. The runId is the persisted assistant message ID. Failed or cancelled turns are excluded from subsequent model context. Streams cannot be replayed; fetch messages after a disconnect.", Parameters: conversationParams{}, Request: messageRequest{}, Response: Event{}, ResponseContentType: "text/event-stream", Session: true, Errors: []int{400, 401, 403, 404, 409, 415, 429, 503}}, h.send)
	routes.Handle(http.MethodPost, "/conversations/:id/cancel", api.Operation{ID: "cancelAssistantRun", Summary: "Cancel the conversation's active run", Parameters: conversationParams{}, Response: acknowledgement{}, Session: true, Errors: []int{401, 403, 404}}, h.cancel)
	routes.Handle(http.MethodDelete, "/conversations/:id", api.Operation{ID: "deleteAssistantConversation", Summary: "Delete a conversation and its messages", Parameters: conversationParams{}, Response: acknowledgement{}, Session: true, Errors: []int{401, 403, 404, 409}}, h.delete)
}
func userID(c fiber.Ctx) uint { return c.Locals("authUser").(domain.User).ID }
func (h *Handler) status(c fiber.Ctx) error {
	return c.JSON(statusResponse{Enabled: h.service.runner != nil, Model: h.service.model})
}
func (h *Handler) create(c fiber.Ctx) error {
	var input createRequest
	if err := api.ReadJSON(c, &input); err != nil {
		return err
	}
	input.Title = strings.TrimSpace(input.Title)
	if len(input.Title) > 120 {
		return domain.ValidationError("title must be at most 120 bytes")
	}
	if input.Title == "" {
		input.Title = "Traffic investigation"
	}
	now := time.Now().UTC()
	conversation := domain.AssistantConversation{ID: uuid.NewString(), UserID: userID(c), Title: input.Title, CreatedAt: now, UpdatedAt: now}
	if err := h.service.db.WithContext(c.Context()).Create(&conversation).Error; err != nil {
		return err
	}
	return c.Status(201).JSON(conversation)
}
func (h *Handler) list(c fiber.Ctx) error {
	items := make([]domain.AssistantConversation, 0)
	if err := h.service.db.WithContext(c.Context()).Where("user_id = ?", userID(c)).Order("updated_at DESC, id ASC").Limit(100).Find(&items).Error; err != nil {
		return err
	}
	return c.JSON(conversationList{Items: items})
}
func (h *Handler) messages(c fiber.Ctx) error {
	conversation, err := h.service.conversation(c.Context(), userID(c), c.Params("id"))
	if err != nil {
		return err
	}
	items, err := h.service.messages(c.Context(), conversation.ID)
	if err != nil {
		return err
	}
	return c.JSON(messageList{Items: items})
}
func (h *Handler) cancel(c fiber.Ctx) error {
	conversation, err := h.service.conversation(c.Context(), userID(c), c.Params("id"))
	if err != nil {
		return err
	}
	h.service.mu.Lock()
	if cancel, ok := h.service.active[conversation.ID]; ok {
		cancel()
	}
	h.service.mu.Unlock()
	return c.JSON(acknowledgement{Success: true})
}
func (h *Handler) delete(c fiber.Ctx) error {
	s := h.service
	s.mu.Lock()
	defer s.mu.Unlock()
	conversation, err := s.conversation(c.Context(), userID(c), c.Params("id"))
	if err != nil {
		return err
	}
	if _, ok := s.active[conversation.ID]; ok {
		return fiber.NewError(409, "Cancel the active run before deleting the conversation")
	}
	if err := s.db.WithContext(c.Context()).Delete(&conversation).Error; err != nil {
		return err
	}
	return c.JSON(acknowledgement{Success: true})
}
func (h *Handler) send(c fiber.Ctx) error {
	var input messageRequest
	if err := api.ReadJSON(c, &input); err != nil {
		return err
	}
	t, err := h.service.begin(userID(c), strings.Clone(c.Params("id")), input.Content)
	if err != nil {
		return err
	}
	c.Set("Content-Type", "text/event-stream; charset=utf-8")
	c.Set("X-Accel-Buffering", "no")
	// Fiber recycles its Ctx after the handler returns. The writer captures only owned data.
	return c.SendStreamWriter(func(w *bufio.Writer) {
		defer t.cancel()
		streamCtx, disconnect := context.WithCancel(context.Background())
		defer disconnect()
		events := make(chan Event, 16)
		go func() {
			defer close(events)
			h.service.Execute(t, func(e Event) error {
				select {
				case events <- e:
					return nil
				case <-streamCtx.Done():
					return streamCtx.Err()
				}
			})
		}()
		heartbeat := time.NewTicker(15 * time.Second)
		defer heartbeat.Stop()
		for {
			select {
			case event, ok := <-events:
				if !ok {
					return
				}
				payload, err := json.Marshal(event)
				if err != nil {
					return
				}
				if _, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, payload); err != nil {
					return
				}
				if err = w.Flush(); err != nil {
					return
				}
			case <-heartbeat.C:
				if _, err := w.WriteString(": heartbeat\n\n"); err != nil {
					return
				}
				if err := w.Flush(); err != nil {
					return
				}
			}
		}
	})
}
