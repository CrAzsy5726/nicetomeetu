package extractor

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"nicetomeetu/internal/embedding"
	"nicetomeetu/internal/llm"
	"nicetomeetu/internal/vectorstore"
)

const (
	similarityThreshold = 0.90 // 高于此相似度视为近似重复，跳过写入
)

var allowedTypes = map[string]bool{
	"basic":       true,
	"interest":    true,
	"personality": true,
	"emotion":     true,
	"episodic":    true,
}

type Fact struct {
	Type       string  `json:"type"`
	Content    string  `json:"content"`
	Importance float32 `json:"importance"`
	Replaces   string  `json:"replaces,omitempty"`
}

type output struct {
	Facts []Fact `json:"facts"`
}

type Extractor struct {
	llm *llm.Client
	emb *embedding.Client
	vs  *vectorstore.Store
}

func New(l *llm.Client, e *embedding.Client, vs *vectorstore.Store) *Extractor {
	return &Extractor{llm: l, emb: e, vs: vs}
}

// Run 从用户最近发言中提取画像事实并写入向量库（含去重与更新检测）。
func (e *Extractor) Run(ctx context.Context, scope string, recentUserMessages []string) error {
	if len(recentUserMessages) == 0 {
		return nil
	}

	// 拉取已有画像：既传给提示词做「更新检测」，也用于删除被修正的旧条目
	known, err := e.vs.ListFacts(ctx, scope)
	if err != nil {
		known = nil
	}
	known = sortByRecent(known, 30)

	facts, err := e.extract(ctx, recentUserMessages, known)
	if err != nil {
		return err
	}

	// 过滤并规范化
	valid := sanitize(facts)
	if len(valid) == 0 {
		return nil
	}

	// content → id 映射，用于按 replaces 删除旧画像
	idByContent := make(map[string]string, len(known))
	for _, k := range known {
		idByContent[k.Content] = k.ID
	}

	// 批量向量化：一次请求处理所有事实，显著降低延迟与 API 调用次数
	contents := make([]string, len(valid))
	for i, f := range valid {
		contents[i] = f.Content
	}
	vecs, err := e.emb.EmbedBatch(ctx, contents)
	if err != nil {
		return err
	}
	if len(vecs) != len(valid) {
		return fmt.Errorf("embed batch returned %d vectors for %d facts", len(vecs), len(valid))
	}

	for i, f := range valid {
		// 若标记了 replaces，先删除被修正/否定的旧画像
		if f.Replaces != "" {
			if oldID, ok := idByContent[f.Replaces]; ok {
				_ = e.vs.DeleteFact(ctx, oldID)
			}
		}
		if err := e.upsertVec(ctx, scope, f, vecs[i]); err != nil {
			return err
		}
	}
	return nil
}

func sortByRecent(facts []vectorstore.FactRow, limit int) []vectorstore.FactRow {
	sort.Slice(facts, func(i, j int) bool { return facts[i].UpdatedAt > facts[j].UpdatedAt })
	if len(facts) > limit {
		facts = facts[:limit]
	}
	return facts
}

// sanitize 过滤空条目、规范化类型与重要度。
func sanitize(facts []Fact) []Fact {
	valid := make([]Fact, 0, len(facts))
	for _, f := range facts {
		if f.Type == "" || f.Content == "" {
			continue
		}
		if !allowedTypes[f.Type] {
			f.Type = "basic"
		}
		if f.Importance < 1 || f.Importance > 5 {
			f.Importance = 3
		}
		valid = append(valid, f)
	}
	return valid
}

func (e *Extractor) extract(ctx context.Context, userMessages []string, known []vectorstore.FactRow) ([]Fact, error) {
	sys := `你是一个严谨的用户画像提取器，从用户发言中抽取关于用户本人的稳定事实。

【铁律】
1. 每条 content 必须原样引用用户发言中出现的词语，不能改写、不能替换、不能添加、不能联想（例如原文说"篮球"就绝不能写成"乒乓球"或"健身"）。
2. 宁可漏掉，也不可编造；没有把握就跳过。
3. content 一律以"用户"开头，用第三人称简洁表述。
4. type 只能取：basic（基本信息/称呼/职业/城市）、interest（兴趣爱好/喜欢与讨厌）、personality（性格/说话风格）、emotion（情绪/关系状态）、episodic（近期事件）。
5. importance 为 1-5 整数。

【更新检测】
如果用户的新发言修正或否定了某条「已有画像」（例如职业变了、喜好变了、状态变了），请在该条事实里加上 "replaces" 字段，值必须是那条已有画像的原文（逐字复制）。否则不要加 replaces。使用 replaces 时，content 直接表述修正后的当前状态（例如"用户是老师"），不要带"现在""改""以前"这类变化词。

【示例】
输入："我喜欢踢足球"
输出：{"facts":[{"type":"interest","content":"用户喜欢踢足球","importance":4}]}

【输出格式】只返回 JSON：{"facts":[{"type":"...","content":"...","importance":3,"replaces":"被修正的原文（可选）"}]}`

	var sb strings.Builder
	if len(known) == 0 {
		sb.WriteString("已有画像：\n（暂无）\n\n")
	} else {
		sb.WriteString("已有画像：\n")
		for _, k := range known {
			sb.WriteString("- " + k.Content + "\n")
		}
		sb.WriteString("\n")
	}
	sb.WriteString("用户发言：\n")
	for _, m := range userMessages {
		sb.WriteString(m + "\n")
	}

	raw, err := e.llm.ChatJSON(ctx, []llm.Message{
		{Role: "system", Content: sys},
		{Role: "user", Content: sb.String()},
	})
	if err != nil {
		return nil, err
	}

	var out output
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("parse extract json: %w (raw=%s)", err, raw)
	}
	return out.Facts, nil
}

func (e *Extractor) upsertVec(ctx context.Context, scope string, f Fact, vec []float32) error {
	// 同类型下检索最相似的一条，若近似重复则跳过，避免向量库膨胀
	similar, err := e.vs.SearchType(ctx, scope, f.Type, vec, 1)
	if err != nil {
		return err
	}
	if len(similar) > 0 && similar[0].Score > similarityThreshold {
		return nil
	}

	return e.vs.Insert(ctx, scope, f.Type, f.Content, "", f.Importance, vec)
}

// InsertFact 直接写入一条画像（用于用户手动纠错后重插）。
func (e *Extractor) InsertFact(ctx context.Context, scope, factType, content string, importance float32) error {
	if content == "" {
		return nil
	}
	if !allowedTypes[factType] {
		factType = "basic"
	}
	if importance < 1 || importance > 5 {
		importance = 3
	}
	vec, err := e.emb.Embed(ctx, content)
	if err != nil {
		return err
	}
	return e.vs.Insert(ctx, scope, factType, content, "", importance, vec)
}
