package memory

import (
	"context"
	"strings"
	"sync"
	"time"

	"nicetomeetu/internal/extractor"
	"nicetomeetu/internal/llm"
	"nicetomeetu/internal/retriever"
	"nicetomeetu/internal/store"
	"nicetomeetu/internal/vectorstore"
)

const (
	defaultWorkers = 4   // 提取 worker 默认并发数
	jobQueueSize   = 256 // 提取任务缓冲
)

// ScopeKey 构造某用户在某伙伴下的画像作用域（用于按伙伴隔离记忆）。
func ScopeKey(userID, personaID string) string {
	return userID + "::" + personaID
}

// userFromScope 从作用域里取回用户 ID（用于 SSE 按用户推送）。
func userFromScope(scope string) string {
	if i := strings.Index(scope, "::"); i >= 0 {
		return scope[:i]
	}
	return scope
}

// personaFromScope 从作用域里取回伙伴 ID。
func personaFromScope(scope string) string {
	if i := strings.Index(scope, "::"); i >= 0 {
		return scope[i+2:]
	}
	return ""
}

type extractJob struct {
	scope    string
	messages []string
}

// Service 聚合三层记忆：短期（近期消息）、长期（Milvus 画像，按伙伴隔离）、日志（全量消息）。
// 内置有界 worker 池，异步处理画像提取，聊天主流程不被阻塞。
type Service struct {
	vs  *vectorstore.Store
	db  *store.Store
	ret *retriever.Retriever
	ex  *extractor.Extractor
	llm *llm.Client

	jobs chan extractJob
	wg   sync.WaitGroup
	hub  *hub
}

func New(vs *vectorstore.Store, db *store.Store, ret *retriever.Retriever, ex *extractor.Extractor, l *llm.Client, workers int) *Service {
	if workers <= 0 {
		workers = defaultWorkers
	}
	s := &Service{
		vs:   vs,
		db:   db,
		ret:  ret,
		ex:   ex,
		llm:  l,
		jobs: make(chan extractJob, jobQueueSize),
		hub:  newHub(),
	}
	for i := 0; i < workers; i++ {
		s.wg.Add(1)
		go s.worker()
	}
	return s
}

// Subscribe 订阅某用户的实时事件流（SSE 推送用）。
func (s *Service) Subscribe(userID string) (<-chan Event, func()) {
	return s.hub.subscribe(userID)
}

func (s *Service) worker() {
	defer s.wg.Done()
	for job := range s.jobs {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		if err := s.ex.Run(ctx, job.scope, job.messages); err == nil {
			// 提取完成后广播实时事件，前端据此刷新了解度
			if n, err := s.vs.Count(ctx, job.scope); err == nil {
				s.hub.publish(userFromScope(job.scope), Event{Type: "memory_updated", Familiarity: n})
			}
		}
		cancel()
	}
}

// Close 关闭 worker 池并等待在途任务完成。
func (s *Service) Close() {
	close(s.jobs)
	s.wg.Wait()
}

// RememberAsync 将画像提取任务投递到 worker 池，立即返回、不阻塞调用方。
func (s *Service) RememberAsync(scope string, recentUserMessages []string) {
	if len(recentUserMessages) == 0 {
		return
	}
	select {
	case s.jobs <- extractJob{scope: scope, messages: recentUserMessages}:
	default:
		// 极端情况下队列已满则丢弃，优先保证聊天响应不被拖慢
	}
}

// RetrieveProfile 检索当前最相关的长期画像。
func (s *Service) RetrieveProfile(ctx context.Context, scope, query string, topK int) ([]vectorstore.Fact, error) {
	return s.ret.Retrieve(ctx, scope, query, topK)
}

// ListFacts 返回当前伙伴下的全部画像（供记忆面板展示）。
func (s *Service) ListFacts(ctx context.Context, scope string) ([]vectorstore.FactRow, error) {
	return s.vs.ListFacts(ctx, scope)
}

// EmptyDimensions 返回尚无画像的维度类型（用于引导 AI 补足认知空白）。
func (s *Service) EmptyDimensions(ctx context.Context, scope string) ([]string, error) {
	facts, err := s.vs.ListFacts(ctx, scope)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, f := range facts {
		seen[f.Type] = true
	}
	var empty []string
	for _, t := range []string{"basic", "interest", "personality", "emotion", "episodic"} {
		if !seen[t] {
			empty = append(empty, t)
		}
	}
	return empty, nil
}

// DeleteFact 删除一条画像。
func (s *Service) DeleteFact(ctx context.Context, scope, id string) error {
	if err := s.vs.DeleteFact(ctx, id); err != nil {
		return err
	}
	s.publishCount(ctx, scope)
	return nil
}

// CorrectFact 修正一条画像：删除旧条目并按新内容重插。
func (s *Service) CorrectFact(ctx context.Context, scope string, id string, factType, newContent string) error {
	if err := s.vs.DeleteFact(ctx, id); err != nil {
		return err
	}
	if err := s.ex.InsertFact(ctx, scope, factType, newContent, 3); err != nil {
		return err
	}
	s.publishCount(ctx, scope)
	return nil
}

func (s *Service) publishCount(ctx context.Context, scope string) {
	if n, err := s.vs.Count(ctx, scope); err == nil {
		s.hub.publish(userFromScope(scope), Event{Type: "memory_updated", Familiarity: n})
	}
}

