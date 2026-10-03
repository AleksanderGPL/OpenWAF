package rules

import (
	"context"
	"errors"
	"fmt"
	"github.com/corazawaf/coraza/v3/experimental"
	"net/netip"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"OpenWAF/internal/domain"
	"github.com/corazawaf/coraza/v3"
	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type RuleInput struct {
	ServiceID   *uint                  `json:"serviceId"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Enabled     *bool                  `json:"enabled" default:"true"`
	Action      string                 `json:"action" enum:"block,log"`
	Conditions  []domain.RuleCondition `json:"conditions"`
}

type PolicyInput struct {
	Mode                   string `json:"mode" enum:"blocking,detection,off"`
	BlockingParanoiaLevel  int    `json:"blockingParanoiaLevel" minimum:"1" maximum:"4"`
	DetectionParanoiaLevel int    `json:"detectionParanoiaLevel" minimum:"1" maximum:"4"`
	InboundThreshold       int    `json:"inboundThreshold" minimum:"1" maximum:"100"`
	MaxBodyBytes           int64  `json:"maxBodyBytes" minimum:"1024" maximum:"134217728"`
	RateLimitPerMinute     int    `json:"rateLimitPerMinute" minimum:"0" maximum:"1000000"`
	RateLimitAction        string `json:"rateLimitAction" enum:"block,log"`
	DisabledRuleIDs        []int  `json:"disabledRuleIds"`
}

type compiledRule struct {
	rule     domain.Rule
	regex    []*regexp.Regexp
	networks []netip.Prefix
}
type snapshot struct {
	policies map[uint]domain.RulePolicy
	engines  map[uint]coraza.WAF
	rules    []compiledRule
}
type Service struct {
	db         *gorm.DB
	mu         sync.RWMutex
	writeMu    sync.Mutex
	state      snapshot
	Catalog    []CatalogRule
	ruleIDs    map[int]bool
	countersMu sync.Mutex
	counters   map[counterKey]*counter
	lastSweep  time.Time
}

func New(ctx context.Context, db *gorm.DB) (*Service, error) {
	items, err := catalog()
	if err != nil {
		return nil, err
	}
	s := &Service{db: db, Catalog: items, ruleIDs: map[int]bool{}, counters: map[counterKey]*counter{}}
	for _, r := range items {
		s.ruleIDs[r.ID] = true
	}
	p := defaultPolicy()
	if err := db.WithContext(ctx).Where("scope = ?", 0).Attrs(p).FirstOrCreate(&domain.RulePolicy{}).Error; err != nil {
		return nil, err
	}
	state, err := s.load(ctx)
	if err != nil {
		closeNewEngines(state, snapshot{})
		return nil, err
	}
	s.state = state
	return s, nil
}

func (s *Service) load(ctx context.Context, previous ...snapshot) (snapshot, error) {
	var policies []domain.RulePolicy
	var rules []domain.Rule
	if err := s.db.WithContext(ctx).Order("scope").Find(&policies).Error; err != nil {
		return snapshot{}, err
	}
	if err := s.db.WithContext(ctx).Order("id").Find(&rules).Error; err != nil {
		return snapshot{}, err
	}
	state := snapshot{policies: map[uint]domain.RulePolicy{}, engines: map[uint]coraza.WAF{}}
	for _, p := range policies {
		state.policies[p.Scope] = p
	}
	if _, ok := state.policies[0]; !ok {
		return state, errors.New("global rule policy missing")
	}
	for _, p := range policies {
		if err := s.validatePolicy(p); err != nil {
			return state, err
		}
		effective := effectivePolicy(state, p.Scope)
		var engine coraza.WAF
		if len(previous) > 0 {
			old := previous[0]
			if _, exists := old.policies[p.Scope]; exists && engineKey(effectivePolicy(old, p.Scope)) == engineKey(effective) {
				engine = old.engines[p.Scope]
			}
		}
		var err error
		if engine == nil {
			engine, err = compile(effective)
		}
		if err != nil {
			return state, fmt.Errorf("compile WAF policy %d: %w", p.Scope, err)
		}
		state.engines[p.Scope] = engine
	}
	for _, r := range rules {
		c, err := validateRule(r)
		if err != nil {
			return state, err
		}
		state.rules = append(state.rules, c)
	}
	return state, nil
}

func effectivePolicy(state snapshot, scope uint) domain.RulePolicy {
	p, ok := state.policies[scope]
	if !ok {
		p = state.policies[0]
	}
	if scope != 0 {
		ids := append([]int{}, state.policies[0].DisabledRuleIDs...)
		ids = append(ids, p.DisabledRuleIDs...)
		sort.Ints(ids)
		p.DisabledRuleIDs = []int{}
		for _, id := range ids {
			if len(p.DisabledRuleIDs) == 0 || p.DisabledRuleIDs[len(p.DisabledRuleIDs)-1] != id {
				p.DisabledRuleIDs = append(p.DisabledRuleIDs, id)
			}
		}
	}
	p.Scope = scope
	p.Service = nil
	p.ServiceID = nil
	if scope != 0 {
		id := scope
		p.ServiceID = &id
	}
	return p
}

func (s *Service) checkScope(ctx context.Context, scope uint) error {
	if scope == 0 {
		return nil
	}
	var n int64
	if err := s.db.WithContext(ctx).Model(&domain.Service{}).Where("id = ?", scope).Count(&n).Error; err != nil {
		return err
	}
	if n == 0 {
		return domain.ErrServiceNotFound
	}
	return nil
}

func (s *Service) validatePolicy(p domain.RulePolicy) error {
	if p.Mode != "blocking" && p.Mode != "detection" && p.Mode != "off" {
		return domain.ValidationError("mode must be blocking, detection or off")
	}
	if p.BlockingParanoiaLevel < 1 || p.BlockingParanoiaLevel > 4 || p.DetectionParanoiaLevel < p.BlockingParanoiaLevel || p.DetectionParanoiaLevel > 4 {
		return domain.ValidationError("Paranoia levels must be 1..4 and detection must be at least blocking")
	}
	if p.InboundThreshold < 1 || p.InboundThreshold > 100 || p.MaxBodyBytes < 1024 || p.MaxBodyBytes > 128*1024*1024 || p.RateLimitPerMinute < 0 || p.RateLimitPerMinute > 1000000 {
		return domain.ValidationError("Policy limit is outside its allowed range")
	}
	if p.RateLimitAction != "block" && p.RateLimitAction != "log" {
		return domain.ValidationError("rateLimitAction must be block or log")
	}
	if len(p.DisabledRuleIDs) > 100 {
		return domain.ValidationError("At most 100 disabled rule IDs are permitted")
	}
	for _, id := range p.DisabledRuleIDs {
		if !s.ruleIDs[id] || id < 900000 || id >= 949000 && id < 1000000 {
			return domain.ValidationError("Only known CRS detection rules and bundled patch IDs may be disabled")
		}
		if id >= 900000 && id < 911000 {
			return domain.ValidationError("CRS initialization rules cannot be disabled")
		}
	}
	return nil
}

func validateRule(r domain.Rule) (compiledRule, error) {
	r.Name = strings.TrimSpace(r.Name)
	if r.Name == "" || len(r.Name) > 120 || len(r.Description) > 1000 || len(r.Conditions) < 1 || len(r.Conditions) > 10 {
		return compiledRule{}, domain.ValidationError("A rule requires a name (1..120 bytes), description up to 1000 bytes and 1..10 conditions")
	}
	if r.Action != "block" && r.Action != "log" {
		return compiledRule{}, domain.ValidationError("action must be block or log")
	}
	c := compiledRule{rule: r, regex: make([]*regexp.Regexp, len(r.Conditions)), networks: make([]netip.Prefix, len(r.Conditions))}
	for i, cond := range r.Conditions {
		switch cond.Target {
		case "path", "method", "ip":
			if cond.Key != "" {
				return c, domain.ValidationError("key is only supported for query and header conditions")
			}
		case "query", "header":
			if cond.Key == "" || len(cond.Key) > 128 || strings.ContainsAny(cond.Key, "\r\n\x00") {
				return c, domain.ValidationError("query and header conditions require a valid key")
			}
		default:
			return c, domain.ValidationError("Unknown condition target")
		}
		if cond.Value == "" || len(cond.Value) > 512 || strings.ContainsAny(cond.Value, "\r\n\x00") {
			return c, domain.ValidationError("Condition value must be 1..512 bytes without control characters")
		}
		switch cond.Operator {
		case "equals", "contains", "prefix", "suffix":
		case "regex":
			re, err := regexp.Compile(cond.Value)
			if err != nil {
				return c, domain.ValidationError("Invalid regular expression")
			}
			c.regex[i] = re
		case "cidr":
			if cond.Target != "ip" {
				return c, domain.ValidationError("cidr only supports the ip target")
			}
			network, err := netip.ParsePrefix(cond.Value)
			if err != nil {
				return c, domain.ValidationError("Invalid CIDR")
			}
			c.networks[i] = network.Masked()
		default:
			return c, domain.ValidationError("Unknown condition operator")
		}
	}
	return c, nil
}

func (s *Service) List(ctx context.Context, scope uint) ([]domain.Rule, error) {
	if err := s.checkScope(ctx, scope); err != nil {
		return nil, err
	}
	q := s.db.WithContext(ctx).Order("id")
	if scope == 0 {
		q = q.Where("service_id IS NULL")
	} else {
		q = q.Where("service_id = ?", scope)
	}
	out := []domain.Rule{}
	err := q.Find(&out).Error
	return out, err
}

func (s *Service) mutate(ctx context.Context, fn func(*gorm.DB) error) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	s.mu.RLock()
	previous := s.state
	s.mu.RUnlock()
	var next snapshot
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := fn(tx); err != nil {
			return err
		}
		reader := &Service{db: tx, Catalog: s.Catalog, ruleIDs: s.ruleIDs}
		var err error
		next, err = reader.load(ctx, previous)
		return err
	})
	if err != nil {
		closeNewEngines(next, previous)
	}
	if err == nil {
		s.mu.Lock()
		s.state = next
		s.mu.Unlock()
		closeNewEngines(previous, next)
		s.countersMu.Lock()
		s.counters = map[counterKey]*counter{}
		s.countersMu.Unlock()
	}
	return err
}

func (s *Service) SaveRule(ctx context.Context, id uint, input RuleInput) (domain.Rule, error) {
	r := domain.Rule{ID: id, ServiceID: input.ServiceID, Name: input.Name, Description: input.Description, Action: input.Action, Conditions: input.Conditions, Enabled: true}
	if input.Enabled != nil {
		r.Enabled = *input.Enabled
	}
	c, err := validateRule(r)
	if err != nil {
		return r, err
	}
	r = c.rule
	scope := uint(0)
	if r.ServiceID != nil {
		scope = *r.ServiceID
		if scope == 0 {
			return r, domain.ValidationError("serviceId must be positive or null")
		}
	}
	if err := s.checkScope(ctx, scope); err != nil {
		return r, err
	}
	err = s.mutate(ctx, func(tx *gorm.DB) error {
		if id == 0 {
			var count int64
			if err := tx.Model(&domain.Rule{}).Count(&count).Error; err != nil {
				return err
			}
			if count >= 500 {
				return domain.ValidationError("At most 500 custom rules are permitted")
			}
			return tx.Create(&r).Error
		}
		var old domain.Rule
		if err := tx.First(&old, id).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return fiber.ErrNotFound
		} else if err != nil {
			return err
		}
		oldScope := uint(0)
		if old.ServiceID != nil {
			oldScope = *old.ServiceID
		}
		if oldScope != scope {
			return domain.ValidationError("A rule's scope cannot be changed; create a new rule")
		}
		r.CreatedAt = old.CreatedAt
		return tx.Model(&old).Select("Name", "Description", "Enabled", "Action", "Conditions", "UpdatedAt").Updates(&r).Error
	})
	return r, err
}

func (s *Service) DeleteRule(ctx context.Context, id uint) error {
	return s.mutate(ctx, func(tx *gorm.DB) error {
		result := tx.Delete(&domain.Rule{}, id)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return fiber.ErrNotFound
		}
		return nil
	})
}

type PolicyView struct {
	Policy           domain.RulePolicy  `json:"policy"`
	Inherited        bool               `json:"inherited"`
	ConfiguredPolicy *domain.RulePolicy `json:"configuredPolicy"`
}

func (s *Service) Policy(ctx context.Context, scope uint) (PolicyView, error) {
	if err := s.checkScope(ctx, scope); err != nil {
		return PolicyView{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, exists := s.state.policies[scope]
	var configured *domain.RulePolicy
	if exists {
		raw := s.state.policies[scope]
		configured = &raw
	}
	return PolicyView{Policy: effectivePolicy(s.state, scope), Inherited: !exists, ConfiguredPolicy: configured}, nil
}
func (s *Service) SavePolicy(ctx context.Context, scope uint, in PolicyInput) (PolicyView, error) {
	p := domain.RulePolicy{Scope: scope, Mode: in.Mode, BlockingParanoiaLevel: in.BlockingParanoiaLevel, DetectionParanoiaLevel: in.DetectionParanoiaLevel, InboundThreshold: in.InboundThreshold, MaxBodyBytes: in.MaxBodyBytes, RateLimitPerMinute: in.RateLimitPerMinute, RateLimitAction: in.RateLimitAction, DisabledRuleIDs: append([]int{}, in.DisabledRuleIDs...)}
	if scope != 0 {
		p.ServiceID = &scope
	}
	if err := s.validatePolicy(p); err != nil {
		return PolicyView{}, err
	}
	if err := s.checkScope(ctx, scope); err != nil {
		return PolicyView{}, err
	}
	if err := s.mutate(ctx, func(tx *gorm.DB) error {
		if scope != 0 {
			var count int64
			if err := tx.Model(&domain.RulePolicy{}).Where("scope != 0 AND scope != ?", scope).Count(&count).Error; err != nil {
				return err
			}
			if count >= 128 {
				return domain.ValidationError("At most 128 service policy overrides are permitted")
			}
		}
		return tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "scope"}}, UpdateAll: true}).Create(&p).Error
	}); err != nil {
		return PolicyView{}, err
	}
	return s.Policy(ctx, scope)
}
func (s *Service) ResetPolicy(ctx context.Context, scope uint) error {
	if scope == 0 {
		return domain.ValidationError("The global policy cannot be deleted")
	}
	if err := s.checkScope(ctx, scope); err != nil {
		return err
	}
	return s.mutate(ctx, func(tx *gorm.DB) error { return tx.Delete(&domain.RulePolicy{}, "scope = ?", scope).Error })
}

func engineKey(p domain.RulePolicy) string {
	return fmt.Sprintf("%s/%d/%d/%d/%d/%v", p.Mode, p.BlockingParanoiaLevel, p.DetectionParanoiaLevel, p.InboundThreshold, p.MaxBodyBytes, p.DisabledRuleIDs)
}

func closeNewEngines(state, keep snapshot) {
	for scope, engine := range state.engines {
		if keep.engines[scope] != engine {
			if closer, ok := engine.(experimental.WAFCloser); ok {
				_ = closer.Close()
			}
		}
	}
}
func (s *Service) Close() {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	closeNewEngines(s.state, snapshot{})
}
