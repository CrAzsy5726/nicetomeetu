package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

func New(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	s := &Store{pool: pool}
	if err := s.migrate(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() { s.pool.Close() }

func (s *Store) migrate(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			username TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			persona_id TEXT,
			created_at BIGINT
		)`,
		`CREATE TABLE IF NOT EXISTS auth_tokens (
			token_hash TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			created_at BIGINT,
			expires_at BIGINT
		)`,
		`CREATE TABLE IF NOT EXISTS chat_sessions (
			id TEXT PRIMARY KEY,
			user_id TEXT,
			persona_id TEXT,
			title TEXT,
			created_at BIGINT
		)`,
		`CREATE TABLE IF NOT EXISTS messages (
			id BIGSERIAL PRIMARY KEY,
			session_id TEXT,
			user_id TEXT,
			role TEXT,
			content TEXT,
			created_at BIGINT
		)`,
		`CREATE TABLE IF NOT EXISTS diary (
			id BIGSERIAL PRIMARY KEY,
			user_id TEXT,
			persona_id TEXT,
			content TEXT,
			created_at BIGINT
		)`,
		`CREATE TABLE IF NOT EXISTS personas (
			id TEXT PRIMARY KEY,
			user_id TEXT,
			name TEXT,
			avatar TEXT,
			personality TEXT,
			style TEXT NOT NULL DEFAULT '',
			accent TEXT NOT NULL DEFAULT '',
			tagline TEXT NOT NULL DEFAULT '',
			opening TEXT NOT NULL DEFAULT '',
			scene TEXT NOT NULL DEFAULT '',
			hidden BOOLEAN NOT NULL DEFAULT FALSE,
			turns BIGINT NOT NULL DEFAULT 0,
			created_at BIGINT
		)`,
		// 兼容旧库：为已存在的表补充新增列（幂等）
		`ALTER TABLE chat_sessions ADD COLUMN IF NOT EXISTS persona_id TEXT`,
		`ALTER TABLE diary ADD COLUMN IF NOT EXISTS persona_id TEXT`,
		`ALTER TABLE personas ADD COLUMN IF NOT EXISTS style TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE personas ADD COLUMN IF NOT EXISTS accent TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE personas ADD COLUMN IF NOT EXISTS tagline TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE personas ADD COLUMN IF NOT EXISTS opening TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE personas ADD COLUMN IF NOT EXISTS scene TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE personas ADD COLUMN IF NOT EXISTS hidden BOOLEAN NOT NULL DEFAULT FALSE`,
		`ALTER TABLE personas ADD COLUMN IF NOT EXISTS turns BIGINT NOT NULL DEFAULT 0`,
		`CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(session_id, id)`,
		`CREATE INDEX IF NOT EXISTS idx_auth_tokens_user ON auth_tokens(user_id)`,
	}
	for _, stmt := range stmts {
		if _, err := s.pool.Exec(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

// User 是用户记录。
type User struct {
	ID           string `json:"id"`
	Username     string `json:"username"`
	PasswordHash string `json:"-"`
	PersonaID    string `json:"persona_id"`
}

// CreateUser 创建用户（id 为内部标识，username 为登录名与显示名）。
func (s *Store) CreateUser(ctx context.Context, id, username, passwordHash string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO users(id, username, password_hash, persona_id, created_at) VALUES($1, $2, $3, NULL, $4)`,
		id, username, passwordHash, time.Now().Unix())
	return err
}

// GetUserByUsername 按用户名查用户。
func (s *Store) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	var u User
	err := s.pool.QueryRow(ctx,
		`SELECT id, username, password_hash, COALESCE(persona_id, '') FROM users WHERE username = $1`,
		username).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.PersonaID)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// GetUserByID 按内部 ID 查用户。
func (s *Store) GetUserByID(ctx context.Context, id string) (*User, error) {
	var u User
	err := s.pool.QueryRow(ctx,
		`SELECT id, username, password_hash, COALESCE(persona_id, '') FROM users WHERE id = $1`,
		id).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.PersonaID)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// SetPersona 设置用户当前伙伴。
func (s *Store) SetPersona(ctx context.Context, userID, personaID string) error {
	_, err := s.pool.Exec(ctx, `UPDATE users SET persona_id = $1 WHERE id = $2`, personaID, userID)
	return err
}

