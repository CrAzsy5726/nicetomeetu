package prompt

import (
	"strings"
	"testing"

	"nicetomeetu/internal/persona"
)

func testPersona() persona.Persona {
	return persona.Persona{ID: "aye", Name: "阿野", Personality: "热情", Style: "爽朗", Opening: "嘿！"}
}

func TestBuildSystemEmptyFacts(t *testing.T) {
	s := BuildSystem(testPersona(), nil, nil)
	if !strings.Contains(s, "你们才刚刚认识") {
		t.Errorf("expected stranger hint, got:\n%s", s)
	}
	if strings.Contains(s, "用户喜欢") {
		t.Errorf("should not contain facts when empty")
	}
}

func TestBuildSystemWithFacts(t *testing.T) {
	facts := []ProfileFact{{Type: "interest", Content: "用户喜欢打篮球"}}
	s := BuildSystem(testPersona(), facts, nil)
	if !strings.Contains(s, "用户喜欢打篮球") {
		t.Errorf("expected fact content, got:\n%s", s)
	}
	if strings.Contains(s, "你们才刚刚认识") {
		t.Errorf("stranger hint should be absent when facts exist")
	}
}

func TestBuildSystemGapHint(t *testing.T) {
	facts := []ProfileFact{{Type: "interest", Content: "用户喜欢打篮球"}}
	s := BuildSystem(testPersona(), facts, []string{"personality", "emotion"})
	if !strings.Contains(s, "「性格风格」") || !strings.Contains(s, "「情绪状态」") {
		t.Errorf("expected gap hint naming empty dims, got:\n%s", s)
	}
}

func TestBuildSystemNoGapHintWhenComplete(t *testing.T) {
	facts := []ProfileFact{{Type: "interest", Content: "x"}}
	s := BuildSystem(testPersona(), facts, nil)
	if strings.Contains(s, "还了解很少") {
		t.Errorf("should not emit gap hint when emptyTypes is nil")
	}
}

func TestBuildSystemEmotionRule(t *testing.T) {
	s := BuildSystem(testPersona(), nil, nil)
	if !strings.Contains(s, "情绪状态") || !strings.Contains(s, "共情") {
		t.Errorf("expected emotion empathy rule, got:\n%s", s)
	}
}

func BenchmarkBuildSystem(b *testing.B) {
	p := testPersona()
	facts := []ProfileFact{{Type: "interest", Content: "用户喜欢打篮球"}}
	empty := []string{"personality", "emotion"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = BuildSystem(p, facts, empty)
	}
}
