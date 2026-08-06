package filter

import (
	"testing"

	"github.com/Remindal/scout/internal/domain"
)

func testRules() *Rules {
	return NewRules(
		[]string{"go", "golang", "backend", "api", "scraper"},
		[]string{"blockchain", "crypto", "web3"},
		50,
	)
}

func TestRulesAcceptMatchingJob(t *testing.T) {
	ok, _ := testRules().Accept(domain.Job{
		Title:       "Golang backend API developer",
		Description: "Build a REST service",
		Budget:      "$500",
	})
	if !ok {
		t.Fatal("matching job should pass")
	}
}

func TestRulesRejectNoIncludeKeyword(t *testing.T) {
	ok, reason := testRules().Accept(domain.Job{
		Title:       "Logo designer needed",
		Description: "Photoshop work for brand identity",
		Budget:      "$500",
	})
	if ok {
		t.Fatal("job without include keywords should be rejected")
	}
	if reason == "" {
		t.Fatal("rejection should carry a reason")
	}
}

func TestRulesRejectExcludeKeyword(t *testing.T) {
	ok, _ := testRules().Accept(domain.Job{
		Title:       "Golang backend for crypto exchange",
		Description: "Web3 trading API",
		Budget:      "$5000",
	})
	if ok {
		t.Fatal("job with exclude keyword should be rejected even if include keyword matches")
	}
}

func TestRulesRejectLowFixedBudget(t *testing.T) {
	ok, _ := testRules().Accept(domain.Job{
		Title:       "Go scraper",
		Description: "quick crawler task",
		Budget:      "$20",
	})
	if ok {
		t.Fatal("fixed budget below min should be rejected")
	}
}

func TestRulesHourlyBudgetSkipsBudgetCheck(t *testing.T) {
	ok, _ := testRules().Accept(domain.Job{
		Title:       "Go backend help",
		Description: "api maintenance",
		Budget:      "$5-10/hr",
	})
	if !ok {
		t.Fatal("hourly budget is not strongly parsed and must not trigger budget rejection")
	}
}

func TestParseFixedBudgetUSD(t *testing.T) {
	cases := []struct {
		in     string
		want   int
		wantOK bool
	}{
		{"$500", 500, true},
		{"$1,200", 1200, true},
		{" $75 ", 75, true},
		{"$25-50/hr", 0, false},
		{"", 0, false},
		{"Hourly", 0, false},
	}
	for _, c := range cases {
		got, ok := ParseFixedBudgetUSD(c.in)
		if ok != c.wantOK || (ok && got != c.want) {
			t.Errorf("ParseFixedBudgetUSD(%q) = %d,%v; want %d,%v", c.in, got, ok, c.want, c.wantOK)
		}
	}
}
