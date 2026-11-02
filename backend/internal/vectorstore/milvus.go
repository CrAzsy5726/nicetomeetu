package vectorstore

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

type Store struct {
	c          client.Client
	collection string
	dim        int
}

// Fact 是从 Milvus 检索出的一条画像事实。
type Fact struct {
	Type    string
	Content string
	Score   float32
}

func New(ctx context.Context, addr, collection string, dim int) (*Store, error) {
	c, err := client.NewClient(ctx, client.Config{Address: addr})
	if err != nil {
		return nil, fmt.Errorf("connect milvus: %w", err)
	}
	s := &Store{c: c, collection: collection, dim: dim}
	if err := s.ensure(ctx); err != nil {
		c.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() { s.c.Close() }

func (s *Store) ensure(ctx context.Context) error {
	has, err := s.c.HasCollection(ctx, s.collection)
	if err != nil {
		return fmt.Errorf("has collection: %w", err)
	}
	if !has {
		schema := entity.NewSchema().
			WithName(s.collection).
			WithDescription("user profile facts").
			WithField(entity.NewField().WithName("id").WithDataType(entity.FieldTypeInt64).WithIsPrimaryKey(true).WithIsAutoID(true)).
			WithField(entity.NewField().WithName("user_id").WithDataType(entity.FieldTypeVarChar).WithMaxLength(64)).
			WithField(entity.NewField().WithName("fact_type").WithDataType(entity.FieldTypeVarChar).WithMaxLength(32)).
			WithField(entity.NewField().WithName("content").WithDataType(entity.FieldTypeVarChar).WithMaxLength(2048)).
			WithField(entity.NewField().WithName("importance").WithDataType(entity.FieldTypeFloat)).
			WithField(entity.NewField().WithName("created_at").WithDataType(entity.FieldTypeInt64)).
			WithField(entity.NewField().WithName("updated_at").WithDataType(entity.FieldTypeInt64)).
			WithField(entity.NewField().WithName("source").WithDataType(entity.FieldTypeVarChar).WithMaxLength(1024)).
			WithField(entity.NewField().WithName("embedding").WithDataType(entity.FieldTypeFloatVector).WithDim(int64(s.dim)))

		if err := s.c.CreateCollection(ctx, schema, 1); err != nil {
			return fmt.Errorf("create collection: %w", err)
		}

		idx, err := entity.NewIndexFlat(entity.IP)
		if err != nil {
			return err
		}
		if err := s.c.CreateIndex(ctx, s.collection, "embedding", idx, false); err != nil {
			return fmt.Errorf("create index: %w", err)
		}
	}
	// 幂等加载，确保可检索
	if err := s.c.LoadCollection(ctx, s.collection, false); err != nil {
		return fmt.Errorf("load collection: %w", err)
	}
	return nil
}

// Insert 写入一条画像事实。
func (s *Store) Insert(ctx context.Context, userID, factType, content, source string, importance float32, embedding []float32) error {
	now := time.Now().Unix()
	cols := []entity.Column{
		entity.NewColumnVarChar("user_id", []string{userID}),
		entity.NewColumnVarChar("fact_type", []string{factType}),
		entity.NewColumnVarChar("content", []string{content}),
		entity.NewColumnFloat("importance", []float32{importance}),
		entity.NewColumnInt64("created_at", []int64{now}),
		entity.NewColumnInt64("updated_at", []int64{now}),
		entity.NewColumnVarChar("source", []string{source}),
		entity.NewColumnFloatVector("embedding", s.dim, [][]float32{embedding}),
	}
	_, err := s.c.Insert(ctx, s.collection, "", cols...)
	return err
}

// Search 在指定用户下做向量检索，返回 topK 条画像。
func (s *Store) Search(ctx context.Context, userID string, embedding []float32, topK int) ([]Fact, error) {
	return s.search(ctx, fmt.Sprintf(`user_id == "%s"`, userID), embedding, topK)
}

// SearchType 在指定用户 + 类型下检索（用于去重判断）。
func (s *Store) SearchType(ctx context.Context, userID, factType string, embedding []float32, topK int) ([]Fact, error) {
	expr := fmt.Sprintf(`user_id == "%s" and fact_type == "%s"`, userID, factType)
	return s.search(ctx, expr, embedding, topK)
}

func (s *Store) search(ctx context.Context, expr string, embedding []float32, topK int) ([]Fact, error) {
	sp, _ := entity.NewIndexFlatSearchParam()
	results, err := s.c.Search(ctx, s.collection, []string{}, expr,
		[]string{"fact_type", "content"}, []entity.Vector{entity.FloatVector(embedding)},
		"embedding", entity.IP, topK, sp)
	if err != nil {
		// Milvus 2.4.0 在 0 条匹配时不返回非主键字段，SDK 据此报错；视为无结果
		if isNoResultErr(err) {
			return nil, nil
		}
		return nil, err
	}
	if len(results) == 0 {
		return nil, nil
	}
	r := results[0]
	if r.Err != nil {
		if isNoResultErr(r.Err) {
			return nil, nil
		}
		return nil, r.Err
	}

	types := columnStrings(r.Fields.GetColumn("fact_type"))
	contents := columnStrings(r.Fields.GetColumn("content"))
	n := r.ResultCount
	facts := make([]Fact, 0, n)
	for i := 0; i < n; i++ {
		f := Fact{Score: r.Scores[i]}
		if i < len(contents) {
			f.Content = contents[i]
		}
		if i < len(types) {
			f.Type = types[i]
		}
		facts = append(facts, f)
	}
	return facts, nil
}

// Count 返回某用户已沉淀的画像条数（用于「了解度」）。
func (s *Store) Count(ctx context.Context, userID string) (int, error) {
	rs, err := s.c.Query(ctx, s.collection, []string{}, fmt.Sprintf(`user_id == "%s"`, userID), []string{"id"})
	if err != nil {
		if isNoResultErr(err) {
			return 0, nil
		}
		return 0, err
	}
	return rs.Len(), nil
}

func isNoResultErr(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	// Milvus 2.4.0 在 0 条匹配时，Search/Query 不会返回非主键字段数据，
	// 导致 SDK 解析时报以下两类错误；此时视为「无结果」而非真正的错误。
	return strings.Contains(s, "extra output fields") || strings.Contains(s, "type not matched")
}

// DeleteUser 清空某用户全部画像（重新回到陌生状态）。
func (s *Store) DeleteUser(ctx context.Context, userID string) error {
	return s.c.Delete(ctx, s.collection, "", fmt.Sprintf(`user_id == "%s"`, userID))
}

// FactRow 是一条画像的完整记录（含主键，供记忆面板展示与删除/纠错）。
// ID 用字符串传递：Milvus 自增主键是 int64，超出 JS 安全整数范围，直接 JSON 会丢精度。
type FactRow struct {
	ID         string  `json:"id"`
	Type       string  `json:"type"`
	Content    string  `json:"content"`
	Importance float32 `json:"importance"`
	CreatedAt  int64   `json:"created_at"`
	UpdatedAt  int64   `json:"updated_at"`
}

// ListFacts 返回某用户全部画像（按更新时间倒序）。
func (s *Store) ListFacts(ctx context.Context, userID string) ([]FactRow, error) {
	rs, err := s.c.Query(ctx, s.collection, []string{}, fmt.Sprintf(`user_id == "%s"`, userID),
		[]string{"id", "fact_type", "content", "importance", "created_at", "updated_at"})
	if err != nil {
		if isNoResultErr(err) {
			return []FactRow{}, nil
		}
		return nil, err
	}
	ids := columnInt64s(rs.GetColumn("id"))
	types := columnStrings(rs.GetColumn("fact_type"))
	contents := columnStrings(rs.GetColumn("content"))
	imps := columnFloats(rs.GetColumn("importance"))
	created := columnInt64s(rs.GetColumn("created_at"))
	updated := columnInt64s(rs.GetColumn("updated_at"))
	n := rs.Len()
	rows := make([]FactRow, 0, n)
	for i := 0; i < n; i++ {
		r := FactRow{}
		if i < len(ids) {
			r.ID = strconv.FormatInt(ids[i], 10)
		}
		if i < len(types) {
			r.Type = types[i]
		}
		if i < len(contents) {
			r.Content = contents[i]
		}
		if i < len(imps) {
			r.Importance = imps[i]
		}
		if i < len(created) {
			r.CreatedAt = created[i]
		}
		if i < len(updated) {
			r.UpdatedAt = updated[i]
		}
		rows = append(rows, r)
	}
	return rows, nil
}

// DeleteFact 按主键删除一条画像。
func (s *Store) DeleteFact(ctx context.Context, id string) error {
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return err
	}
	return s.c.Delete(ctx, s.collection, "", fmt.Sprintf("id == %d", n))
}

func columnStrings(col entity.Column) []string {
	if col == nil {
		return nil
	}
	switch v := col.(type) {
	case *entity.ColumnVarChar:
		return v.Data()
	case *entity.ColumnString:
		return v.Data()
	}
	return nil
}

func columnInt64s(col entity.Column) []int64 {
	if col == nil {
		return nil
	}
	if v, ok := col.(*entity.ColumnInt64); ok {
		return v.Data()
	}
	return nil
}

func columnFloats(col entity.Column) []float32 {
	if col == nil {
		return nil
	}
	if v, ok := col.(*entity.ColumnFloat); ok {
		return v.Data()
	}
	return nil
}
