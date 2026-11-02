# NiceToMeetU · 个性化 AI 聊天伙伴

一个基于「**反向 RAG**」的个性化 AI 聊天伙伴：向量数据库**初始为空**，模拟陌生感；随着你与 AI 的对话，系统不断从对话中提取你的画像写入向量库，让 AI 在后续互动里越来越「熟悉」你——记得你的称呼、兴趣与近况，用更贴合你的语气说话。

> 和大多数「写死一个 system prompt」的 AI 陪伴不同，这里的 AI 记忆是**真正随对话长出来的**，而且你能看见、修改、删除它。

## 特性

- **反向 RAG**：向量库从空开始，AI 越聊越懂你。
- **按伙伴隔离**：每个伙伴有独立的画像、对话、日记——像一个个真正独立的「人」。
- **盲盒伙伴**：抽一个你一无所知的神秘角色，从一场「初次邂逅」开始，聊得越多 TA 越愿意让你认识（名字 → 头像 → 性格逐层解锁）。
- **自定义伙伴**：上传头像、起名、选性格，创建专属伙伴。
- **记忆面板**：查看 AI 记住了你什么，可**编辑 / 删除**；还有关系等级、了解度覆盖图、词云。
- **日记**：AI 以第一人称写关于你的日记（按伙伴隔离）。
- **引用回复**：右键（移动端左滑）气泡可引用/复制，回复时带上引用上下文。
- **关系里程碑**：了解度跨过阈值时庆祝，从「陌生人」到「知己」。
- **主动闪回**：静默一段时间后，AI 会主动想起你聊过的事来关心你。
- **多主题**：8 套主题，各有独立的字体、边框、背景纹理与动效。
- **多用户 + 鉴权**：注册/登录，bcrypt 密码 + 令牌（服务端只存 SHA-256 摘要）。

## 技术栈

| 层 | 选型 |
|---|---|
| 后端 | Go + chi |
| 对话 LLM | DeepSeek `deepseek-chat` |
| 向量化 | 智谱 GLM `embedding-3`（1024 维） |
| 向量库 | Milvus（画像按 `user_id::persona_id` 隔离） |
| 存储 | PostgreSQL（用户 / 会话 / 消息 / 日记 / 自定义人设） |
| 前端 | React + Vite + TypeScript（多主题） |

## 目录结构

```
backend/          Go 后端
  cmd/server/     入口
  internal/       config / auth / llm / embedding / extractor /
                  retriever / vectorstore / memory / persona /
                  prompt / store / httpapi
frontend/         React 前端（多主题聊天界面）
docker-compose.yml   Milvus + PostgreSQL 一键启动
.env.example      环境变量模板
```

## 快速开始

### 0. 准备 Milvus 与 PostgreSQL

```bash
docker compose up -d
```

- Milvus：`localhost:19530`
- PostgreSQL：`localhost:5433`（用户 `ntmu` / 密码 `ntmupass` / 库 `ntmu`）

### 1. 配置密钥

```bash
cd backend
cp .env.example .env   # 填入 DEEPSEEK_API_KEY、ZHIPU_API_KEY
```

### 2. 启动后端

```bash
go run ./cmd/server
```

### 3. 启动前端

```bash
cd frontend
npm install
npm run dev
```

浏览器打开 `http://localhost:5173`，注册一个账号，选一个伙伴开始对话。

## 核心机制

- **反向 RAG**：`extractor` 在每轮对话后用 LLM 提取用户画像（`basic / interest / personality / emotion / episodic` 五类），向量化后写入 Milvus，并对近似重复做去重。
- **三层记忆**：短期（近期消息）、长期（Milvus 画像）、日志（PostgreSQL 全量消息）。
- **按伙伴隔离**：画像、会话、日记都以 `user_id + persona_id` 为作用域，互不相通。
- **并发架构**：画像提取走有界 worker 池（聊天不阻塞）、批量向量化、五维并发检索、后台定时整合。
- **适度主动**：系统提示词引导 AI 在自然时机询问，并针对认知空白补足。
- **矛盾检测**：用户改口时（如「我换工作了」），AI 自动更新旧画像，记忆保持一致。
- **记忆衰减**：画像按「重要度 × 时效」标注强度（强/中/淡），过期事件型画像后台自动清理。
- **了解度**：界面「已了解你 N 件事」随画像积累实时增长（SSE 推送）。
- **情绪感知**：AI 留意用户当前情绪，先共情再回应。

## API 一览

除注册/登录外，所有接口通过请求头 `Authorization: Bearer <token>` 鉴权。

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/api/auth/register` | 注册 |
| POST | `/api/auth/login` | 登录 |
| POST | `/api/auth/logout` | 登出 |
| GET | `/api/personas` | 列出预设 + 自定义/盲盒伙伴 |
| GET | `/api/personalities` | 列出可选性格模板 |
| POST | `/api/personas/custom` | 创建自定义伙伴（multipart 头像） |
| POST | `/api/personas/blindbox` | 抽盲盒伙伴 |
| DELETE | `/api/personas/{id}` | 删除自定义/盲盒伙伴 |
| GET | `/api/state` | 当前用户状态（人设 / 了解度 / 会话 / 消息） |
| POST | `/api/persona` | 选择/切换伙伴（恢复该伙伴的会话） |
| POST | `/api/persona/clear` | 清空当前伙伴（换伙伴） |
| POST | `/api/chat` | 聊天（SSE 流式，支持引用） |
| POST | `/api/reset` | 重新认识（清空当前伙伴的记忆/对话/日记） |
| GET | `/api/memory` | 列出当前伙伴的画像 |
| DELETE | `/api/memory/{id}` | 删除一条画像 |
| POST | `/api/memory/correct` | 纠错一条画像 |
| GET | `/api/memory/digest` | 生成画像摘要 |
| POST | `/api/flashback` | 主动回忆（闪回） |
| GET | `/api/diary` | 列出当前伙伴的日记 |
| POST | `/api/diary` | 写一篇今天的日记 |
| GET | `/api/events` | 实时事件流（SSE） |

## License

[MIT](./LICENSE)
