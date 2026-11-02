package memory

import (
	"testing"

	"nicetomeetu/internal/vectorstore"
)

func TestPickFlashbackPrefersInterest(t *testing.T) {
	facts := []vectorstore.FactRow{
		{Content: "用户是产品经理", Type: "basic", CreatedAt: 100},
		{Content: "用户喜欢画画", Type: "interest", CreatedAt: 200},
	}
	got := pickFlashback(facts)
	if got.Content != "用户喜欢画画" {
		t.Errorf("expected interest fact, got %s", got.Content)
	}
}

func TestPickFlashbackOldestWithinType(t *testing.T) {
	facts := []vectorstore.FactRow{
		{Content: "用户喜欢篮球", Type: "interest", CreatedAt: 100},
		{Content: "用户喜欢画画", Type: "interest", CreatedAt: 200},
	}
	got := pickFlashback(facts)
	if got.Content != "用户喜欢篮球" {
		t.Errorf("expected oldest interest fact, got %s", got.Content)
	}
}

func TestPickFlashbackFallback(t *testing.T) {
	facts := []vectorstore.FactRow{
		{Content: "用户是产品经理", Type: "basic", CreatedAt: 100},
	}
	got := pickFlashback(facts)
	if got.Content != "用户是产品经理" {
		t.Errorf("expected the only fact, got %s", got.Content)
	}
}

func BenchmarkPickFlashback(b *testing.B) {
	facts := []vectorstore.FactRow{
		{Content: "用户喜欢篮球", Type: "interest", CreatedAt: 100},
		{Content: "用户喜欢画画", Type: "interest", CreatedAt: 200},
		{Content: "用户是产品经理", Type: "basic", CreatedAt: 50},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = pickFlashback(facts)
	}
}
