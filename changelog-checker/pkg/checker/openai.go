package checker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OpenAIClient is a simple client for OpenAI API
type OpenAIClient struct {
	apiKey     string
	httpClient *http.Client
}

// NewOpenAIClient creates a new OpenAI client
func NewOpenAIClient(apiKey string) *OpenAIClient {
	return &OpenAIClient{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// ChatRequest represents a request to the OpenAI Chat API
type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

// Message represents a message in a ChatRequest
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatResponse represents a response from the OpenAI Chat API
type ChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

// CheckSimilarity checks if two texts are similar in meaning using OpenAI API
func (c *OpenAIClient) CheckSimilarity(text1, text2 string) (bool, error) {
	// Create request
	chatRequest := ChatRequest{
		Model: "gpt-3.5-turbo",
		Messages: []Message{
			{
				Role:    "system",
				Content: "You are a helpful assistant that determines if two texts are similar in meaning.",
			},
			{
				Role:    "user",
				Content: fmt.Sprintf("PR Title: %s\nChangelog Description: %s\n\nAre these two texts describing the same change? Answer only YES or NO.", text1, text2),
			},
		},
	}
	
	// Convert to JSON
	jsonData, err := json.Marshal(chatRequest)
	if err != nil {
		return false, err
	}
	
	// Create HTTP request
	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return false, err
	}
	
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	
	// Send request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	
	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}
	
	// Parse response
	var chatResponse ChatResponse
	if err := json.Unmarshal(body, &chatResponse); err != nil {
		return false, err
	}
	
	// Check for error
	if chatResponse.Error.Message != "" {
		return false, fmt.Errorf("OpenAI API error: %s", chatResponse.Error.Message)
	}
	
	// Check if response has choices
	if len(chatResponse.Choices) == 0 {
		return false, fmt.Errorf("OpenAI API returned no choices")
	}
	
	// Get answer
	answer := chatResponse.Choices[0].Message.Content
	
	// Convert to uppercase for comparison
	answer = strings.ToUpper(answer)
	
	// Check if answer contains YES
	return strings.Contains(answer, "YES"), nil
}

// TestOpenAIKey tests if the OpenAI API key is valid
func (c *OpenAIClient) TestOpenAIKey() (bool, error) {
	chatRequest := ChatRequest{
		Model: "gpt-3.5-turbo",
		Messages: []Message{
			{
				Role:    "user",
				Content: "Say TEST",
			},
		},
	}
	
	// Convert to JSON
	jsonData, err := json.Marshal(chatRequest)
	if err != nil {
		return false, err
	}
	
	// Create HTTP request
	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return false, err
	}
	
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	
	// Send request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	
	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}
	
	// Parse response
	var chatResponse ChatResponse
	if err := json.Unmarshal(body, &chatResponse); err != nil {
		return false, err
	}
	
	// Check for error
	if chatResponse.Error.Message != "" {
		return false, fmt.Errorf("OpenAI API error: %s", chatResponse.Error.Message)
	}
	
	return true, nil
}