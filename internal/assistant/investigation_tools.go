package assistant

import (
	"context"
	"time"

	"OpenWAF/internal/domain"
	"OpenWAF/internal/telemetry"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"gorm.io/gorm"
)

type securityMatch struct {
	RuleID   string `json:"ruleId"`
	Source   string `json:"source"`
	Requests int64  `json:"requests"`
}
type securityOverview struct {
	telemetry.Window
	Items []securityMatch `json:"items"`
	Limit int             `json:"limit"`
}
type minuteCount struct {
	Minute   string `json:"minute"`
	Requests int64  `json:"requests"`
	Blocked  int64  `json:"blocked"`
	Errors   int64  `json:"errors"`
}
type minuteOverview struct {
	telemetry.Window
	Items []minuteCount `json:"items"`
}

func (s *Service) investigationTools() ([]tool.BaseTool, error) {
	matches, err := utils.InferTool("get_security_matches", "Count security rule matches including requests allowed in detection mode. Top 100 rules across the selected window.", func(ctx context.Context, q query) (securityOverview, error) {
		f, err := q.filter()
		if err != nil {
			return securityOverview{}, err
		}
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		result := securityOverview{Window: f.Window, Items: []securityMatch{}, Limit: 100}
		ruleID := f.RuleID
		f.RuleID = ""
		db := investigationQuery(s.db.WithContext(ctx), f)
		if ruleID != "" {
			db = db.Where("json_extract(match.value, '$.ruleId') = ?", ruleID)
		}
		err = db.Select("json_extract(match.value, '$.ruleId') AS rule_id, json_extract(match.value, '$.source') AS source, COUNT(DISTINCT request_logs.id) AS requests").Joins("JOIN json_each(request_logs.rule_matches) AS match").Group("json_extract(match.value, '$.ruleId'), json_extract(match.value, '$.source')").Order("requests DESC, json_extract(match.value, '$.ruleId') ASC").Limit(100).Scan(&result.Items).Error
		return result, err
	})
	if err != nil {
		return nil, err
	}
	timeline, err := utils.InferTool("get_minute_traffic", "Get minute-level request counts, blocks and upstream errors. Query at most two hours per call.", func(ctx context.Context, q query) (minuteOverview, error) {
		f, err := q.filter()
		if err != nil {
			return minuteOverview{}, err
		}
		if f.To.Sub(f.From) > 2*time.Hour {
			return minuteOverview{}, domain.ValidationError("minute traffic window must be at most two hours")
		}
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		result := minuteOverview{Window: f.Window, Items: []minuteCount{}}
		err = investigationQuery(s.db.WithContext(ctx), f).Select("strftime('%Y-%m-%dT%H:%M:00Z', timestamp) AS minute, COUNT(*) AS requests, SUM(action = 'blocked') AS blocked, SUM(action = 'allowed' AND (status >= 500 OR error_category != '')) AS errors").Group("minute").Order("minute ASC").Scan(&result.Items).Error
		return result, err
	})
	if err != nil {
		return nil, err
	}
	return []tool.BaseTool{matches, timeline}, nil
}
func investigationQuery(db *gorm.DB, f telemetry.Filter) *gorm.DB {
	db = db.Table("request_logs").Where("request_logs.timestamp >= ? AND request_logs.timestamp < ?", f.From, f.To)
	if f.ServiceID != 0 {
		db = db.Where("request_logs.service_id = ?", f.ServiceID)
	}
	if f.IP != "" {
		db = db.Where("request_logs.ip = ?", f.IP)
	}
	if f.Action != "" {
		db = db.Where("request_logs.action = ?", f.Action)
	}
	if f.Method != "" {
		db = db.Where("request_logs.method = ?", f.Method)
	}
	if f.Status != 0 {
		db = db.Where("request_logs.status = ?", f.Status)
	}
	if f.RuleID != "" {
		db = db.Where("request_logs.rule_id = ?", f.RuleID)
	}
	if f.Search != "" {
		db = db.Where("instr(lower(request_logs.path),lower(?)) > 0", f.Search)
	}
	return db
}
