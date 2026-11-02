package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"nicetomeetu/internal/auth"
	"nicetomeetu/internal/config"
	"nicetomeetu/internal/llm"
	"nicetomeetu/internal/memory"
	"nicetomeetu/internal/persona"
	"nicetomeetu/internal/prompt"
	"nicetomeetu/internal/store"
)

type ctxKey string

const userIDKey ctxKey = "userID"

// tokenTTL 是会话令牌有效期（30 天）。
const tokenTTL = 30 * 24 * time.Hour

// userID 从鉴权中间件注入的上下文读取当前用户 ID。
func userID(r *http.Request) string {
	id, _ := r.Context().Value(userIDKey).(string)
	return id
}

// currentScope 返回当前用户当前伙伴下的画像作用域（用于按伙伴隔离记忆）。
func (h *Handler) currentScope(ctx context.Context, uid string) string {
	u, err := h.db.GetUserByID(ctx, uid)
	if err != nil || u.PersonaID == "" {
		return uid
	}
	return memory.ScopeKey(uid, u.PersonaID)
}

type Handler struct {
	cfg *config.Config
	db  *store.Store
	svc *memory.Service
	llm *llm.Client
}

func New(cfg *config.Config, db *store.Store, svc *memory.Service, l *llm.Client) *Handler {
	return &Handler{cfg: cfg, db: db, svc: svc, llm: l}
}

func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(cors)
	r.Use(loggingMiddleware)

	// 上传的头像静态服务
	r.Handle("/uploads/*", http.StripPrefix("/uploads/", http.FileServer(http.Dir(h.cfg.UploadDir))))

	// 公开路由
	r.Post("/api/auth/register", h.register)
	r.Post("/api/auth/login", h.login)

	// 需要鉴权的路由
	r.Group(func(r chi.Router) {
		r.Use(h.authMiddleware)
		r.Get("/api/personas", h.listPersonas)
		r.Get("/api/personalities", h.listPersonalities)
		r.Post("/api/personas/custom", h.createCustomPersona)
		r.Post("/api/personas/blindbox", h.drawBlindBox)
		r.Delete("/api/personas/{id}", h.deletePersona)
		r.Get("/api/state", h.state)
		r.Post("/api/persona", h.selectPersona)
		r.Post("/api/persona/clear", h.clearPersona)
		r.Post("/api/chat", h.chat)
		r.Post("/api/reset", h.reset)
		r.Get("/api/memory", h.listMemory)
		r.Delete("/api/memory/{id}", h.deleteFact)
		r.Post("/api/memory/correct", h.correctFact)
		r.Get("/api/memory/digest", h.memoryDigest)
		r.Post("/api/flashback", h.flashback)
		r.Get("/api/diary", h.listDiary)
		r.Post("/api/diary", h.writeDiary)
		r.Post("/api/auth/logout", h.logout)
		r.Get("/api/events", h.events)
	})
	return r
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// Flush 转发到底层 ResponseWriter（若支持），保持 SSE/流式可用。
func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// loggingMiddleware 记录每个请求的方法、路径、状态码与耗时。
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		log.Printf("%s %s user=%s status=%d latency=%s",
			r.Method, r.URL.Path, userID(r), rec.status, time.Since(start).Round(time.Millisecond))
	})
}

// authMiddleware 校验 Bearer 令牌并把用户 ID 注入上下文。
func (h *Handler) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := extractToken(r)
		if token == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}
		uid, err := h.db.GetUserIDByToken(r.Context(), auth.HashToken(token), time.Now().Unix())
		if err != nil || uid == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "登录已过期，请重新登录"})
			return
		}
		ctx := context.WithValue(r.Context(), userIDKey, uid)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func extractToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// ---------- 鉴权 ----------

type authReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var body authReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	username := strings.TrimSpace(body.Username)
	if username == "" || len(body.Password) < 6 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "用户名不能为空，密码至少 6 位"})
		return
	}

	// 用户名唯一性检查
	if _, err := h.db.GetUserByUsername(r.Context(), username); err == nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "该用户名已被注册"})
		return
	}

	hash, err := auth.HashPassword(body.Password)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	id := genID()
	if err := h.db.CreateUser(r.Context(), id, username, hash); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	h.issueToken(w, r, id, username)
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var body authReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	username := strings.TrimSpace(body.Username)
	u, err := h.db.GetUserByUsername(r.Context(), username)
	if err != nil || !auth.CheckPassword(body.Password, u.PasswordHash) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "用户名或密码错误"})
		return
	}
	h.issueToken(w, r, u.ID, u.Username)
}

