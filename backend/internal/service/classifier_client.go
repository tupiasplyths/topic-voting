package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

type ClassifyResult struct {
	Label      string
	Confidence float64
	IsNew      bool
	AllScores  map[string]float64
}

type ClassifierClient interface {
	Classify(ctx context.Context, message, topic string, existingLabels []string, threshold float64) (*ClassifyResult, error)
	HealthCheck(ctx context.Context) error
}

type classifierClient struct {
	httpClient *http.Client
	baseURL    string
}

func NewClassifierClient(baseURL string, timeout time.Duration) ClassifierClient {
	return &classifierClient{
		httpClient: &http.Client{Timeout: timeout},
		baseURL:    baseURL,
	}
}

type classifyRequest struct {
	Message        string   `json:"message"`
	Topic          string   `json:"topic"`
	ExistingLabels []string `json:"existing_labels"`
	Threshold      float64  `json:"threshold"`
}

type classifyResponse struct {
	Label      string             `json:"label"`
	Confidence float64            `json:"confidence"`
	IsNew      bool               `json:"is_new"`
	AllScores  map[string]float64 `json:"all_scores"`
}

func (c *classifierClient) Classify(ctx context.Context, message, topic string, existingLabels []string, threshold float64) (*ClassifyResult, error) {
	body := classifyRequest{
		Message:        message,
		Topic:          topic,
		ExistingLabels: existingLabels,
		Threshold:      threshold,
	}

	payload, err := json.Marshal(body)
	if err != nil {
		log.Printf("[classifier] marshal error: %v", err)
		return &ClassifyResult{Label: "uncategorized"}, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/classify", bytes.NewReader(payload))
	if err != nil {
		log.Printf("[classifier] create request error: %v", err)
		return &ClassifyResult{Label: "uncategorized"}, nil
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Printf("[classifier] request error: %v", err)
		return &ClassifyResult{Label: "uncategorized"}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[classifier] non-200 status: %d", resp.StatusCode)
		return &ClassifyResult{Label: "uncategorized"}, nil
	}

	var result classifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("[classifier] decode error: %v", err)
		return &ClassifyResult{Label: "uncategorized"}, nil
	}

	return &ClassifyResult{
		Label:      result.Label,
		Confidence: result.Confidence,
		IsNew:      result.IsNew,
		AllScores:  result.AllScores,
	}, nil
}

func (c *classifierClient) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return fmt.Errorf("create health request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("health check request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned %d", resp.StatusCode)
	}

	return nil
}
