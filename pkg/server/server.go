package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/altenwald/backlog/pkg/store"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/mark3labs/mcp-go/server"
)

type Server struct {
	httpServer *http.Server
	sseServer  *server.SSEServer
	mcpServer  *server.MCPServer
	store      *store.Store
	port       int
}

func NewServer(st *store.Store, port int) *Server {
	if port <= 0 {
		port = 8484
	}
	mcpSrv := NewMCPServer(st)
	sseSrv := server.NewSSEServer(mcpSrv, server.WithBaseURL(fmt.Sprintf("http://127.0.0.1:%d", port+1)))
	return &Server{
		store:     st,
		port:      port,
		mcpServer: mcpSrv,
		sseServer: sseSrv,
	}
}

func (s *Server) GetMCPServer() *server.MCPServer {
	return s.mcpServer
}

func (s *Server) Start() error {
	// Start MCP SSE Server on port+1 (e.g. 8485)
	mcpAddr := fmt.Sprintf("127.0.0.1:%d", s.port+1)
	go func() {
		_ = s.sseServer.Start(mcpAddr)
	}()

	// REST API Router
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// REST API Endpoints
	api := NewAPIHandler(s.store)
	api.RegisterRoutes(r)

	// Health & info check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, http.StatusOK, map[string]any{
			"status":         "ok",
			"active_project": s.store.GetActiveProjectSlug(),
			"api_port":       s.port,
			"mcp_sse_port":   s.port + 1,
			"mcp_sse_url":    fmt.Sprintf("http://127.0.0.1:%d/sse", s.port+1),
		})
	})

	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf("127.0.0.1:%d", s.port),
		Handler: r,
	}

	return s.httpServer.ListenAndServe()
}

func (s *Server) Stop(ctx context.Context) error {
	if s.sseServer != nil {
		_ = s.sseServer.Shutdown(ctx)
	}
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}
