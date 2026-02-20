package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"localagent/pkg/finance"
)

type StockTool struct {
	yf *finance.YahooClient
}

func NewStockTool(yf *finance.YahooClient) *StockTool {
	return &StockTool{yf: yf}
}

func (t *StockTool) Name() string {
	return "stock_price"
}

func (t *StockTool) Description() string {
	return "Get current stock price and financial data for a ticker symbol, index, or commodity. Examples: NVDA, AAPL, ^GSPC (S&P 500), ^DJI (Dow Jones), GC=F (gold), CL=F (crude oil), BTC-USD (Bitcoin)."
}

func (t *StockTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"symbol": map[string]any{
				"type":        "string",
				"description": "Ticker symbol (e.g. NVDA, ^GSPC, GC=F, BTC-USD)",
			},
		},
		"required": []string{"symbol"},
	}
}

func (t *StockTool) DeclaredDomains() []string {
	return []string{"query2.finance.yahoo.com", "fc.yahoo.com"}
}

func (t *StockTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	symbol, ok := args["symbol"].(string)
	if !ok || symbol == "" {
		return ErrorResult("symbol is required")
	}

	data, err := t.fetchQuote(ctx, symbol)
	if err != nil {
		return ErrorResult(fmt.Sprintf("failed to fetch quote for %s: %v", symbol, err))
	}

	return SilentResult(data)
}

func (t *StockTool) fetchQuote(ctx context.Context, symbol string) (string, error) {
	body, err := t.yf.FetchQuoteSummary(ctx, symbol, "price")
	if err != nil {
		return "", err
	}

	var result struct {
		Price json.RawMessage `json:"price"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	return formatStockPrice(symbol, result.Price)
}

func formatStockPrice(symbol string, raw json.RawMessage) (string, error) {
	var price struct {
		ShortName              string        `json:"shortName"`
		LongName               string        `json:"longName"`
		Currency               string        `json:"currency"`
		Exchange               string        `json:"exchangeName"`
		QuoteType              string        `json:"quoteType"`
		MarketState            string        `json:"marketState"`
		RegularMarketPrice     finance.Value `json:"regularMarketPrice"`
		RegularMarketChange    finance.Value `json:"regularMarketChange"`
		RegularMarketChangePct finance.Value `json:"regularMarketChangePercent"`
		RegularMarketDayHigh   finance.Value `json:"regularMarketDayHigh"`
		RegularMarketDayLow    finance.Value `json:"regularMarketDayLow"`
		RegularMarketVolume    finance.Value `json:"regularMarketVolume"`
		RegularMarketOpen      finance.Value `json:"regularMarketOpen"`
		RegularMarketPrevClose finance.Value `json:"regularMarketPreviousClose"`
		MarketCap              finance.Value `json:"marketCap"`
		FiftyTwoWeekHigh       finance.Value `json:"fiftyTwoWeekHigh"`
		FiftyTwoWeekLow        finance.Value `json:"fiftyTwoWeekLow"`
		PostMarketPrice        finance.Value `json:"postMarketPrice"`
		PostMarketChange       finance.Value `json:"postMarketChange"`
		PostMarketChangePct    finance.Value `json:"postMarketChangePercent"`
		PreMarketPrice         finance.Value `json:"preMarketPrice"`
		PreMarketChange        finance.Value `json:"preMarketChange"`
		PreMarketChangePct     finance.Value `json:"preMarketChangePercent"`
	}

	if err := json.Unmarshal(raw, &price); err != nil {
		return "", fmt.Errorf("failed to parse price data: %w", err)
	}

	name := price.LongName
	if name == "" {
		name = price.ShortName
	}

	var b strings.Builder

	fmt.Fprintf(&b, "%s (%s)\n", name, symbol)
	fmt.Fprintf(&b, "Exchange: %s | Type: %s | Currency: %s\n", price.Exchange, price.QuoteType, price.Currency)
	fmt.Fprintf(&b, "Market State: %s\n\n", price.MarketState)

	fmt.Fprintf(&b, "Price: %s", price.RegularMarketPrice.Fmt)
	if price.RegularMarketChange.Fmt != "" {
		direction := "+"
		if price.RegularMarketChange.Raw < 0 {
			direction = ""
		}
		fmt.Fprintf(&b, " (%s%s, %s%s)", direction, price.RegularMarketChange.Fmt, direction, price.RegularMarketChangePct.Fmt)
	}
	b.WriteString("\n")

	if price.RegularMarketOpen.Fmt != "" {
		fmt.Fprintf(&b, "Open: %s\n", price.RegularMarketOpen.Fmt)
	}
	if price.RegularMarketDayHigh.Fmt != "" && price.RegularMarketDayLow.Fmt != "" {
		fmt.Fprintf(&b, "Day Range: %s - %s\n", price.RegularMarketDayLow.Fmt, price.RegularMarketDayHigh.Fmt)
	}
	if price.FiftyTwoWeekLow.Fmt != "" && price.FiftyTwoWeekHigh.Fmt != "" {
		fmt.Fprintf(&b, "52-Week Range: %s - %s\n", price.FiftyTwoWeekLow.Fmt, price.FiftyTwoWeekHigh.Fmt)
	}
	if price.RegularMarketVolume.Fmt != "" {
		fmt.Fprintf(&b, "Volume: %s\n", price.RegularMarketVolume.Fmt)
	}
	if price.RegularMarketPrevClose.Fmt != "" {
		fmt.Fprintf(&b, "Previous Close: %s\n", price.RegularMarketPrevClose.Fmt)
	}
	if price.MarketCap.Fmt != "" {
		fmt.Fprintf(&b, "Market Cap: %s\n", price.MarketCap.Fmt)
	}

	if price.MarketState == "POST" || price.MarketState == "PREPRE" || price.MarketState == "POSTPOST" {
		if price.PostMarketPrice.Fmt != "" {
			direction := "+"
			if price.PostMarketChange.Raw < 0 {
				direction = ""
			}
			fmt.Fprintf(&b, "\nAfter Hours: %s (%s%s, %s%s)\n",
				price.PostMarketPrice.Fmt, direction, price.PostMarketChange.Fmt, direction, price.PostMarketChangePct.Fmt)
		}
	}
	if price.MarketState == "PRE" {
		if price.PreMarketPrice.Fmt != "" {
			direction := "+"
			if price.PreMarketChange.Raw < 0 {
				direction = ""
			}
			fmt.Fprintf(&b, "\nPre-Market: %s (%s%s, %s%s)\n",
				price.PreMarketPrice.Fmt, direction, price.PreMarketChange.Fmt, direction, price.PreMarketChangePct.Fmt)
		}
	}

	return b.String(), nil
}
