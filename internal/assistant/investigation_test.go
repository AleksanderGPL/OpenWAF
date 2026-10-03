package assistant

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/cloudwego/eino/components/tool/utils"
)

func TestInvestigationToolRecoversInvalidArguments(t *testing.T) {
	base, err := utils.InferTool("test_query", "Test query validation", func(ctx context.Context, q query) (string, error) {
		if _, err := q.filter(); err != nil {
			return "", err
		}
		return "valid", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var budget atomic.Int32
	bounded := boundedTool{InvokableTool: base, budget: &budget, parallel: make(chan struct{}, 3)}
	output, err := bounded.InvokableRun(context.Background(), `{"from":"2026-10-03T12:00:00Z","to":"invalid"}`)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]string
	if err := json.Unmarshal([]byte(output), &result); err != nil || result["error"] == "" || result["action"] == "" {
		t.Fatalf("missing corrective tool response: %s", output)
	}
	output, err = bounded.InvokableRun(context.Background(), `{"from":"2026-10-03T12:00:00Z","to":"2026-10-03T12:05:00Z"}`)
	if err != nil || output == "" || budget.Load() != 2 {
		t.Fatalf("corrected call failed: output=%s err=%v budget=%d", output, err, budget.Load())
	}
	budget.Store(24)
	if _, err := bounded.InvokableRun(context.Background(), `{}`); err == nil {
		t.Fatal("tool budget was bypassed")
	}
}

func TestInvestigationToolCancellationRemainsTerminal(t *testing.T) {
	base, err := utils.InferTool("test_cancel", "Test cancellation", func(ctx context.Context, q query) (string, error) {
		<-ctx.Done()
		return "", fmt.Errorf("query interrupted: %w", ctx.Err())
	})
	if err != nil {
		t.Fatal(err)
	}
	var budget atomic.Int32
	bounded := boundedTool{InvokableTool: base, budget: &budget, parallel: make(chan struct{}, 3)}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := bounded.InvokableRun(ctx, `{}`); err == nil {
		t.Fatal("cancellation was converted to recoverable tool output")
	}
}
