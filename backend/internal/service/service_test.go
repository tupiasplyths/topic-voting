package service

import (
	"testing"
)

func TestComputeWeight_ChatMode_NonDonation(t *testing.T) {
	pv := &PendingVote{VotingMode: "chat", IsDonation: false}
	w := computeWeight(pv, nil)
	if w != 1.0 {
		t.Fatalf("expected 1.0, got %f", w)
	}
}

func TestComputeWeight_ChatMode_DonationAmount(t *testing.T) {
	pv := &PendingVote{VotingMode: "chat", IsDonation: true, DonationAmount: 5.50}
	w := computeWeight(pv, nil)
	if w != 6.50 {
		t.Fatalf("expected 6.50, got %f", w)
	}
}

func TestComputeWeight_ChatMode_Bits(t *testing.T) {
	pv := &PendingVote{VotingMode: "chat", IsDonation: true, BitsAmount: 150}
	w := computeWeight(pv, nil)
	if w != 2.5 {
		t.Fatalf("expected 2.5, got %f", w)
	}
}

func TestComputeWeight_ChatMode_ZeroBits(t *testing.T) {
	pv := &PendingVote{VotingMode: "chat", IsDonation: true, BitsAmount: 0}
	w := computeWeight(pv, nil)
	if w != 1.0 {
		t.Fatalf("expected 1.0, got %f", w)
	}
}

func TestComputeWeight_DonationMode_NonDonation(t *testing.T) {
	pv := &PendingVote{VotingMode: "donation", IsDonation: false}
	w := computeWeight(pv, nil)
	if w != 0 {
		t.Fatalf("expected 0, got %f", w)
	}
}

func TestComputeWeight_DonationMode_Bits(t *testing.T) {
	pv := &PendingVote{VotingMode: "donation", IsDonation: true, BitsAmount: 500}
	w := computeWeight(pv, nil)
	if w != 5.0 {
		t.Fatalf("expected 5.0, got %f", w)
	}
}

func TestComputeWeight_DonationMode_DonationAmount(t *testing.T) {
	ex := &ExchangeRateService{rates: map[string]float64{"USD": 1.0}}
	pv := &PendingVote{VotingMode: "donation", IsDonation: true, DonationAmount: 20.0, DonationCurrency: "USD"}
	w := computeWeight(pv, ex)
	if w != 20.0 {
		t.Fatalf("expected 20.0, got %f", w)
	}
}

func TestComputeWeight_DonationMode_DonationAmount_WithExchange(t *testing.T) {
	ex := &ExchangeRateService{rates: map[string]float64{"CAD": 0.74}}
	pv := &PendingVote{VotingMode: "donation", IsDonation: true, DonationAmount: 100.0, DonationCurrency: "CAD"}
	w := computeWeight(pv, ex)
	if w != 74.0 {
		t.Fatalf("expected 74.0, got %f", w)
	}
}

func TestComputeWeight_DonationMode_DonationAmount_UnknownCurrency(t *testing.T) {
	ex := &ExchangeRateService{rates: map[string]float64{"USD": 1.0}}
	pv := &PendingVote{VotingMode: "donation", IsDonation: true, DonationAmount: 50.0, DonationCurrency: "XYZ"}
	w := computeWeight(pv, ex)
	if w != 50.0 {
		t.Fatalf("expected 50.0, got %f", w)
	}
}

func TestLevenshteinRatio(t *testing.T) {
	tests := []struct {
		a, b string
		want float64
	}{
		{"", "", 1.0},
		{"abc", "abc", 1.0},
		{"ABC", "abc", 0.0},
		{"Burger", "Burgers", 0.857},
		{"Pizza", "pizza", 0.800},
		{"Pizza", "Pizzas", 0.833},
		{"Cheese", "Cheesy", 0.833},
		{"A", "B", 0.0},
		{"abc", "xyz", 0.0},
	}

	for _, tt := range tests {
		got := levenshteinRatio(tt.a, tt.b)
		delta := got - tt.want
		if delta < 0 {
			delta = -delta
		}
		if delta > 0.001 {
			t.Errorf("levenshteinRatio(%q, %q) = %.3f, want %.3f", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestFindSimilarGroups(t *testing.T) {
	svc := &LabelCleanupService{similarity: 0.80}
	counts := map[string]int{"Burger": 10, "Burgers": 3, "Pizza": 20, "Pizzas": 5, "Cheese": 8, "Bread": 4}

	tests := []struct {
		name   string
		labels []string
		want   int
	}{
		{
			name:   "two similar pairs",
			labels: []string{"Burger", "Burgers", "Pizza", "Pizzas", "Cheese", "Bread"},
			want:   2,
		},
		{
			name:   "exact duplicates (case-insensitive via norm)",
			labels: []string{"pizza", "Pizza"},
			want:   1,
		},
		{
			name:   "no similar labels",
			labels: []string{"Pizza", "Burger", "Cheese"},
			want:   0,
		},
		{
			name:   "transitive chain: A~B, B~C",
			labels: []string{"hellp", "hello", "helo"},
			want:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			groups := svc.findSimilarGroups(tt.labels, counts)
			if len(groups) != tt.want {
				t.Errorf("findSimilarGroups() got %d groups, want %d", len(groups), tt.want)
			}
		})
	}
}

func TestPickCanonical(t *testing.T) {
	svc := &LabelCleanupService{}
	counts := map[string]int{"Burger": 10, "Burgers": 3, "Pizza": 20, "pizza": 5}

	got := svc.pickCanonical([]string{"Burgers", "Burger"}, counts)
	if got != "Burger" {
		t.Errorf("pickCanonical() want Burger, got %q", got)
	}

	got = svc.pickCanonical([]string{"pizza", "Pizza"}, counts)
	if got != "Pizza" {
		t.Errorf("pickCanonical() want Pizza, got %q", got)
	}

	got = svc.pickCanonical([]string{"EqualA", "EqualB"}, map[string]int{"EqualA": 5, "EqualB": 5})
	if got != "EqualA" {
		t.Errorf("pickCanonical() want EqualA (tiebreaker: first in list), got %q", got)
	}
}

func TestJsonLabels(t *testing.T) {
	got := jsonLabels([]string{"Pizza", "Burger"})
	want := `["Pizza","Burger"]`
	if got != want {
		t.Errorf("jsonLabels() got %q, want %q", got, want)
	}
}
