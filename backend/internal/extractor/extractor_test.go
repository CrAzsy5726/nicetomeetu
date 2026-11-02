package extractor

import (
	"testing"

	"nicetomeetu/internal/vectorstore"
)

func TestSanitize(t *testing.T) {
	in := []Fact{
		{Type: "", Content: "空类型"},                                        // 丢弃
		{Type: "unknown", Content: "未知类型", Importance: 3},                  // 映射为 basic
		{Type: "interest", Content: "用户喜欢篮球", Importance: 99},           // importance 钳制为 3
		{Type: "basic", Content: "用户叫林一", Importance: 5},                // 保留
		{Type: "emotion", Content: "", Importance: 3},                       // content 空，丢弃
	}
	out := sanitize(in)
	if len(out) != 3 {
		t.Fatalf("expected 3 valid facts, got %d: %+v", len(out), out)
	}
	if out[0].Type != "basic" {
		t.Errorf("unknown type should map to basic, got %s", out[0].Type)
	}
	if out[1].Importance != 3 {
		t.Errorf("importance 99 should clamp to 3, got %v", out[1].Importance)
	}
}

func TestSortByRecent(t *testing.T) {
	facts := []vectorstore.FactRow{
		{Content: "a", UpdatedAt: 100},
		{Content: "b", UpdatedAt: 300},
		{Content: "c", UpdatedAt: 200},
	}
	got := sortByRecent(facts, 2)
	if len(got) != 2 {
		t.Fatalf("expected limit 2, got %d", len(got))
	}
	if got[0].Content != "b" || got[1].Content != "c" {
		t.Errorf("expected [b c], got [%s %s]", got[0].Content, got[1].Content)
	}
}

func BenchmarkSanitize(b *testing.B) {
	facts := []Fact{
		{Type: "basic", Content: "用户叫林一", Importance: 5},
		{Type: "interest", Content: "用户喜欢打篮球", Importance: 4},
		{Type: "unknown", Content: "x", Importance: 3},
		{Type: "episodic", Content: "用户最近在学吉他", Importance: 3},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sanitize(facts)
	}
}