// CustomPersona 是用户自定义的人设（原始记录）。盲盒人设也复用此结构，Hidden 标记身份隐藏。
type CustomPersona struct {
	ID          string `json:"id"`
	UserID      string `json:"-"`
	Name        string `json:"name"`
	Avatar      string `json:"avatar"`
	Personality string `json:"personality"` // 性格选项 ID，或自定义性格全文
	Style       string `json:"-"`
	Accent      string `json:"-"`
	Tagline     string `json:"-"`
	Opening     string `json:"-"`
	Scene       string `json:"-"`
	Hidden      bool   `json:"-"`
	Turns       int64  `json:"-"`
}

// CreatePersona 创建自定义/盲盒人设。
func (s *Store) CreatePersona(ctx context.Context, c CustomPersona) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO personas(id, user_id, name, avatar, personality, style, accent, tagline, opening, scene, hidden, created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		c.ID, c.UserID, c.Name, c.Avatar, c.Personality, c.Style, c.Accent, c.Tagline, c.Opening, c.Scene, c.Hidden, time.Now().Unix())
	return err
}

// ListPersonas 返回用户的自定义/盲盒人设。
func (s *Store) ListPersonas(ctx context.Context, userID string) ([]CustomPersona, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, user_id, name, avatar, personality, style, accent, tagline, opening, scene, hidden FROM personas WHERE user_id = $1 ORDER BY created_at ASC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []CustomPersona{}
	for rows.Next() {
		var p CustomPersona
		if err := rows.Scan(&p.ID, &p.UserID, &p.Name, &p.Avatar, &p.Personality, &p.Style, &p.Accent, &p.Tagline, &p.Opening, &p.Scene, &p.Hidden); err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, rows.Err()
}

// GetPersona 按 ID 查自定义/盲盒人设。
func (s *Store) GetPersona(ctx context.Context, id string) (*CustomPersona, error) {
	var p CustomPersona
	err := s.pool.QueryRow(ctx,
		`SELECT id, user_id, name, avatar, personality, style, accent, tagline, opening, scene, hidden FROM personas WHERE id = $1`, id).
		Scan(&p.ID, &p.UserID, &p.Name, &p.Avatar, &p.Personality, &p.Style, &p.Accent, &p.Tagline, &p.Opening, &p.Scene, &p.Hidden)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// DeletePersona 删除自定义人设。
func (s *Store) DeletePersona(ctx context.Context, userID, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM personas WHERE id = $1 AND user_id = $2`, id, userID)
	return err
}

// CreateAuthToken 记录一个会话令牌的哈希（只存哈希，不存明文）。
func (s *Store) CreateAuthToken(ctx context.Context, tokenHash, userID string, expiresAt int64) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO auth_tokens(token_hash, user_id, created_at, expires_at) VALUES($1, $2, $3, $4)`,
		tokenHash, userID, time.Now().Unix(), expiresAt)
	return err
}

// GetUserIDByToken 按令牌哈希查用户（过期或不存在返回空串）。
func (s *Store) GetUserIDByToken(ctx context.Context, tokenHash string, now int64) (string, error) {
	var userID string
	err := s.pool.QueryRow(ctx,
		`SELECT user_id FROM auth_tokens WHERE token_hash = $1 AND expires_at > $2`,
		tokenHash, now).Scan(&userID)
	if err != nil {
		return "", err
	}
	return userID, nil
}

// DeleteAuthToken 删除会话令牌（登出）。
func (s *Store) DeleteAuthToken(ctx context.Context, tokenHash string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM auth_tokens WHERE token_hash = $1`, tokenHash)
	return err
}

// DiaryEntry 是一条 AI 日记。
type DiaryEntry struct {
	ID        int64  `json:"id"`
	Content   string `json:"content"`
	CreatedAt int64  `json:"created_at"`
}

// AddDiaryEntry 写入一条日记（关联到某个伙伴）。
func (s *Store) AddDiaryEntry(ctx context.Context, userID, personaID, content string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO diary(user_id, persona_id, content, created_at) VALUES($1, $2, $3, $4)`,
		userID, personaID, content, time.Now().Unix())
	return err
}

