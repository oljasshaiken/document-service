package main

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

var (
	appCfg = defaultConfig()
	// pdfGen renders HTML to PDF; overridden in tests.
	pdfGen = func(ctx context.Context, html string) ([]byte, error) {
		return generatePDF(ctx, appCfg, html)
	}
	renderSem chan struct{}
)

func init() {
	initConcurrency(appCfg.MaxConcurrentRenders)
}

func initConcurrency(n int) {
	if n < 1 {
		n = 1
	}
	renderSem = make(chan struct{}, n)
}

func main() {
	cfg, err := loadConfig()
	if err != nil {
		slog.Error("config error", "err", err)
		os.Exit(1)
	}
	appCfg = cfg
	initConcurrency(cfg.MaxConcurrentRenders)

	var logHandler slog.Handler
	if cfg.LogJSON {
		logHandler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	} else {
		logHandler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	}
	slog.SetDefault(slog.New(logHandler))

	srv := newServer(cfg)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		slog.Info("server listening", "addr", cfg.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown error", "err", err)
		os.Exit(1)
	}
	slog.Info("shutdown complete")
}

func newServer(cfg *Config) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthHandler)
	mux.Handle("/metrics", metricsHandler())

	htmlChain := http.Handler(http.HandlerFunc(htmlToPDFHandler))
	xmlChain := http.Handler(http.HandlerFunc(xmlToPDFHandler))

	if cfg.RateLimitRPS > 0 {
		il := newIPLimiter(cfg.RateLimitRPS, cfg.RateLimitBurst)
		htmlChain = rateLimitMiddleware(il, htmlChain)
		xmlChain = rateLimitMiddleware(il, xmlChain)
	}
	htmlChain = apiKeyAuth(cfg, htmlChain)
	xmlChain = apiKeyAuth(cfg, xmlChain)
	htmlChain = instrumentHandler("html_to_pdf", htmlChain)
	xmlChain = instrumentHandler("xml_to_pdf", xmlChain)

	mux.Handle("/html-to-pdf", htmlChain)
	mux.Handle("/xml-to-pdf", xmlChain)

	return &http.Server{
		Addr:              cfg.Addr,
		Handler:           mux,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func htmlToPDFHandler(w http.ResponseWriter, r *http.Request) {
	handleDocumentToPDF(w, r, "html")
}

func xmlToPDFHandler(w http.ResponseWriter, r *http.Request) {
	handleDocumentToPDF(w, r, "xml")
}

func handleDocumentToPDF(w http.ResponseWriter, r *http.Request, format string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := acquireRender(r.Context()); err != nil {
		if errors.Is(err, context.Canceled) {
			http.Error(w, "client disconnected", http.StatusRequestTimeout)
			return
		}
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}
	defer releaseRender()

	defer r.Body.Close()
	r.Body = http.MaxBytesReader(w, r.Body, appCfg.MaxBodyBytes)

	htmlBytes, err := io.ReadAll(r.Body)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	content := string(htmlBytes)
	if content == "" {
		http.Error(w, "empty request body", http.StatusBadRequest)
		return
	}

	pdfSource, err := toPDFSource(content, format)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	pdfBytes, err := pdfGen(r.Context(), pdfSource)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			http.Error(w, "client disconnected", http.StatusRequestTimeout)
			return
		}
		slog.Error("pdf generation error", "err", err)
		http.Error(w, "failed to generate PDF", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="document.pdf"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(pdfBytes)
}

func acquireRender(ctx context.Context) error {
	select {
	case renderSem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func releaseRender() {
	<-renderSem
}

func toPDFSource(content, format string) (string, error) {
	switch format {
	case "html":
		return content, nil
	case "xml":
		if err := validateXML(content); err != nil {
			return "", fmt.Errorf("invalid XML")
		}
		return fmt.Sprintf(
			`<!doctype html><html><head><meta charset="utf-8"></head><body><pre>%s</pre></body></html>`,
			html.EscapeString(content),
		), nil
	default:
		return "", fmt.Errorf("unsupported format")
	}
}

func validateXML(content string) error {
	dec := xml.NewDecoder(strings.NewReader(content))
	dec.Strict = true
	for {
		t, err := dec.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		_ = t
	}
}
