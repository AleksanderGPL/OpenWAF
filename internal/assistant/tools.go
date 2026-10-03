package assistant

import (
	"context"
	"fmt"
	"net/netip"
	"strings"
	"time"

	"OpenWAF/internal/telemetry"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type query struct {
	From      string `json:"from,omitempty" jsonschema:"description=Inclusive RFC3339 timestamp; provide with to. Default last 24 hours."`
	To        string `json:"to,omitempty" jsonschema:"description=Exclusive RFC3339 timestamp; provide with from."`
	ServiceID uint64 `json:"serviceId,omitempty"`
	IP        string `json:"ip,omitempty"`
	Action    string `json:"action,omitempty" jsonschema:"enum=allowed,enum=blocked"`
	Method    string `json:"method,omitempty"`
	Status    int    `json:"status,omitempty"`
	RuleID    string `json:"ruleId,omitempty"`
	Search    string `json:"search,omitempty"`
	Page      int    `json:"page,omitempty"`
	Limit     int    `json:"limit,omitempty"`
}

func (q query) filter() (telemetry.Filter, error) {
	now := time.Now().UTC()
	f := telemetry.Filter{Window: telemetry.Window{From: now.Add(-24 * time.Hour), To: now}, ServiceID: q.ServiceID, IP: q.IP, Action: q.Action, Method: strings.ToUpper(q.Method), Status: q.Status, RuleID: q.RuleID, Search: q.Search, Page: q.Page, Limit: q.Limit}
	if q.From != "" || q.To != "" {
		var err error
		if f.From, err = time.Parse(time.RFC3339Nano, q.From); err != nil {
			return f, fmt.Errorf("from must be RFC3339")
		}
		if f.To, err = time.Parse(time.RFC3339Nano, q.To); err != nil {
			return f, fmt.Errorf("to must be RFC3339")
		}
	}
	if !f.From.Before(f.To) || f.To.Sub(f.From) > 30*24*time.Hour {
		return f, fmt.Errorf("query window must be positive and at most 30 days")
	}
	if f.Page == 0 {
		f.Page = 1
	}
	if f.Limit == 0 {
		f.Limit = 25
	}
	if f.Page < 1 || f.Page > 1000 || f.Limit < 1 || f.Limit > 100 {
		return f, fmt.Errorf("page must be 1–1000 and limit 1–100")
	}
	if f.Action != "" && f.Action != "allowed" && f.Action != "blocked" {
		return f, fmt.Errorf("invalid action")
	}
	if f.Status != 0 && (f.Status < 100 || f.Status > 599) {
		return f, fmt.Errorf("invalid status")
	}
	if len(f.Search) > 256 || len(f.RuleID) > 128 || len(f.Method) > 32 {
		return f, fmt.Errorf("filter too long")
	}
	if f.IP != "" {
		ip, err := netip.ParseAddr(f.IP)
		if err != nil || ip.Zone() != "" {
			return f, fmt.Errorf("invalid IP")
		}
		f.IP = ip.Unmap().String()
	}
	return f, nil
}

func queryTool[T any](name, description string, fn func(context.Context, telemetry.Filter) (T, error)) (tool.BaseTool, error) {
	return utils.InferTool(name, description, func(ctx context.Context, q query) (T, error) {
		var zero T
		f, err := q.filter()
		if err != nil {
			return zero, err
		}
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		return fn(ctx, f)
	})
}

func telemetryTools(s *telemetry.Service) ([]tool.BaseTool, error) {
	var result []tool.BaseTool
	add := func(t tool.BaseTool, err error) error {
		if err != nil {
			return err
		}
		result = append(result, t)
		return nil
	}
	if err := add(queryTool("get_request_stats", "Get retained request metrics and the previous equal time window. Includes collection failures and retention coverage.", s.Summary)); err != nil {
		return nil, err
	}
	if err := add(queryTool("get_traffic", "Get hourly or daily request metrics to compare traffic over time.", s.Traffic)); err != nil {
		return nil, err
	}
	if err := add(queryTool("get_threats", "Get blocked requests grouped by WAF rule.", s.Threats)); err != nil {
		return nil, err
	}
	if err := add(queryTool("get_blocked_sources", "Get the ten most blocked source IPs.", s.BlockedSources)); err != nil {
		return nil, err
	}
	if err := add(queryTool("search_requests", "Search paginated request logs. Search matches path, hostname, IP, reason or exact request ID. Cite returned IDs as evidence.", s.Logs)); err != nil {
		return nil, err
	}
	type requestID struct {
		ID uint64 `json:"id" jsonschema:"required,description=Request log ID"`
	}
	if err := add(utils.InferTool("get_request", "Get one request log by ID.", func(ctx context.Context, q requestID) (any, error) {
		if q.ID == 0 {
			return nil, fmt.Errorf("id must be positive")
		}
		return s.RequestLog(ctx, q.ID)
	})); err != nil {
		return nil, err
	}
	return result, nil
}