// issueToken 生成并记录会话令牌，返回给客户端。
func (h *Handler) issueToken(w http.ResponseWriter, r *http.Request, userID, username string) {
	token, err := auth.GenerateToken()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	expiresAt := time.Now().Add(tokenTTL).Unix()
	if err := h.db.CreateAuthToken(r.Context(), auth.HashToken(token), userID, expiresAt); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"token": token, "user_id": userID, "username": username})
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	token := extractToken(r)
	if token != "" {
		_ = h.db.DeleteAuthToken(r.Context(), auth.HashToken(token))
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ---------- 业务 ----------

func (h *Handler) listPersonas(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	custom, err := h.db.ListPersonas(ctx, userID(r))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	all := make([]persona.Persona, 0, len(persona.All)+len(custom))
	all = append(all, persona.All...)
	for _, c := range custom {
		all = append(all, h.customToPersona(c))
	}
	writeJSON(w, http.StatusOK, map[string]any{"personas": all})
}

// customToPersona 把自定义/盲盒人设原始记录解析成完整 Persona。
// 盲盒人设（Hidden=true）全字段已落库，直接还原；
// 自定义人设的 personality 字段可能是预设性格 ID，也可能是自由填写的性格描述。
func (h *Handler) customToPersona(c store.CustomPersona) persona.Persona {
	if c.Hidden {
		return persona.Persona{
			ID:          c.ID,
			Name:        c.Name,
			Avatar:      c.Avatar,
			Tagline:     c.Tagline,
			Personality: c.Personality,
			Style:       c.Style,
			Opening:     c.Opening,
			Scene:       c.Scene,
			Accent:      c.Accent,
			Hidden:      true,
			Turns:       c.Turns,
		}
	}

	p := persona.Persona{
		ID:      c.ID,
		Name:    c.Name,
		Avatar:  c.Avatar,
		Opening: "你好，我是" + c.Name + "，很高兴认识你！",
	}
	if opt, ok := persona.GetPersonality(c.Personality); ok {
		p.Tagline = opt.Label + " · 自定义"
		p.Personality = opt.Personality
		p.Style = opt.Style
		p.Accent = opt.Accent
	} else {
		p.Tagline = "自定义性格"
		p.Personality = c.Personality
		p.Style = "自然地表达、真诚地回应，始终符合你设定的性格。"
		p.Accent = "#8b5cf6"
	}
	return p
}

// resolvePersona 先查预设，再查自定义人设。
func (h *Handler) resolvePersona(ctx context.Context, uid, personaID string) (persona.Persona, bool) {
	if p, ok := persona.Get(personaID); ok {
		return p, true
	}
	c, err := h.db.GetPersona(ctx, personaID)
	if err != nil || c.UserID != uid {
		return persona.Persona{}, false
	}
	return h.customToPersona(*c), true
}

func (h *Handler) listPersonalities(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"personalities": persona.Personalities})
}

func (h *Handler) createCustomPersona(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(16 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "上传失败"})
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	personalityID := r.FormValue("personality")
	if name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "名字不能为空"})
		return
	}
	if personalityID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "请选择或填写性格"})
		return
	}
	file, header, err := r.FormFile("avatar")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "请上传头像"})
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext == "" || len(ext) > 5 {
		ext = ".png"
	}
	filename := genID() + ext
	if err := os.WriteFile(filepath.Join(h.cfg.UploadDir, filename), data, 0o644); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	avatarPath := "/uploads/" + filename
	id := "custom_" + genID()[:8]
	c := store.CustomPersona{ID: id, UserID: userID(r), Name: name, Avatar: avatarPath, Personality: personalityID}
	if err := h.db.CreatePersona(r.Context(), c); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"persona": h.customToPersona(c)})
}

// drawBlindBox 随机抽一个隐藏角色与一个邂逅场景，创建一个盲盒伙伴。
func (h *Handler) drawBlindBox(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	char := persona.RandomBlindBox()

	id := "blindbox_" + genID()[:8]
	c := store.CustomPersona{
		ID:          id,
		UserID:      userID(r),
		Name:        char.Name,
		Avatar:      char.Avatar,
		Personality: char.Personality,
		Style:       char.Style,
		Accent:      char.Accent,
		Tagline:     char.Tagline,
		Opening:     char.Opening,
		Scene:       char.Scene,
		Hidden:      true,
	}
	if err := h.db.CreatePersona(ctx, c); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	p := h.customToPersona(c)

	if err := h.db.SetPersona(ctx, userID(r), p.ID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	sessionID := genID()
	if err := h.db.CreateSession(ctx, userID(r), p.ID, sessionID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if err := h.db.AddMessage(ctx, sessionID, userID(r), "assistant", p.Opening); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"session_id": sessionID,
		"opening":    p.Opening,
		"persona":    p,
	})
}

