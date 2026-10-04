package rules

import (
	"net/url"
	"testing"
)

func TestDefaultPolicyInitializesCRS(t *testing.T) {
	engine, err := compile(defaultPolicy())
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		path   string
		status int
	}{
		{"ordinary search passes", "/api/todos?q=OpenWAF", 0},
		{"SQL injection is blocked", "/api/todos?q=" + url.QueryEscape("' UNION SELECT id,title,done,priority FROM internal_notes-- "), 403},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			tx := engine.NewTransaction()
			defer tx.Close()
			tx.ProcessConnection("127.0.0.1", 1234, "127.0.0.1", 8080)
			tx.ProcessURI(test.path, "GET", "HTTP/1.1")
			tx.AddRequestHeader("Host", "demo.test")
			tx.AddRequestHeader("Accept", "*/*")
			tx.AddRequestHeader("User-Agent", "OpenWAF regression test")
			interruption := tx.ProcessRequestHeaders()
			if interruption == nil {
				interruption, err = tx.ProcessRequestBody()
				if err != nil {
					t.Fatal(err)
				}
			}
			status := 0
			if interruption != nil {
				status = interruption.Status
			}
			if status != test.status {
				t.Fatalf("status = %d, want %d", status, test.status)
			}
			tx.ProcessLogging()
		})
	}
}
