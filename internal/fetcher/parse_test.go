package fetcher

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseMoney(t *testing.T) {
	cases := []struct {
		in   string
		want float64
		nil  bool
	}{
		{"$23K", 23000, false},
		{"$3.1K total spent", 3100, false},
		{"$0", 0, false},
		{"$0 spent", 0, false},
		{"$1.5M", 1500000, false},
		{"$500", 500, false},
		{"$1,200", 1200, false},
		{"no money here", 0, true},
		{"", 0, true},
	}
	for _, c := range cases {
		got := ParseMoney(c.in)
		if c.nil {
			if got != nil {
				t.Errorf("ParseMoney(%q) = %v, want nil", c.in, *got)
			}
			continue
		}
		if got == nil || *got != c.want {
			t.Errorf("ParseMoney(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestParseProposalsBucket(t *testing.T) {
	cases := map[string]string{
		"Less than 5":       "fewer_than_5",
		"Proposals: Less than 5": "fewer_than_5",
		"5 to 10":           "5_to_10",
		"10 to 15":          "10_to_15",
		"15 to 20":          "15_to_20",
		"20 to 50":          "20_to_50",
		"50+":               "50_plus",
		"unknown text":      "",
		"":                  "",
	}
	for in, want := range cases {
		if got := ParseProposalsBucket(in); got != want {
			t.Errorf("ParseProposalsBucket(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseRelativeTime(t *testing.T) {
	now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		in      string
		wantAgo time.Duration
		nil     bool
	}{
		{"Posted 45 minutes ago", 45 * time.Minute, false},
		{"Posted 2 hours ago", 2 * time.Hour, false},
		{"Posted 3 days ago", 3 * 24 * time.Hour, false},
		{"Posted yesterday", 24 * time.Hour, false},
		{"Posted last week", 7 * 24 * time.Hour, false},
		{"last week", 7 * 24 * time.Hour, false},
		{"Posted a day ago", 24 * time.Hour, false},
		{"garbage", 0, true},
		{"", 0, true},
	}
	for _, c := range cases {
		got := ParseRelativeTime(c.in, now)
		if c.nil {
			if got != nil {
				t.Errorf("ParseRelativeTime(%q) = %v, want nil", c.in, got)
			}
			continue
		}
		if got == nil {
			t.Fatalf("ParseRelativeTime(%q) = nil, want %v ago", c.in, c.wantAgo)
		}
		if d := now.Sub(*got); d != c.wantAgo {
			t.Errorf("ParseRelativeTime(%q) ago = %v, want %v", c.in, d, c.wantAgo)
		}
	}
}

func TestParseClientSignals(t *testing.T) {
	jsonStr := `[
	  {"cid":"~022085339489984003058","verified":true,"spent":3100,"feedback":4.9,"proposalsTier":"Less than 5","publishedOn":"2026-08-06T12:20:19.030Z"},
	  {"cid":"~022084978184954154565","verified":false,"spent":0,"feedback":null,"proposalsTier":null,"publishedOn":null}
	]`
	m, err := ParseClientSignals(jsonStr)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(m) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(m))
	}
	rich := m["022085339489984003058"]
	if rich.Verified == nil || !*rich.Verified {
		t.Error("verified should be true")
	}
	if rich.Spent == nil || *rich.Spent != 3100 {
		t.Errorf("spent = %v, want 3100", rich.Spent)
	}
	poor := m["022084978184954154565"]
	if poor.Verified == nil || *poor.Verified {
		t.Error("verified should be false (not nil)")
	}
	if poor.Spent == nil || *poor.Spent != 0 {
		t.Errorf("spent = %v, want 0", poor.Spent)
	}

	if _, err := ParseClientSignals("not json"); err == nil {
		t.Error("invalid json should return error")
	}
}

func TestParseDetailPageRichFixture(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("tests", "fixtures", "job_detail_rich.html"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	info := ParseDetailPage(string(data), time.Now().UTC())

	// $23K total spent
	if info.Spent == nil || *info.Spent != 23000 {
		t.Errorf("spent = %v, want 23000", info.Spent)
	}
	// Rating is 4.9 out of 5
	if info.Rating == nil || *info.Rating != 4.9 {
		t.Errorf("rating = %v, want 4.9", info.Rating)
	}
}

func TestParseDetailPageActivityFixture(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("tests", "fixtures", "job_detail_good.html"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	info := ParseDetailPage(string(data), time.Now().UTC())

	if info.Interviewing == nil || *info.Interviewing != 0 {
		t.Errorf("interviewing = %v, want 0", info.Interviewing)
	}
	if info.InvitesSent == nil || *info.InvitesSent != 0 {
		t.Errorf("invites = %v, want 0", info.InvitesSent)
	}
	if info.Proposals != "15_to_20" {
		t.Errorf("proposals = %q, want 15_to_20", info.Proposals)
	}
	// 该 fixture 无 Last viewed 行 → nil，不报错
	if info.LastViewedAt != nil {
		t.Errorf("last_viewed should be nil, got %v", info.LastViewedAt)
	}
}