// deletePersona 删除用户的自定义/盲盒人设（通用删除）。
func (h *Handler) deletePersona(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	ctx := r.Context()
	p, err := h.db.GetPersona(ctx, id)
	if err != nil || p.UserID != userID(r) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "persona not found"})
		return
	}
	if err := h.db.DeletePersona(ctx, userID(r), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	// 清理上传的头像文件（仅 /uploads/ 下的用户上传头像，不碰共享的 /characters/ 等）
	if strings.HasPrefix(p.Avatar, "/uploads/") {
		_ = os.Remove(filepath.Join(h.cfg.UploadDir, strings.TrimPrefix(p.Avatar, "/uploads/")))
	}
	// 若删的是当前伙伴，清空引用
	if u, err := h.db.GetUserByID(ctx, userID(r)); err == nil && u.PersonaID == id {
		_ = h.db.SetPersona(ctx, userID(r), "")
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

type stateResp struct {
	PersonaID   string          `json:"persona_id,omitempty"`
	Persona     *persona.Persona `json:"persona"`
	Familiarity int             `json:"familiarity"`
	SessionID   string          `json:"session_id,omitempty"`
	Messages    []store.Message `json:"messages"`
}

func (h *Handler) state(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u, err := h.db.GetUserByID(ctx, userID(r))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	resp := stateResp{Messages: []store.Message{}}
	if u.PersonaID == "" {
		writeJSON(w, http.StatusOK, resp)
		return
	}
	p, ok := h.resolvePersona(ctx, userID(r), u.PersonaID)
	if !ok {
		writeJSON(w, http.StatusOK, resp)
		return
	}
	resp.PersonaID = u.PersonaID
	resp.Persona = &p

	fam, err := h.svc.Familiarity(ctx, h.currentScope(ctx, userID(r)))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	resp.Familiarity = fam

	sessionID, msgs, err := h.latestSession(ctx, userID(r), u.PersonaID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	resp.SessionID = sessionID
	resp.Messages = msgs
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) selectPersona(w http.ResponseWriter, r *http.Request) {
	var body struct {
		PersonaID string `json:"persona_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	p, ok := h.resolvePersona(r.Context(), userID(r), body.PersonaID)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown persona"})
		return
	}
	ctx := r.Context()
	if err := h.db.SetPersona(ctx, userID(r), p.ID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	// 恢复该伙伴的最近会话；没有则新建
	sessionID, err := h.db.LatestSession(ctx, userID(r), p.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if sessionID == "" {
		sessionID = genID()
		if err := h.db.CreateSession(ctx, userID(r), p.ID, sessionID); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if err := h.db.AddMessage(ctx, sessionID, userID(r), "assistant", p.Opening); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}
	msgs, _ := h.db.RecentMessages(ctx, sessionID, 200)
	writeJSON(w, http.StatusOK, map[string]any{
		"session_id": sessionID,
		"opening":    p.Opening,
		"persona":    p,
		"messages":   msgs,
	})
}

// clearPersona 清空当前伙伴（换伙伴），持久化到数据库。
func (h *Handler) clearPersona(w http.ResponseWriter, r *http.Request) {
	if err := h.db.ClearPersona(r.Context(), userID(r)); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

type chatReq struct {
	SessionID string `json:"session_id"`
	Message   string `json:"message"`
	Quote     string `json:"quote,omitempty"` // 被引用的原文（作为 AI 上下文）
}

func (h *Handler) chat(w http.ResponseWriter, r *http.Request) {
	var body chatReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Message == "" || body.SessionID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}

	ctx := r.Context()
	u, err := h.db.GetUserByID(ctx, userID(r))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	p, ok := h.resolvePersona(ctx, userID(r), u.PersonaID)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "please select a persona first"})
		return
	}

	// 1. 检索长期画像
	facts, err := h.svc.RetrieveProfile(ctx, h.currentScope(ctx, userID(r)), body.Message, 6)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	pf := make([]prompt.ProfileFact, 0, len(facts))
	for _, f := range facts {
		pf = append(pf, prompt.ProfileFact{Type: f.Type, Content: f.Content})
	}

	// 2. 短期记忆
	history, err := h.svc.RecentMessages(ctx, body.SessionID, 12)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// 3. 组装消息（含认知空白引导）；带引用时把引用拼进发给 LLM 的用户消息，作为上下文
	userMsg := body.Message
	if body.Quote != "" {
		userMsg = "【引用】" + body.Quote + "\n\n" + body.Message
	}
	emptyTypes, _ := h.svc.EmptyDimensions(ctx, h.currentScope(ctx, userID(r)))
	system := prompt.BuildSystem(p, pf, emptyTypes)
	msgs := []llm.Message{{Role: "system", Content: system}}
	for _, m := range history {
		msgs = append(msgs, llm.Message{Role: m.Role, Content: m.Content})
	}
	msgs = append(msgs, llm.Message{Role: "user", Content: userMsg})

	// 4. 落库用户消息（带引用，保留上下文）
	if err := h.db.AddMessage(ctx, body.SessionID, userID(r), "user", userMsg); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	// 累加交互轮数（盲盒揭示进度；预设人设无记录，为 no-op）
	_ = h.db.IncrementPersonaTurns(ctx, u.PersonaID)

	// 5. 提前投递画像提取（与流式回复并行）；只用用户自己的话，避免把被引用的 AI 原文当成用户信息
	h.svc.RememberAsync(h.currentScope(ctx, userID(r)), recentUserMessages(history, body.Message))

	// 6. 流式输出
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "streaming unsupported"})
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	var full strings.Builder
	for ev := range h.llm.ChatStream(ctx, msgs) {
		if ev.Err != nil {
			_ = writeSSE(w, flusher, map[string]string{"error": ev.Err.Error()})
			return
		}
		if ev.Done {
			break
		}
		full.WriteString(ev.Delta)
		_ = writeSSE(w, flusher, map[string]string{"delta": ev.Delta})
	}
	_ = writeSSE(w, flusher, map[string]string{"done": "true"})

	// 7. 落库助手消息
	reply := full.String()
	if reply != "" {
		_ = h.db.AddMessage(ctx, body.SessionID, userID(r), "assistant", reply)
	}
}

func (h *Handler) reset(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u, err := h.db.GetUserByID(ctx, userID(r))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if err := h.svc.Reset(ctx, h.currentScope(ctx, userID(r))); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	p, ok := h.resolvePersona(ctx, userID(r), u.PersonaID)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no persona selected"})
		return
	}
	sessionID := genID()
	if err := h.db.CreateSession(ctx, userID(r), p.ID, sessionID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if err := h.db.AddMessage(ctx, sessionID, userID(r), "assistant", p.Opening); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"session_id": sessionID,
		"opening":    p.Opening,
	})
}

func (h *Handler) listMemory(w http.ResponseWriter, r *http.Request) {
	facts, err := h.svc.ListFacts(r.Context(), h.currentScope(r.Context(), userID(r)))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"facts": facts})
}

func (h *Handler) deleteFact(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	if err := h.svc.DeleteFact(r.Context(), h.currentScope(r.Context(), userID(r)), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) correctFact(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID      string `json:"id"`
		Type    string `json:"type"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Content == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	if err := h.svc.CorrectFact(r.Context(), h.currentScope(r.Context(), userID(r)), body.ID, body.Type, body.Content); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) memoryDigest(w http.ResponseWriter, r *http.Request) {
	digest, err := h.svc.Digest(r.Context(), h.currentScope(r.Context(), userID(r)))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"digest": digest})
}

func (h *Handler) flashback(w http.ResponseWriter, r *http.Request) {
	var body struct {
		SessionID string `json:"session_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.SessionID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	msg, err := h.svc.Flashback(r.Context(), h.currentScope(r.Context(), userID(r)))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if err := h.db.AddMessage(r.Context(), body.SessionID, userID(r), "assistant", msg); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": msg})
}

func (h *Handler) listDiary(w http.ResponseWriter, r *http.Request) {
	entries, err := h.svc.ListDiary(r.Context(), h.currentScope(r.Context(), userID(r)))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": entries})
}

func (h *Handler) writeDiary(w http.ResponseWriter, r *http.Request) {
	entry, err := h.svc.WriteDiary(r.Context(), h.currentScope(r.Context(), userID(r)))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"entry": entry})
}

func (h *Handler) events(w http.ResponseWriter, r *http.Request) {
	uid := userID(r)
	ch, cancel := h.svc.Subscribe(uid)
	defer cancel()

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "streaming unsupported"})
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	for {
		select {
		case ev := <-ch:
			b, _ := json.Marshal(ev)
			_, _ = fmt.Fprintf(w, "data: %s\n\n", b)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

// latestSession 返回用户与某伙伴最近的会话及其全部消息（用于刷新恢复）。
func (h *Handler) latestSession(ctx context.Context, userID, personaID string) (string, []store.Message, error) {
	sessionID, err := h.db.LatestSession(ctx, userID, personaID)
	if err != nil {
		return "", nil, err
	}
	if sessionID == "" {
		return "", []store.Message{}, nil
	}
	msgs, err := h.db.RecentMessages(ctx, sessionID, 200)
	if err != nil {
		return "", nil, err
	}
	return sessionID, msgs, nil
}

func recentUserMessages(history []store.Message, current string) []string {
	var out []string
	for _, m := range history {
		if m.Role == "user" {
			out = append(out, m.Content)
		}
	}
	out = append(out, current)
	if len(out) > 3 {
		out = out[len(out)-3:]
	}
	return out
}

func writeSSE(w http.ResponseWriter, flusher http.Flusher, data map[string]string) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "data: %s\n\n", b)
	flusher.Flush()
	return err
}

func genID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
