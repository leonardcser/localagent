package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"localagent/pkg/finance"
)

type CurrencyTool struct {
	yf *finance.YahooClient
}

func NewCurrencyTool(yf *finance.YahooClient) *CurrencyTool {
	return &CurrencyTool{yf: yf}
}

func (t *CurrencyTool) Name() string {
	return "convert_currency"
}

func (t *CurrencyTool) Description() string {
	return "Convert an amount between currencies using live exchange rates. Use ISO 4217 currency codes (e.g. USD, EUR, GBP, JPY, CHF, CAD, AUD, CNY, INR, BRL)."
}

func (t *CurrencyTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"from": map[string]any{
				"type":        "string",
				"description": "Source currency code (e.g. USD)",
			},
			"to": map[string]any{
				"type":        "string",
				"description": "Target currency code (e.g. EUR)",
			},
			"amount": map[string]any{
				"type":        "number",
				"description": "Amount to convert (defaults to 1)",
			},
		},
		"required": []string{"from", "to"},
	}
}

func (t *CurrencyTool) DeclaredDomains() []string {
	return []string{"query2.finance.yahoo.com", "fc.yahoo.com"}
}

func (t *CurrencyTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	from, _ := args["from"].(string)
	to, _ := args["to"].(string)
	if from == "" || to == "" {
		return ErrorResult("both 'from' and 'to' currency codes are required")
	}

	from = strings.ToUpper(strings.TrimSpace(from))
	to = strings.ToUpper(strings.TrimSpace(to))

	amount := 1.0
	if a, ok := args["amount"].(float64); ok && a > 0 {
		amount = a
	}

	symbol := from + to + "=X"

	body, err := t.yf.FetchQuoteSummary(ctx, symbol, "price")
	if err != nil {
		return ErrorResult(fmt.Sprintf("failed to fetch exchange rate for %s/%s: %v", from, to, err))
	}

	var result struct {
		Price struct {
			RegularMarketPrice     finance.Value `json:"regularMarketPrice"`
			RegularMarketChange    finance.Value `json:"regularMarketChange"`
			RegularMarketChangePct finance.Value `json:"regularMarketChangePercent"`
			RegularMarketDayHigh   finance.Value `json:"regularMarketDayHigh"`
			RegularMarketDayLow    finance.Value `json:"regularMarketDayLow"`
			RegularMarketPrevClose finance.Value `json:"regularMarketPreviousClose"`
			FiftyTwoWeekHigh       finance.Value `json:"fiftyTwoWeekHigh"`
			FiftyTwoWeekLow        finance.Value `json:"fiftyTwoWeekLow"`
			ShortName              string        `json:"shortName"`
		} `json:"price"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return ErrorResult(fmt.Sprintf("failed to parse exchange rate data: %v", err))
	}

	rate := result.Price.RegularMarketPrice.Raw
	if rate == 0 {
		return ErrorResult(fmt.Sprintf("no exchange rate found for %s/%s", from, to))
	}

	converted := amount * rate

	var b strings.Builder

	fmt.Fprintf(&b, "%s/%s", from, to)
	if result.Price.ShortName != "" {
		fmt.Fprintf(&b, " (%s)", result.Price.ShortName)
	}
	b.WriteString("\n")

	fmt.Fprintf(&b, "Rate: %s", result.Price.RegularMarketPrice.Fmt)
	if result.Price.RegularMarketChange.Fmt != "" {
		direction := "+"
		if result.Price.RegularMarketChange.Raw < 0 {
			direction = ""
		}
		fmt.Fprintf(&b, " (%s%s, %s%s)", direction, result.Price.RegularMarketChange.Fmt, direction, result.Price.RegularMarketChangePct.Fmt)
	}
	b.WriteString("\n")

	if amount != 1 {
		fmt.Fprintf(&b, "\n%.2f %s = %.2f %s\n", amount, from, converted, to)
	}

	if result.Price.RegularMarketDayHigh.Fmt != "" && result.Price.RegularMarketDayLow.Fmt != "" {
		fmt.Fprintf(&b, "Day Range: %s - %s\n", result.Price.RegularMarketDayLow.Fmt, result.Price.RegularMarketDayHigh.Fmt)
	}
	if result.Price.FiftyTwoWeekLow.Fmt != "" && result.Price.FiftyTwoWeekHigh.Fmt != "" {
		fmt.Fprintf(&b, "52-Week Range: %s - %s\n", result.Price.FiftyTwoWeekLow.Fmt, result.Price.FiftyTwoWeekHigh.Fmt)
	}
	if result.Price.RegularMarketPrevClose.Fmt != "" {
		fmt.Fprintf(&b, "Previous Close: %s\n", result.Price.RegularMarketPrevClose.Fmt)
	}

	return SilentResult(b.String())
}
