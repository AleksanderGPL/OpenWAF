package rules

import (
	"io"
	"net/http"
	"net/netip"
	"sort"
	"strconv"
	"strings"
	"time"

	"OpenWAF/internal/domain"
	"github.com/corazawaf/coraza/v3/experimental"
	"github.com/corazawaf/coraza/v3/types"
)

type counterKey struct {
	service uint
	ip      string
}
type counter struct {
	started           time.Time
	requests, blocked int
}

func (s *Service) traffic(service uint, ip string, blocked bool) (int, int) {
	s.countersMu.Lock()
	defer s.countersMu.Unlock()
	now := time.Now()
	key := counterKey{service, ip}
	c := s.counters[key]
	if c == nil {
		if len(s.counters) >= 100000 {
			if now.Sub(s.lastSweep) >= time.Second {
				s.lastSweep = now
				for k, v := range s.counters {
					if now.Sub(v.started) >= time.Minute {
						delete(s.counters, k)
					}
				}
			}
			if len(s.counters) >= 100000 {
				return -1, 0
			}
		}
		c = &counter{started: now}
		s.counters[key] = c
	}
	if now.Sub(c.started) >= time.Minute {
		c.started = now
		c.requests = 0
		c.blocked = 0
	}
	if blocked {
		c.blocked++
	} else {
		c.requests++
	}
	return c.requests, c.blocked
}

