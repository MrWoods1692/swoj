// swoj 是 Online Judge 服务主入口：初始化数据库与测评队列，然后启动 HTTP 服务。
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/swoj/swoj/internal/app"
)

func main() {
	cfg := app.LoadConfig()

	db, err := app.OpenDB(filepath.Join(cfg.DataDir, "db"), cfg)
	if err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}
	defer db.Close()

	judgeDir := filepath.Join(cfg.DataDir, "judge")
	if err := os.MkdirAll(judgeDir, 0o755); err != nil {
		log.Fatalf("初始化测评目录失败: %v", err)
	}
	defer os.RemoveAll(judgeDir)

	srv := app.NewServer(cfg, db, judgeDir)
	defer srv.Queue.Shutdown(context.Background())

	handler := app.NewRouter(srv)
	if app.DirExists(cfg.StaticDir) {
		handler = staticHandler(handler, cfg.StaticDir)
	}

	httpSrv := http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("Swoj 已启动，监听 %s", cfg.ListenAddr)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP 服务异常: %v", err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shutdownCtx)
	log.Println("服务已停止")
}

// staticHandler 提供前端构建产物，并对前端路由做 index.html 回退。
// 跳过 /api/ 前缀，避免把接口请求误判为页面。
func staticHandler(api http.Handler, dir string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			api.ServeHTTP(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			api.ServeHTTP(w, r)
			return
		}
		p := filepath.Join(dir, filepath.Clean(strings.TrimPrefix(r.URL.Path, "/")))
		if _, err := os.Stat(p); err != nil {
			p = filepath.Join(dir, "index.html")
		}
		http.ServeFile(w, r, p)
	})
}
