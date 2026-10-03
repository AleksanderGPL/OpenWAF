package assistant

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"OpenWAF/internal/domain"
	"OpenWAF/internal/telemetry"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/compose"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Report struct {
	Title           string   `json:"title" jsonschema:"required,description=Concise operator-facing title"`
	Summary         string   `json:"summary" jsonschema:"required"`
	Severity        string   `json:"severity" jsonschema:"required,enum=info,enum=low,enum=medium,enum=high,enum=critical"`
	Assessment      string   `json:"assessment" jsonschema:"required,enum=likely_malicious,enum=likely_benign,enum=inconclusive"`
	Explanation     string   `json:"explanation" jsonschema:"required,description=Evidence-backed explanation distinguishing observations from hypotheses"`
	Patterns        []string `json:"patterns" jsonschema:"required,description=Possible attack patterns with uncertainty stated; empty if none"`
	Recommendations []string `json:"recommendations" jsonschema:"required"`
	Limitations     []string `json:"limitations" jsonschema:"required"`
	RequestIDs      []uint64 `json:"requestIds" jsonschema:"required,description=At most 20 request IDs actually returned by telemetry tools; empty only for an inconclusive assessment when evidence is unavailable"`
}

func (s *Service) Enabled() bool { return s.runner != nil }
func (s *Service) CanInvestigate() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.runner != nil && !s.closing && len(s.active) < 3
}

func (s *Service) Investigate(ctx context.Context, telemetryService *telemetry.Service, brief string, validate func(Report) error, send func(Event) error) (Report, string, error) {
	var report Report
	if !s.Enabled() {
		return report, "", fmt.Errorf("AI is not configured")
	}
	tools, err := telemetryTools(telemetryService)
	if err != nil {
		return report, "", err
	}
	extra, err := s.investigationTools()
	if err != nil {
		return report, "", err
	}
	tools = append(tools, extra...)
	submitted := false
	var reportMu sync.Mutex
	publication, err := utils.InferTool("submit_finding", "Submit the final structured finding after gathering evidence. Call exactly once. This does not change traffic or rules.", func(ctx context.Context, input Report) (string, error) {
		reportMu.Lock()
		defer reportMu.Unlock()
		if submitted {
			return "", fmt.Errorf("finding already submitted")
		}
		if err := validate(input); err != nil {
			return "", err
		}
		if input.Patterns == nil {
			input.Patterns = []string{}
		}
		if input.Recommendations == nil {
			input.Recommendations = []string{}
		}
		if input.Limitations == nil {
			input.Limitations = []string{}
		}
		if input.RequestIDs == nil {
			input.RequestIDs = []uint64{}
		}
		report = input
		submitted = true
		return "Finding recorded. Finish with a concise operator-facing summary.", nil
	})
	if err != nil {
		return report, "", err
	}
	tools = append(tools, publication)
	var budget atomic.Int32
	parallel := make(chan struct{}, 3)
	for i, t := range tools {
		invoke, ok := t.(tool.InvokableTool)
		if !ok {
			return report, "", fmt.Errorf("investigation tool does not support invocation")
		}
		tools[i] = &boundedTool{InvokableTool: invoke, budget: &budget, parallel: parallel}
	}
	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{Name: "openwaf_investigator", Description: "Investigate detected traffic anomalies", Instruction: instruction + " Investigate the supplied investigation autonomously. Explain briefly what you are checking. A trigger is evidence of unusual activity, not proof of an attack. Use explicit time windows derived from the trigger for all aggregate queries. Fetch initial evidence IDs using get_request or search_requests. Gather evidence using tools, then call submit_finding exactly once. Never invent request IDs. If no request evidence remains available, submit an inconclusive finding with an empty requestIds list and explain the data limitations. Do not claim an attack succeeded based on HTTP status alone.", Model: s.chatModel, MaxIterations: 8, ToolsConfig: adk.ToolsConfig{ToolsNodeConfig: compose.ToolsNodeConfig{Tools: tools}}})
	if err != nil {
		return report, "", err
	}
	now := time.Now().UTC()
	c := domain.AssistantConversation{ID: uuid.NewString(), UserID: 0, Title: "Automatic investigation", CreatedAt: now, UpdatedAt: now}
	if err := s.db.WithContext(ctx).Create(&c).Error; err != nil {
		return report, "", err
	}
	defer s.db.WithContext(context.Background()).Delete(&c)
	t, err := s.begin(0, c.ID, brief)
	if err != nil {
		return report, "", err
	}
	t.runner = adk.NewRunner(ctx, adk.RunnerConfig{Agent: agent, EnableStreaming: true})
	stop := context.AfterFunc(ctx, t.cancel)
	defer stop()
	s.Execute(t, send)
	if t.message.Status != "completed" {
		return report, "", fmt.Errorf("investigation did not complete")
	}
	if !submitted {
		return report, "", fmt.Errorf("investigation did not submit a finding")
	}
	return report, t.message.History, nil
}

func (s *Service) CreateFollowUp(ctx context.Context, user uint, title, contextText string) (domain.AssistantConversation, error) {
	now := time.Now().UTC()
	c := domain.AssistantConversation{ID: uuid.NewString(), UserID: user, Title: title, Context: contextText, CreatedAt: now, UpdatedAt: now}
	seed, err := json.Marshal([]map[string]string{{"role": "user", "content": "Continue investigating the recorded investigation supplied in the conversation context."}, {"role": "assistant", "content": "I can help examine the evidence and investigate further using telemetry tools."}})
	if err != nil {
		return c, err
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&c).Error; err != nil {
			return err
		}
		return tx.Create(&domain.AssistantMessage{ID: uuid.NewString(), ConversationID: c.ID, Role: "assistant", Content: "This conversation is linked to the recorded investigation. Ask a follow-up question to investigate further.", Status: "completed", History: string(seed), CreatedAt: now}).Error
	})
	return c, err
}

type boundedTool struct {
	tool.InvokableTool
	budget   *atomic.Int32
	parallel chan struct{}
}

func (t *boundedTool) InvokableRun(ctx context.Context, input string, opts ...tool.Option) (string, error) {
	if t.budget.Add(1) > 24 {
		return "", fmt.Errorf("investigation tool-call limit reached")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	select {
	case t.parallel <- struct{}{}:
		defer func() { <-t.parallel }()
	case <-ctx.Done():
		return "", ctx.Err()
	}
	output, err := t.InvokableTool.InvokableRun(ctx, input, opts...)
	if err == nil || ctx.Err() != nil {
		return output, err
	}
	message := err.Error()
	if len(message) > 1024 {
		message = message[:1024]
	}
	result, marshalErr := json.Marshal(map[string]string{"error": message, "action": "Correct the tool arguments or use another evidence query."})
	if marshalErr != nil {
		return "", marshalErr
	}
	return string(result), nil
}
