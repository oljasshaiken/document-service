package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTMLToPDFHandler_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/html-to-pdf", nil)
	rr := httptest.NewRecorder()

	htmlToPDFHandler(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rr.Code)
	}
}

func TestHTMLToPDFHandler_EmptyHTML(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/html-to-pdf", strings.NewReader(""))
	rr := httptest.NewRecorder()

	htmlToPDFHandler(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestHTMLToPDFHandler_PDFGenerationError(t *testing.T) {
	origGenerator := pdfGenerator
	pdfGenerator = func(string) ([]byte, error) {
		return nil, errors.New("boom")
	}
	t.Cleanup(func() {
		pdfGenerator = origGenerator
	})

	req := httptest.NewRequest(http.MethodPost, "/html-to-pdf", strings.NewReader("<h1>hi</h1>"))
	rr := httptest.NewRecorder()

	htmlToPDFHandler(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
	}
}

func TestHTMLToPDFHandler_Success(t *testing.T) {
	const fakePDF = "%PDF-1.7 fake"
	origGenerator := pdfGenerator
	pdfGenerator = func(html string) ([]byte, error) {
		if html == "" {
			t.Fatal("expected non-empty html")
		}
		return []byte(fakePDF), nil
	}
	t.Cleanup(func() {
		pdfGenerator = origGenerator
	})

	req := httptest.NewRequest(http.MethodPost, "/html-to-pdf", strings.NewReader("<p>Hello</p>"))
	rr := httptest.NewRecorder()

	htmlToPDFHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
	if got := rr.Header().Get("Content-Type"); got != "application/pdf" {
		t.Fatalf("expected Content-Type application/pdf, got %q", got)
	}
	if got := rr.Header().Get("Content-Disposition"); got != `attachment; filename="document.pdf"` {
		t.Fatalf("unexpected Content-Disposition: %q", got)
	}
	if body := rr.Body.String(); body != fakePDF {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestHTMLToPDFHandler_ReadBodyError(t *testing.T) {
	origGenerator := pdfGenerator
	pdfGenerator = func(string) ([]byte, error) {
		t.Fatal("pdf generator should not be called for body read error")
		return nil, nil
	}
	t.Cleanup(func() {
		pdfGenerator = origGenerator
	})

	req := httptest.NewRequest(http.MethodPost, "/html-to-pdf", errReader{})
	rr := httptest.NewRecorder()

	htmlToPDFHandler(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestHTMLToPDFHandler_BodyTooLarge(t *testing.T) {
	origGenerator := pdfGenerator
	pdfGenerator = func(string) ([]byte, error) {
		t.Fatal("pdf generator should not be called for oversized request")
		return nil, nil
	}
	t.Cleanup(func() {
		pdfGenerator = origGenerator
	})

	tooLarge := strings.Repeat("a", maxHTMLBodySize+1)
	req := httptest.NewRequest(http.MethodPost, "/html-to-pdf", strings.NewReader(tooLarge))
	rr := httptest.NewRecorder()

	htmlToPDFHandler(rr, req)

	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status %d, got %d", http.StatusRequestEntityTooLarge, rr.Code)
	}
}

func TestHealthHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()

	healthHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
	if got := rr.Body.String(); got != "ok" {
		t.Fatalf("expected health body %q, got %q", "ok", got)
	}
}

func TestNewServer_Routes(t *testing.T) {
	srv := newServer()

	healthReq := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	healthRR := httptest.NewRecorder()
	srv.Handler.ServeHTTP(healthRR, healthReq)
	if healthRR.Code != http.StatusOK {
		t.Fatalf("expected health route status %d, got %d", http.StatusOK, healthRR.Code)
	}

	notFoundReq := httptest.NewRequest(http.MethodGet, "/not-found", nil)
	notFoundRR := httptest.NewRecorder()
	srv.Handler.ServeHTTP(notFoundRR, notFoundReq)
	if notFoundRR.Code != http.StatusNotFound {
		t.Fatalf("expected not found status %d, got %d", http.StatusNotFound, notFoundRR.Code)
	}
}

func TestXMLToPDFHandler_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/xml-to-pdf", nil)
	rr := httptest.NewRecorder()

	xmlToPDFHandler(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rr.Code)
	}
}

func TestXMLToPDFHandler_InvalidXML(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/xml-to-pdf", strings.NewReader("<root><unclosed></root>"))
	rr := httptest.NewRecorder()

	xmlToPDFHandler(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestXMLToPDFHandler_Success(t *testing.T) {
	const fakePDF = "%PDF-1.7 fake"
	origGenerator := pdfGenerator
	pdfGenerator = func(content string) ([]byte, error) {
		if !strings.Contains(content, "<pre>") {
			t.Fatal("expected XML content wrapped in pre tag")
		}
		if !strings.Contains(content, "&lt;note&gt;") {
			t.Fatal("expected escaped XML content")
		}
		return []byte(fakePDF), nil
	}
	t.Cleanup(func() {
		pdfGenerator = origGenerator
	})

	xmlBody := `<note><to>Tove</to><from>Jani</from><body>Hello</body></note>`
	req := httptest.NewRequest(http.MethodPost, "/xml-to-pdf", strings.NewReader(xmlBody))
	rr := httptest.NewRecorder()

	xmlToPDFHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
	if got := rr.Header().Get("Content-Type"); got != "application/pdf" {
		t.Fatalf("expected Content-Type application/pdf, got %q", got)
	}
	if body := rr.Body.String(); body != fakePDF {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestToPDFSource_HTMLPassthrough(t *testing.T) {
	input := "<h1>Hello</h1>"
	got, err := toPDFSource(input, "html")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != input {
		t.Fatalf("expected passthrough html %q, got %q", input, got)
	}
}

func TestToPDFSource_UnsupportedFormat(t *testing.T) {
	_, err := toPDFSource("payload", "markdown")
	if err == nil {
		t.Fatal("expected error for unsupported format")
	}
}

func TestValidateXML_Valid(t *testing.T) {
	if err := validateXML(`<root><item>ok</item></root>`); err != nil {
		t.Fatalf("expected valid XML, got error: %v", err)
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) {
	return 0, errors.New("read failed")
}
