package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

func generatePDF(parent context.Context, cfg *Config, html string) ([]byte, error) {
	chromePath, err := findChromePath()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(parent, cfg.PDFTimeout)
	defer cancel()

	allocOpts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(chromePath),
		chromedp.NoDefaultBrowserCheck,
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx, allocOpts...)
	defer allocCancel()

	browserCtx, browserCancel := chromedp.NewContext(allocCtx)
	defer browserCancel()

	var pdfBytes []byte

	err = chromedp.Run(browserCtx,
		chromedp.Navigate("data:text/html;charset=utf-8,"+url.PathEscape(html)),
		chromedp.WaitReady("body"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			pdfBytes, _, err = page.PrintToPDF().
				WithPrintBackground(true).
				WithMarginTop(0.4).
				WithMarginBottom(0.4).
				WithMarginLeft(0.4).
				WithMarginRight(0.4).
				WithPaperWidth(8.27).
				WithPaperHeight(11.69).
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
