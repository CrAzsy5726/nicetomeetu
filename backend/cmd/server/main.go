package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"nicetomeetu/internal/config"
	"nicetomeetu/internal/embedding"
	"nicetomeetu/internal/extractor"
	"nicetomeetu/internal/httpapi"
	"nicetomeetu/internal/llm"
	"nicetomeetu/internal/memory"
	"nicetomeetu/internal/retriever"
	"nicetomeetu/internal/store"
	"nicetomeetu/internal/vectorstore"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	if cfg.DeepSeekAPIKey == "" {
		log.Println("[warn] DEEPSEEK_API_KEY 未设置，对话与画像提取将无法工作")
	}
	if cfg.ZhipuAPIKey == "" {
		log.Println("[warn] ZHIPU_API_KEY 未设置，向量化将无法工作")
	}

	// 连接超时上下文
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 上传目录
	if err := os.MkdirAll(cfg.UploadDir, 0o755); err != nil {
		log.Fatalf("mkdir upload dir: %v", err)
	}

	// PostgreSQL 存储
	db, err := store.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect postgres: %v", err)
	}
	defer db.Close()
	log.Println("postgres connected")

	// Milvus 向量库
	vs, err := vectorstore.New(ctx, cfg.MilvusAddr, cfg.MilvusCollection, cfg.EmbedDim)
	if err != nil {
		log.Fatalf("connect milvus (%s): %v", cfg.MilvusAddr, err)
	}
	defer vs.Close()
	log.Printf("milvus connected at %s, collection %q ready", cfg.MilvusAddr, cfg.MilvusCollection)

	// 模型客户端
	llmClient := llm.New(cfg.DeepSeekAPIKey, cfg.DeepSeekBase, cfg.DeepSeekModel)
	embClient := embedding.New(cfg.ZhipuAPIKey, cfg.ZhipuBase, cfg.ZhipuModel, cfg.EmbedDim)

	// 记忆核心（内含画像提取 worker 池）
	ret := retriever.New(embClient, vs)
	ex := extractor.New(llmClient, embClient, vs)
	svc := memory.New(vs, db, ret, ex, llmClient, cfg.ExtractWorkers)
	defer svc.Close()

	// 后台整合任务：定期清理过期画像（事件型低重要度事实的衰减）
	bgCtx, bgCancel := context.WithCancel(context.Background())
	defer bgCancel()
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-bgCtx.Done():
				return
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				scopes, err := db.ListScopes(ctx)
				if err != nil {
					log.Printf("[consolidate] list scopes: %v", err)
					cancel()
					continue
				}
				for _, sc := range scopes {
					scope := memory.ScopeKey(sc[0], sc[1])
					if err := svc.Consolidate(ctx, scope); err != nil {
						log.Printf("[consolidate] %s: %v", scope, err)
					}
				}
				cancel()
			}
		}
	}()

	// HTTP
	handler := httpapi.New(cfg, db, svc, llmClient)
	srv := &http.Server{Addr: cfg.ServerAddr, Handler: handler.Routes()}

	go func() {
		log.Printf("server listening on %s", cfg.ServerAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down...")
	bgCancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	_ = srv.Shutdown(shutdownCtx)
}
