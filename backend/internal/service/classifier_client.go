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

type ClassifierClient interface {
	Classify(ctx context.Context, message, topic string, existingLabels []string, threshold float64) (label string, confidence float64)
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
	Label      string  `json:"label"`
	Confidence float64 `json:"confidence"`
	IsNew      bool    `json:"is_new"`
}

func (c *classifierClient) Classify(ctx context.Context, message, topic string, existingLabels []string, threshold float64) (string, float64) {
	body := classifyRequest{
		Message:        message,
		Topic:          topic,
		ExistingLabels: existingLabels,
		Threshold:      threshold,
	}

	payload, err := json.Marshal(body)
	if err != nil {
		log.Printf("[classifier] marshal error: %v", err)
		return "uncategorized", 0.0
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/classify", bytes.NewReader(payload))
	if err != nil {
		log.Printf("[classifier] create request error: %v", err)
		return "uncategorized", 0.0
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Printf("[classifier] request error: %v", err)
		return "uncategorized", 0.0
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[classifier] non-200 status: %d", resp.StatusCode)
		return "uncategorized", 0.0
	}

	var result classifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("[classifier] decode error: %v", err)
		return "uncategorized", 0.0
	}

	return result.Label, result.Confidence
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