func (c compiledRule) matches(r *http.Request, ip string) bool {
	query := r.URL.Query()
	for i, cond := range c.rule.Conditions {
		var values []string
		switch cond.Target {
		case "path":
			values = []string{r.URL.Path}
		case "method":
			values = []string{r.Method}
		case "ip":
			values = []string{ip}
		case "query":
			values = query[cond.Key]
		case "header":
			if strings.EqualFold(cond.Key, "Host") {
				values = []string{r.Host}
			} else {
				values = r.Header.Values(cond.Key)
			}
		}
		matched := false
		for _, v := range values {
			switch cond.Operator {
			case "equals":
				matched = v == cond.Value
			case "contains":
				matched = strings.Contains(v, cond.Value)
			case "prefix":
				matched = strings.HasPrefix(v, cond.Value)
			case "suffix":
				matched = strings.HasSuffix(v, cond.Value)
			case "regex":
				matched = c.regex[i].MatchString(v)
			case "cidr":
				addr, err := netip.ParseAddr(v)
				matched = err == nil && c.networks[i].Contains(addr.Unmap())
			}
			if matched {
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

func (s *Service) Inspect(r *http.Request, service uint, ip string, event *domain.RequestLog) (status int, cleanup func(), err error) {
	cleanup = func() {}
	s.mu.RLock()
	state := s.state
	p := effectivePolicy(state, service)
	engine, ok := state.engines[service]
	if !ok {
		engine = state.engines[0]
	}
	custom := state.rules
	s.mu.RUnlock()
	if p.Mode == "off" {
		return
	}
	event.RuleMatches = []domain.RuleMatch{}
	event.SecuritySignals = []string{}
	blocked := false
	defer func() {
		if status != 0 {
			blocked = true
			event.Action = "blocked"
		}
		if blocked {
			_, n := s.traffic(service, ip, true)
			if n >= 10 {
				event.SecuritySignals = append(event.SecuritySignals, "repeated_blocks")
			}
		}
	}()
	count, _ := s.traffic(service, ip, false)
	if count < 0 {
		event.SecuritySignals = append(event.SecuritySignals, "rate_tracking_capacity")
		if p.RateLimitPerMinute > 0 && p.RateLimitAction == "block" && p.Mode == "blocking" {
			event.RuleID = "rate_capacity"
			event.Reason = "Rate limiter capacity reached"
			status = 503
			return
		}
	}
	if p.RateLimitPerMinute > 0 && count > p.RateLimitPerMinute {
		event.RuleMatches = append(event.RuleMatches, domain.RuleMatch{RuleID: "rate_limit", Source: "behavior", Message: "Per-service IP request rate exceeded"})
		event.SecuritySignals = append(event.SecuritySignals, "high_request_rate")
		if p.RateLimitAction == "block" && p.Mode == "blocking" {
			event.RuleID = "rate_limit"
			event.Reason = "Per-service IP request rate exceeded"
			status = 429
			return
		}
	}
	for _, c := range custom {
		if !c.rule.Enabled || c.rule.ServiceID != nil && *c.rule.ServiceID != service || !c.matches(r, ip) {
			continue
		}
		id := "custom:" + strconv.FormatUint(uint64(c.rule.ID), 10)
		event.RuleMatches = append(event.RuleMatches, domain.RuleMatch{RuleID: id, Source: "custom", Message: c.rule.Name})
		if c.rule.Action == "block" && p.Mode == "blocking" && status == 0 {
			status = 403
			event.RuleID = id
			event.Reason = c.rule.Name
		}
	}
	if status != 0 {
		return
	}
	var tx types.Transaction
	if contextual, ok := engine.(experimental.WAFWithOptions); ok {
		tx = contextual.NewTransactionWithOptions(experimental.Options{Context: r.Context()})
	} else {
		tx = engine.NewTransaction()
	}
	cleanup = func() { tx.ProcessLogging(); _ = tx.Close() }
	defer s.appendMatches(tx, event)
	tx.ProcessConnection(ip, 0, "", 0)
	tx.ProcessURI(r.URL.RequestURI(), r.Method, r.Proto)
	tx.SetServerName(r.Host)
	for k, values := range r.Header {
		for _, v := range values {
			tx.AddRequestHeader(k, v)
		}
	}
	tx.AddRequestHeader("Host", r.Host)
	for _, v := range r.TransferEncoding {
		tx.AddRequestHeader("Transfer-Encoding", v)
	}
	if r.ContentLength >= 0 && r.Header.Get("Content-Length") == "" && (r.Body != nil && r.Body != http.NoBody) {
		tx.AddRequestHeader("Content-Length", strconv.FormatInt(r.ContentLength, 10))
	}
	interruption := tx.ProcessRequestHeaders()
	if interruption == nil {
		if r.ContentLength > p.MaxBodyBytes {
			status = 413
			event.RuleID = "body_limit"
			event.Reason = "Request body limit exceeded"
			return
		}
		if r.Body != nil && r.Body != http.NoBody && tx.IsRequestBodyAccessible() {
			original := r.Body
			var n int
			interruption, n, err = tx.ReadRequestBodyFrom(io.LimitReader(original, p.MaxBodyBytes+1))
			if err != nil {
				status = 400
				event.ErrorCategory = "waf_request_body"
				return
			}
			if int64(n) >= p.MaxBodyBytes {
				status = 413
				event.RuleID = "body_limit"
				event.Reason = "Request body limit exceeded"
				return
			}
			if interruption == nil {
				var buffered io.Reader
				buffered, err = tx.RequestBodyReader()
				if err != nil {
					status = 500
					event.ErrorCategory = "waf_processing"
					return
				}
				r.Body = &replayedBody{Reader: io.MultiReader(buffered, original), original: original}
			}
		}
		if interruption == nil {
			interruption, err = tx.ProcessRequestBody()
			if err != nil {
				status = 400
				event.ErrorCategory = "waf_request_body"
				return
			}
		}
	}
	if interruption != nil {
		status = interruption.Status
		if status < 400 || status > 599 {
			status = 403
		}
		event.RuleID = strconv.Itoa(interruption.RuleID)
		event.Reason = s.matchedMessage(tx, interruption.RuleID)
		if interruption.Status == 413 {
			event.RuleID = "body_limit"
			event.Reason = "Request body limit exceeded"
		}
	}
	if (len(event.RuleMatches) > 0 || len(tx.MatchedRules()) > 0) && status == 0 {
		event.Reason = "Allowed with security observations"
	}
	return
}

type replayedBody struct {
	io.Reader
	original io.Closer
}

func (r *replayedBody) Close() error { return r.original.Close() }

func (s *Service) appendMatches(tx types.Transaction, event *domain.RequestLog) {
	for _, m := range tx.MatchedRules() {
		rule := m.Rule()
		id := rule.ID()
		source := "crs"
		if id < 900000 {
			source = "coraza"
		} else if id >= 1001001 && id <= 1001003 {
			source = "patch"
		}
		item := domain.RuleMatch{RuleID: strconv.Itoa(id), Source: source, Message: expandedMatchMessage(m, s.message(id)), Severity: rule.Severity().String(), Tags: rule.Tags(), Variables: []string{}}
		seen := map[string]bool{}
		for _, d := range m.MatchedDatas() {
			name := d.Variable().Name()
			if !seen[name] {
				item.Variables = append(item.Variables, name)
				seen[name] = true
			}
		}
		event.RuleMatches = append(event.RuleMatches, item)
		if len(event.RuleMatches) >= 100 {
			break
		}
	}
}

func (s *Service) message(id int) string {
	for _, r := range s.Catalog {
		if r.ID == id && r.Message != "" {
			return r.Message
		}
	}
	return "WAF rule matched"
}

func (s *Service) matchedMessage(tx types.Transaction, id int) string {
	fallback := s.message(id)
	for _, m := range tx.MatchedRules() {
		if m.Rule().ID() != id {
			continue
		}
		if msg := expandedMatchMessage(m, fallback); msg != "" {
			return msg
		}
	}
	return fallback
}

func expandedMatchMessage(m types.MatchedRule, fallback string) string {
	msg := strings.TrimSpace(m.Message())
	if msg == "" {
		msg = fallback
	}
	values := map[string]string{}
	for _, data := range m.MatchedDatas() {
		value := strings.TrimSpace(data.Value())
		if value == "" {
			continue
		}
		name := strings.ToLower(data.Variable().Name())
		if key := strings.ToLower(data.Key()); key != "" {
			values[name+"."+key] = value
			continue
		}
		values[name] = value
	}
	if !strings.Contains(msg, "%{") && !strings.Contains(strings.ToUpper(msg), "TX.") {
		return msg
	}
	msg = messageMacro.ReplaceAllStringFunc(msg, func(token string) string {
		if value, ok := values[strings.ToLower(messageMacro.FindStringSubmatch(token)[1])]; ok {
			return value
		}
		name := strings.ToLower(messageMacro.FindStringSubmatch(token)[1])
		if value, ok := values["tx."+name]; ok {
			return value
		}
		return token
	})
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return len(keys[i]) > len(keys[j]) })
	for _, key := range keys {
		msg = strings.ReplaceAll(msg, strings.ToUpper(key), values[key])
		msg = strings.ReplaceAll(msg, key, values[key])
	}
	if strings.Contains(msg, "%{") {
		return displayMessage(msg)
	}
	return strings.TrimSpace(msg)
}
