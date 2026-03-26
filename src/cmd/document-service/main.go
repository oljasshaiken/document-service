package main

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

const (
	serverAddr      = ":8080"
	maxHTMLBodySize = 2 << 20 // 2 MiB
	pdfTimeout      = 30 * time.Second
)

var pdfGenerator = generatePDF

func main() {
	server := newServer()
	log.Printf("Server starting on %s...", server.Addr)
	log.Fatal(server.ListenAndServe())
}

func newServer() *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/html-to-pdf", htmlToPDFHandler)
	mux.HandleFunc("/xml-to-pdf", xmlToPDFHandler)
	mux.HandleFunc("/healthz", healthHandler)

	return &http.Server{
		Addr:              serverAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
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

	defer r.Body.Close()
	r.Body = http.MaxBytesReader(w, r.Body, maxHTMLBodySize)

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

	pdfBytes, err := pdfGenerator(pdfSource)
	if err != nil {
		log.Printf("PDF generation error: %v", err)
		http.Error(w, "failed to generate PDF", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="document.pdf"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(pdfBytes)
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
	decoder := xml.NewDecoder(strings.NewReader(content))
	for {
		_, err := decoder.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

// generatePDF converts HTML string to PDF bytes using chromedp
func generatePDF(html string) ([]byte, error) {
	chromePath, err := findChromePath()
	if err != nil {
		return nil, err
	}

	allocOpts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(chromePath),
		chromedp.NoDefaultBrowserCheck,
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), allocOpts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, pdfTimeout)
	defer cancel()

	var pdfBytes []byte

	err = chromedp.Run(ctx,
		// URL-escape HTML to make data URL robust for special characters.
		chromedp.Navigate("data:text/html;charset=utf-8,"+url.PathEscape(html)),
		chromedp.WaitReady("body"), // wait until body is rendered
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			pdfBytes, _, err = page.PrintToPDF().
				WithPrintBackground(true). // respect background colors
				WithMarginTop(0.4).        // cm
				WithMarginBottom(0.4).
				WithMarginLeft(0.4).
				WithMarginRight(0.4).
				WithPaperWidth(8.27).   // A4 width in inches
				WithPaperHeight(11.69). // A4 height
				Do(ctx)
			return err
		}),
	)

	return pdfBytes, err
}

func findChromePath() (string, error) {
	if p := os.Getenv("CHROME_PATH"); p != "" {
		return p, nil
	}

	candidates := []string{
		"chromium-browser",
		"chromium",
		"google-chrome",
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
	}

	for _, name := range candidates {
		p, err := exec.LookPath(name)
		if err == nil {
			return p, nil
		}
	}

	return "", fmt.Errorf("chrome executable not found; set CHROME_PATH")
}
