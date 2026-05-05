package service

import (
	"encoding/json"
	"errors"
)

var defaultRates = map[string]float64{
	"USD": 1.0,
	"CAD": 0.74,
	"EUR": 1.08,
	"GBP": 1.27,
	"AUD": 0.65,
	"BRL": 0.19,
}

type Exchanger interface {
	ToUSD(amount float64, currency string) float64
}

type ExchangeRateService struct {
	rates map[string]float64
}

func NewExchangeRateService(ratesJSON string) (*ExchangeRateService, error) {
	if ratesJSON == "" {
		rates := make(map[string]float64, len(defaultRates))
		for k, v := range defaultRates {
			rates[k] = v
		}
		return &ExchangeRateService{rates: rates}, nil
	}

	var rates map[string]float64
	if err := json.Unmarshal([]byte(ratesJSON), &rates); err != nil {
		return nil, errors.New("invalid rates JSON: " + err.Error())
	}
	return &ExchangeRateService{rates: rates}, nil
}

func (s *ExchangeRateService) ToUSD(amount float64, currency string) float64 {
	rate, ok := s.rates[currency]
	if !ok {
		rate = 1.0
	}
	return amount * rate
}