// ListDiaryEntries 返回用户与某伙伴的日记（按时间倒序）。
func (s *Store) ListDiaryEntries(ctx context.Context, userID, personaID string) ([]DiaryEntry, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, content, created_at FROM diary WHERE user_id = $1 AND persona_id = $2 ORDER BY id DESC`, userID, personaID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entries := []DiaryEntry{}
	for rows.Next() {
		var e DiaryEntry
		if err := rows.Scan(&e.ID, &e.Content, &e.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// CreateSession 创建聊天会话（关联到某个伙伴）。
func (s *Store) CreateSession(ctx context.Context, userID, personaID, id string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO chat_sessions(id, user_id, persona_id, title, created_at) VALUES($1, $2, $3, '', $4)`,
		id, userID, personaID, time.Now().Unix())
	return err
}

// AddMessage 写入一条消息。
func (s *Store) AddMessage(ctx context.Context, sessionID, userID, role, content string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO messages(session_id, user_id, role, content, created_at) VALUES($1, $2, $3, $4, $5)`,
		sessionID, userID, role, content, time.Now().Unix())
	return err
}

// Message 是一条消息。
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// RecentMessages 返回指定会话最近 limit 条消息（按时间升序）。
func (s *Store) RecentMessages(ctx context.Context, sessionID string, limit int) ([]Message, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT role, content FROM (
			SELECT role, content, id FROM messages WHERE session_id = $1 ORDER BY id DESC LIMIT $2
		) sub ORDER BY id ASC`, sessionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	msgs := []Message{}
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.Role, &m.Content); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

// LatestSession 返回用户与某伙伴最近的聊天会话 ID。
func (s *Store) LatestSession(ctx context.Context, userID, personaID string) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx,
		`SELECT id FROM chat_sessions WHERE user_id = $1 AND persona_id = $2 ORDER BY created_at DESC, id DESC LIMIT 1`,
		userID, personaID).Scan(&id)
	if err != nil {
		return "", nil // 无会话
	}
	return id, nil
}

// ClearPersona 清空用户当前伙伴（换伙伴）。
func (s *Store) ClearPersona(ctx context.Context, userID string) error {
	_, err := s.pool.Exec(ctx, `UPDATE users SET persona_id = NULL WHERE id = $1`, userID)
	return err
}

// IncrementPersonaTurns 累加某个人设的交互轮数（盲盒揭示进度用）。
func (s *Store) IncrementPersonaTurns(ctx context.Context, personaID string) error {
	_, err := s.pool.Exec(ctx, `UPDATE personas SET turns = turns + 1 WHERE id = $1`, personaID)
	return err
}

// ListScopes 返回所有需要后台处理的 (userID, personaID) 组合。
// 覆盖「用户当前选择的伙伴」+「所有自定义/盲盒伙伴」。
func (s *Store) ListScopes(ctx context.Context) ([][2]string, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, persona_id FROM users WHERE persona_id IS NOT NULL AND persona_id <> ''
		 UNION
		 SELECT user_id, id FROM personas`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var scopes [][2]string
	for rows.Next() {
		var uid, pid string
		if err := rows.Scan(&uid, &pid); err != nil {
			return nil, err
		}
		scopes = append(scopes, [2]string{uid, pid})
	}
	return scopes, rows.Err()
}

// ClearPersonaDiary 删除用户与某伙伴的全部日记。
func (s *Store) ClearPersonaDiary(ctx context.Context, userID, personaID string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM diary WHERE user_id = $1 AND persona_id = $2`, userID, personaID)
	return err
}

// ClearPersonaMessages 删除用户与某伙伴的全部消息与会话。
func (s *Store) ClearPersonaMessages(ctx context.Context, userID, personaID string) error {
	if _, err := s.pool.Exec(ctx,
		`DELETE FROM messages WHERE session_id IN (SELECT id FROM chat_sessions WHERE user_id = $1 AND persona_id = $2)`,
		userID, personaID); err != nil {
		return err
	}
	_, err := s.pool.Exec(ctx, `DELETE FROM chat_sessions WHERE user_id = $1 AND persona_id = $2`, userID, personaID)
	return err
}
