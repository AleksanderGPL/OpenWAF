package rules

import (
	"strings"
	"testing"
)

func TestCatalogMessages(t *testing.T) {
	items, err := catalog()
	if err != nil {
		t.Fatal(err)
	}
	byID := make(map[int]CatalogRule, len(items))
	for _, item := range items {
		byID[item.ID] = item
		if strings.TrimSpace(item.Message) == "" {
			t.Errorf("rule %d has no message", item.ID)
		}
		if strings.HasPrefix(item.Message, "Internal ") {
			t.Errorf("bundled rule %d needs a description: %s", item.ID, item.Message)
		}
		if strings.Contains(item.Message, "%{") {
			t.Errorf("rule %d message still contains a macro: %s", item.ID, item.Message)
		}
	}
	for id, description := range builtinDescriptions {
		item, ok := byID[id]
		if !ok {
			t.Errorf("description for rule %d is no longer in the bundled catalog", id)
			continue
		}
		if item.Message != description {
			t.Errorf("rule %d: got %q, want %q; check whether upstream added a message", id, item.Message, description)
		}
	}
	for id, want := range map[int]string{
		901340:  "Enabling body inspection",
		920540:  "Possible Unicode character bypass detected",
		1001001: "Environment file exposure",
		949110:  "Inbound Anomaly Score Exceeded",
		949111:  "Inbound Anomaly Score Exceeded in phase 1",
	} {
		if got := byID[id].Message; got != want {
			t.Errorf("rule %d upstream message: got %q, want %q", id, got, want)
		}
	}
	s := &Service{Catalog: items}
	if got := s.message(200001); got != "Select JSON request body processor" {
		t.Errorf("request log message: got %q", got)
	}
}

func TestCatalogMessagePrecedence(t *testing.T) {
	for _, tc := range []struct {
		name     string
		id       int
		source   string
		upstream string
		want     string
	}{
		{"upstream before description", 200001, "coraza", "Upstream message", "Upstream message"},
		{"description", 200001, "coraza", "", "Select JSON request body processor"},
		{"unknown CRS rule", 999999, "crs", "", "Internal CRS rule 999999"},
		{"unknown Coraza rule", 200999, "coraza", "", "Internal CORAZA rule 200999"},
		{"unknown patch rule", 1001999, "patch", "", "Internal PATCH rule 1001999"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := catalogMessage(tc.id, tc.source, tc.upstream); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