// Digest 用 LLM 把已知画像整理成一段有人情味的第一人称摘要。
func (s *Service) Digest(ctx context.Context, scope string) (string, error) {
	facts, err := s.vs.ListFacts(ctx, scope)
	if err != nil {
		return "", err
	}
	if len(facts) == 0 {
		return "我们才刚刚认识，我对你还了解不多。多聊聊，让我慢慢认识你吧。", nil
	}
	var sb strings.Builder
	for _, f := range facts {
		sb.WriteString("- " + f.Content + "\n")
	}
	sys := `你是一个 AI 聊天伙伴，要把已知的用户画像整理成一段温暖自然的第一人称摘要，像朋友在回顾你们认识以来的了解。用中文，2~4 句话，不要逐条罗列，要有人情味。`
	return s.llm.Chat(ctx, []llm.Message{
		{Role: "system", Content: sys},
		{Role: "user", Content: "我目前了解的用户信息：\n" + sb.String()},
	})
}

// Flashback 主动回忆：挑一条「很久没聊起」的画像，生成温暖的关心式追问。
func (s *Service) Flashback(ctx context.Context, scope string) (string, error) {
	facts, err := s.vs.ListFacts(ctx, scope)
	if err != nil {
		return "", err
	}
	if len(facts) == 0 {
		return "我们才刚认识，等我多了解你一些，就能主动想起更多关于你的事啦。", nil
	}
	best := pickFlashback(facts)

	sys := `你是用户的 AI 聊天伙伴。请基于下面这条关于用户的记忆，主动发起一个温暖、自然的关心式提问，像朋友突然想起来一样。用中文，1~2 句话，口语化、不生硬，不要重复罗列记忆原文。`
	return s.llm.Chat(ctx, []llm.Message{
		{Role: "system", Content: sys},
		{Role: "user", Content: "你记得关于用户的一条信息：\n" + best.Content},
	})
}

// WriteDiary 让 AI 以第一人称写一篇今天的日记，并存入 PostgreSQL。
func (s *Service) WriteDiary(ctx context.Context, scope string) (string, error) {
	userID := userFromScope(scope)
	sessionID, _ := s.db.LatestSession(ctx, userID, personaFromScope(scope))
	var recent []store.Message
	if sessionID != "" {
		recent, _ = s.db.RecentMessages(ctx, sessionID, 20)
	}
	facts, _ := s.vs.ListFacts(ctx, scope)

	var sb strings.Builder
	if len(recent) > 0 {
		sb.WriteString("最近对话：\n")
		for _, m := range recent {
			sb.WriteString(m.Role + "：" + m.Content + "\n")
		}
	}
	if len(facts) > 0 {
		sb.WriteString("\n你了解的用户画像：\n")
		for _, f := range facts {
			sb.WriteString("- " + f.Content + "\n")
		}
	}

	sys := `你是用户的 AI 聊天伙伴，正在写一篇今天的日记。用第一人称「我」，以温暖、真诚、带点小情绪的口吻，回顾今天和用户的相处。2~4 句话，像朋友记日记一样自然，不要写成汇报或清单。`
	entry, err := s.llm.Chat(ctx, []llm.Message{
		{Role: "system", Content: sys},
		{Role: "user", Content: sb.String()},
	})
	if err != nil {
		return "", err
	}
	if err := s.db.AddDiaryEntry(ctx, userID, personaFromScope(scope), entry); err != nil {
		return "", err
	}
	return entry, nil
}

// ListDiary 返回当前伙伴的日记（按时间倒序）。
func (s *Service) ListDiary(ctx context.Context, scope string) ([]store.DiaryEntry, error) {
	return s.db.ListDiaryEntries(ctx, userFromScope(scope), personaFromScope(scope))
}

func pickFlashback(facts []vectorstore.FactRow) vectorstore.FactRow {
	// 优先 episodic / interest（更生活化、更值得跟进）
	preferred := make([]vectorstore.FactRow, 0, len(facts))
	for _, f := range facts {
		if f.Type == "episodic" || f.Type == "interest" {
			preferred = append(preferred, f)
		}
	}
	if len(preferred) == 0 {
		preferred = facts
	}
	// 选最早记住的那条，营造「好久没聊起」的感觉
	best := preferred[0]
	for _, f := range preferred {
		if f.CreatedAt < best.CreatedAt {
			best = f
		}
	}
	return best
}

// Consolidate 后台整合：清理过期且低重要度的事件型画像。
func (s *Service) Consolidate(ctx context.Context, scope string) error {
	facts, err := s.vs.ListFacts(ctx, scope)
	if err != nil {
		return err
	}
	const episodicTTL = 30 * 24 * 3600 // 30 天
	now := time.Now().Unix()
	for _, f := range facts {
		if f.Type == "episodic" && f.Importance < 3 && now-f.UpdatedAt > episodicTTL {
			_ = s.vs.DeleteFact(ctx, f.ID)
		}
	}
	return nil
}

// Familiarity 返回已沉淀的画像条数。
func (s *Service) Familiarity(ctx context.Context, scope string) (int, error) {
	return s.vs.Count(ctx, scope)
}

// Reset 清空当前伙伴的长期画像与聊天日志，回到陌生状态（保留人设）。
func (s *Service) Reset(ctx context.Context, scope string) error {
	userID := userFromScope(scope)
	personaID := personaFromScope(scope)
	if err := s.vs.DeleteUser(ctx, scope); err != nil {
		return err
	}
	if err := s.db.ClearPersonaMessages(ctx, userID, personaID); err != nil {
		return err
	}
	if err := s.db.ClearPersonaDiary(ctx, userID, personaID); err != nil {
		return err
	}
	s.hub.publish(userID, Event{Type: "memory_reset", Familiarity: 0})
	return nil
}

// RecentMessages 获取短期记忆（最近 N 条消息）。
func (s *Service) RecentMessages(ctx context.Context, sessionID string, limit int) ([]store.Message, error) {
	return s.db.RecentMessages(ctx, sessionID, limit)
}
