package finance

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"sync"
	"time"
)

// Value represents a Yahoo Finance formatted value with raw number and display string.
type Value struct {
	Raw float64 `json:"raw"`
	Fmt string  `json:"fmt"`
}

// YahooClient handles authentication (crumb + cookies) for Yahoo Finance APIs.
type YahooClient struct {
	client *http.Client
	crumb  string
	mu     sync.Mutex
}

func NewYahooClient() *YahooClient {
	jar, _ := cookiejar.New(nil)
	return &YahooClient{
		client: &http.Client{
			Timeout: 15 * time.Second,
			Jar:     jar,
		},
	}
}

func (yc *YahooClient) getCrumb(ctx context.Context) (string, error) {
	yc.mu.Lock()
	defer yc.mu.Unlock()

	if yc.crumb != "" {
		return yc.crumb, nil
	}

	req, err := http.NewRequestWithContext(ctx, "GET", "https://fc.yahoo.com/", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := yc.client.Do(req)
	if err != nil {
		return "", err
	}
	resp.Body.Close()

	req, err = http.NewRequestWithContext(ctx, "GET", "https://query2.finance.yahoo.com/v1/test/getcrumb", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err = yc.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	crumb := strings.TrimSpace(string(body))
	if crumb == "" {
		return "", fmt.Errorf("empty crumb returned")
	}

	yc.crumb = crumb
	return crumb, nil
}

func (yc *YahooClient) clearCrumb() {
	yc.mu.Lock()
	yc.crumb = ""
	yc.mu.Unlock()
}

func (yc *YahooClient) get(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := yc.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Yahoo Finance returned status %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// FetchQuoteSummary fetches a quoteSummary module for a symbol, with automatic crumb retry.
func (yc *YahooClient) FetchQuoteSummary(ctx context.Context, symbol, modules string) (json.RawMessage, error) {
	fetch := func(crumb string) (json.RawMessage, error) {
		url := fmt.Sprintf(
			"https://query2.finance.yahoo.com/v10/finance/quoteSummary/%s?modules=%s&crumb=%s",
			symbol, modules, crumb,
		)
		body, err := yc.get(ctx, url)
		if err != nil {
			return nil, err
		}

		var envelope struct {
			QuoteSummary struct {
				Result []json.RawMessage `json:"result"`
				Error  *struct {
					Description string `json:"description"`
				} `json:"error"`
			} `json:"quoteSummary"`
		}
		if err := json.Unmarshal(body, &envelope); err != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}
		if envelope.QuoteSummary.Error != nil {
			return nil, fmt.Errorf("%s", envelope.QuoteSummary.Error.Description)
		}
		if len(envelope.QuoteSummary.Result) == 0 {
			return nil, fmt.Errorf("no data found for symbol %s", symbol)
		}
		return envelope.QuoteSummary.Result[0], nil
	}

	crumb, err := yc.getCrumb(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to authenticate with Yahoo Finance: %w", err)
	}

	data, err := fetch(crumb)
	if err != nil {
		yc.clearCrumb()
		crumb, err2 := yc.getCrumb(ctx)
		if err2 != nil {
			return nil, fmt.Errorf("failed to authenticate with Yahoo Finance: %w", err2)
		}
		data, err = fetch(crumb)
		if err != nil {
			return nil, err
		}
	}

	return data, nil
}
