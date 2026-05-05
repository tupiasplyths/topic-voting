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
