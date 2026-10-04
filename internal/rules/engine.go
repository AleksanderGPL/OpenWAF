package rules

import (
	_ "embed"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"OpenWAF/internal/domain"
	crs "github.com/corazawaf/coraza-coreruleset/v4"
	"github.com/corazawaf/coraza/v3"
)

const CRSVersion = "4.25.0"

//go:embed filters/exposure.conf
var patches string

type CatalogRule struct {
	ID      int      `json:"id"`
	Source  string   `json:"source"`
	File    string   `json:"file,omitempty"`
	Message string   `json:"message"`
	Tags    []string `json:"tags"`
}

func catalog() ([]CatalogRule, error) {
	paths, err := fs.Glob(crs.FS, "@owasp_crs/*.conf")
	if err != nil {
		return nil, err
	}
	paths = append(paths, "@coraza.conf-recommended")
	idRE := regexp.MustCompile(`\bid:\s*'?([0-9]+)'?`)
	msgRE := regexp.MustCompile(`\bmsg:'([^']*)'`)
	tagRE := regexp.MustCompile(`\btag:'([^']*)'`)
	out := make([]CatalogRule, 0)
	add := func(content, source, file string) {
		content = strings.ReplaceAll(content, "\\\n", " ")
		for _, line := range strings.Split(content, "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "SecRule ") && !strings.HasPrefix(line, "SecAction ") {
				continue
			}
			m := idRE.FindStringSubmatch(line)
			if len(m) != 2 {
				continue
			}
			id, _ := strconv.Atoi(m[1])
			item := CatalogRule{ID: id, Source: source, File: file, Tags: []string{}}
			if m := msgRE.FindStringSubmatch(line); len(m) == 2 {
				item.Message = m[1]
			}
			item.Message = catalogMessage(id, source, item.Message)
			for _, m := range tagRE.FindAllStringSubmatch(line, -1) {
				item.Tags = append(item.Tags, m[1])
			}
			out = append(out, item)
		}
	}
	for _, path := range paths {
		b, err := fs.ReadFile(crs.FS, path)
		if err != nil {
			return nil, err
		}
		source := "crs"
		if path == "@coraza.conf-recommended" {
			source = "coraza"
		}
		add(string(b), source, path)
	}
	add(patches, "patch", "exposure.conf")
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func defaultPolicy() domain.RulePolicy {
	return domain.RulePolicy{Scope: 0, Mode: "blocking", BlockingParanoiaLevel: 1, DetectionParanoiaLevel: 2, InboundThreshold: 5, MaxBodyBytes: 13 * 1024 * 1024, RateLimitPerMinute: 600, RateLimitAction: "log", DisabledRuleIDs: []int{}}
}

func compile(p domain.RulePolicy) (coraza.WAF, error) {
	mode := map[string]string{"blocking": "On", "detection": "DetectionOnly", "off": "Off"}[p.Mode]
	directives := fmt.Sprintf(`
SecRuleEngine %s
SecAuditEngine Off
SecDebugLogLevel 0
SecResponseBodyAccess Off
SecRequestBodyLimit %d
SecRequestBodyLimitAction Reject
SecAction "id:1001000,phase:1,pass,nolog,t:none,setvar:tx.crs_setup_version=4250,setvar:tx.blocking_paranoia_level=%d,setvar:tx.detection_paranoia_level=%d,setvar:tx.inbound_anomaly_score_threshold=%d"
`, mode, p.MaxBodyBytes, p.BlockingParanoiaLevel, p.DetectionParanoiaLevel, p.InboundThreshold)
	cfg := coraza.NewWAFConfig().WithRootFS(crs.FS).
		WithDirectivesFromFile("@coraza.conf-recommended").
		WithDirectivesFromFile("@crs-setup.conf.example").
		WithDirectives(directives + patches).
		WithDirectivesFromFile("@owasp_crs/*.conf")
	if len(p.DisabledRuleIDs) > 0 {
		ids := make([]string, len(p.DisabledRuleIDs))
		for i, id := range p.DisabledRuleIDs {
			ids[i] = strconv.Itoa(id)
		}
		cfg = cfg.WithDirectives("SecRuleRemoveById " + strings.Join(ids, " "))
	}
	return coraza.NewWAF(cfg)
}
