package retriever

import (
	"context"
	"sort"
	"sync"

	"nicetomeetu/internal/embedding"
	"nicetomeetu/internal/vectorstore"
)

// factTypes 是画像的五个维度，检索时并发覆盖，避免只命中当前话题最相关的单一维度。
var factTypes = []string{"basic", "interest", "personality", "emotion", "episodic"}

type Retriever struct {
	emb *embedding.Client
	vs  *vectorstore.Store
}

func New(e *embedding.Client, vs *vectorstore.Store) *Retriever {
	return &Retriever{emb: e, vs: vs}
}

// Retrieve 并发检索各维度画像并合并，保证「了解」的覆盖均衡。
func (r *Retriever) Retrieve(ctx context.Context, scope, query string, topK int) ([]vectorstore.Fact, error) {
	vec, err := r.emb.Embed(ctx, query)
	if err != nil {
		return nil, err
	}

	perType := topK/len(factTypes) + 1

	var (
		mu     sync.Mutex
		merged []vectorstore.Fact
		wg     sync.WaitGroup
	)
	for _, t := range factTypes {
		wg.Add(1)
		go func(t string) {
			defer wg.Done()
			fs, err := r.vs.SearchType(ctx, scope, t, vec, perType)
			if err != nil {
				return // 单维度失败不影响整体
			}
			mu.Lock()
			merged = append(merged, fs...)
			mu.Unlock()
		}(t)
	}
	wg.Wait()

	sort.SliceStable(merged, func(i, j int) bool { return merged[i].Score > merged[j].Score })
	if len(merged) > topK {
		merged = merged[:topK]
	}
	return merged, nil
}
