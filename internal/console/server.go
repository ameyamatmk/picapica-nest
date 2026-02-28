// Package console は Web コンソールの HTTP サーバーを提供する。
// 蒸留レポートと Usage の閲覧 UI を HTMX + Pico CSS で構築する。
package console

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"math"
	"net/http"
	"time"

	"github.com/ameyamatmk/picapica-nest/internal/channellabel"
	"github.com/ameyamatmk/picapica-nest/internal/pricing"
)

// Port は Web コンソールのリスンポート。
const Port = 19100

//go:embed static
var staticFS embed.FS

//go:embed templates
var templateFS embed.FS

// Server は Web コンソールの HTTP サーバー。
type Server struct {
	server        *http.Server
	workspacePath string
	resolver      *channellabel.Resolver
	pricer        *pricing.Pricer

	// ページごとにテンプレートセットを保持する。
	// "content" 定義の衝突を避けるため、layout + ページ固有テンプレートを組み合わせる。
	dashboardTmpl     *template.Template
	distillTmpl       *template.Template
	conversationsTmpl *template.Template
	workspaceTmpl     *template.Template
	usageTmpl         *template.Template
	applogTmpl        *template.Template
}

// ServerOption は NewServer に渡すオプション関数。
type ServerOption func(*Server)

// WithResolver はチャンネルラベル解決用の Resolver を設定する。
func WithResolver(r *channellabel.Resolver) ServerOption {
	return func(s *Server) { s.resolver = r }
}

// WithPricer はコスト計算用の Pricer を設定する。
func WithPricer(p *pricing.Pricer) ServerOption {
	return func(s *Server) { s.pricer = p }
}

// NewServer は新しい Web Console サーバーを作成する。
// workspacePath は PicoClaw ワークスペースのパス。
func NewServer(workspacePath string, opts ...ServerOption) *Server {
	s := &Server{
		workspacePath: workspacePath,
	}
	for _, opt := range opts {
		opt(s)
	}

	funcMap := template.FuncMap{
		"comma":     formatComma,
		"formatUSD": formatUSD,
		"formatJPY": formatJPY,
	}

	s.dashboardTmpl = template.Must(
		template.New("").Funcs(funcMap).ParseFS(templateFS,
			"templates/layout.html",
			"templates/dashboard.html",
		),
	)
	s.distillTmpl = template.Must(
		template.New("").Funcs(funcMap).ParseFS(templateFS,
			"templates/layout.html",
			"templates/distill.html",
			"templates/distill_content.html",
		),
	)
	s.conversationsTmpl = template.Must(
		template.New("").Funcs(funcMap).ParseFS(templateFS,
			"templates/layout.html",
			"templates/conversations.html",
			"templates/conversations_content.html",
		),
	)
	s.workspaceTmpl = template.Must(
		template.New("").Funcs(funcMap).ParseFS(templateFS,
			"templates/layout.html",
			"templates/workspace.html",
			"templates/workspace_content.html",
		),
	)
	s.usageTmpl = template.Must(
		template.New("").Funcs(funcMap).ParseFS(templateFS,
			"templates/layout.html",
			"templates/usage.html",
		),
	)
	s.applogTmpl = template.Must(
		template.New("").Funcs(funcMap).ParseFS(templateFS,
			"templates/layout.html",
			"templates/applog.html",
			"templates/applog_content.html",
		),
	)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.handleIndex)
	mux.HandleFunc("GET /dashboard", s.handleDashboard)
	mux.HandleFunc("GET /distill", s.handleDistill)
	mux.HandleFunc("GET /distill/content", s.handleDistillContent)
	mux.HandleFunc("GET /conversations", s.handleConversations)
	mux.HandleFunc("GET /conversations/messages", s.handleConversationMessages)
	mux.HandleFunc("GET /workspace", s.handleWorkspace)
	mux.HandleFunc("GET /workspace/content", s.handleWorkspaceContent)
	mux.HandleFunc("GET /usage", s.handleUsage)
	mux.HandleFunc("GET /logs", s.handleAppLog)
	mux.HandleFunc("GET /logs/entries", s.handleAppLogEntries)

	staticSub, _ := fs.Sub(staticFS, "static")
	mux.Handle("GET /static/", http.StripPrefix("/static/", cacheControl(http.FileServerFS(staticSub))))

	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", Port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	return s
}

// Start はサーバーを起動する。ブロックする。
func (s *Server) Start() error {
	slog.Info("console server starting", "component", "console", "port", Port)
	return s.server.ListenAndServe()
}

// Stop はサーバーをグレースフルに停止する。
func (s *Server) Stop(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// handleIndex は / へのアクセスを /distill にリダイレクトする。
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/dashboard", http.StatusFound)
}

// pageData はテンプレートに渡す共通データ。
type pageData struct {
	Title  string
	Active string
}

// cacheControl は静的ファイル配信に Cache-Control ヘッダーを付与するミドルウェア。
// embed されたファイルはビルド時に固定されるため、長期キャッシュが安全に使える。
func cacheControl(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=86400")
		next.ServeHTTP(w, r)
	})
}

// formatUSD は USD 金額を "$0.0123" 形式にフォーマットする。
func formatUSD(v float64) string {
	return fmt.Sprintf("$%.4f", v)
}

// formatJPY は JPY 金額を "¥123" 形式にフォーマットする。
// nil の場合は "-" を返す。
func formatJPY(v *float64) string {
	if v == nil {
		return "-"
	}
	return fmt.Sprintf("¥%s", formatComma(int64(math.Round(*v))))
}

// formatComma は数値をコンマ区切りの文字列に変換する。
func formatComma(n any) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var result []byte
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}
	return string(result)
}
