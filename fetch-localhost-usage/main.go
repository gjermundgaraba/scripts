package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

type Chain struct {
	Path string `json:"path"`

	baseUrl string // Optional and only used when fetching for a specific chain
}

// ChainDirectoryResponse represents the structure of the response from https://chains.cosmos.directory
type ChainDirectoryResponse struct {
	Chains []Chain `json:"chains"`
}

type ConnectionResponse struct {
	Connections []Connection `json:"connections"`
	// Pagination can be helpful if you want to check "total" or "next_key"
	Pagination Pagination `json:"pagination"`
}

func (c ConnectionResponse) GetItems() []Connection {
	return c.Connections
}

func (c ConnectionResponse) GetPagination() Pagination {
	return c.Pagination
}

type Connection struct {
	ID       string `json:"id"`
	ClientID string `json:"client_id"`
}

type ChannelResponse struct {
	Channels []struct{} `json:"channels"`
	// Pagination can be helpful if you want to check "total" or "next_key"
	Pagination Pagination `json:"pagination"`
}

func (c ChannelResponse) GetItems() []struct{} {
	return c.Channels
}

func (c ChannelResponse) GetPagination() Pagination {
	return c.Pagination
}

type Pagination struct {
	NextKey interface{} `json:"next_key"`
	Total   string      `json:"total"`
}

type PaginatedResponse[T any] interface {
	GetItems() []T
	GetPagination() Pagination
}

func main() {
	// 1. Fetch the list of chains (or use the provided chain argument)
	var chains []Chain
	if len(os.Args) > 1 {
		chainPath := os.Args[1]

		fmt.Println("Chain argument provided, will only fetch connections for chain:", chainPath)

		baseUrl := ""
		if len(os.Args) > 2 {
			baseUrl = os.Args[2]
		}

		fmt.Println("Base URL override provided:", baseUrl)

		chains = []Chain{{Path: chainPath, baseUrl: baseUrl}}
	} else {
		var err error
		chains, err = fetchChains()
		if err != nil {
			log.Fatalf("Failed to fetch chains: %v", err)
		}
	}

	// Create/Truncate the output file
	fileName := "out/localhost_chain_usage.txt"
	file, err := os.Create(fileName)
	if err != nil {
		log.Fatalf("Failed to create output file: %v", err)
	}
	defer file.Close()

	// 2. For each chain, fetch all IBC connections in pages of 50
	for _, chain := range chains {
		connections, err := fetchPaginated[Connection](func(offset int) (PaginatedResponse[Connection], error) {
			return fetchIBCConnections(chain, offset, 50)
		})
		if err != nil {
			fmt.Printf("Failed to fetch connections for chain %s: %v\n", chain.Path, err)
			continue
		}

		numLocalhost := 0
		hasLocalhost := false
		for _, conn := range connections {
			if conn.ClientID == "09-localhost" {
				hasLocalhost = true
				channels, err := fetchPaginated[struct{}](func(offset int) (PaginatedResponse[struct{}], error) {
					return fetchIBCChannelsForConnection(chain, conn.ID, offset, 50)
				})
				if err != nil {
					fmt.Printf("Failed to fetch channels for connection %s on chain %s: %v\n", conn.ID, chain.Path, err)
					continue
				}
				numLocalhost += len(channels)
			}
		}

		if hasLocalhost {
			_, _ = file.WriteString(fmt.Sprintf("%s, %d\n", chain.Path, numLocalhost))
		}
	}

	fmt.Println("Done! Wrote chains with localhost in:", fileName)
}

func fetchPaginated[T any](f func(int) (PaginatedResponse[T], error)) ([]T, error) {
	offset := 0
	var all []T

	for {
		resp, err := f(offset)
		if err != nil {
			return nil, err
		}

		all = append(all, resp.GetItems()...)

		if resp.GetPagination().NextKey == nil {
			break
		}

		offset += len(resp.GetItems())
	}

	return all, nil
}

// fetchChains fetches the list of chains from https://chains.cosmos.directory
func fetchChains() ([]Chain, error) {
	resp, err := http.Get("https://chains.cosmos.directory")
	if err != nil {
		return nil, fmt.Errorf("GET error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	var chainResp ChainDirectoryResponse
	if err := json.Unmarshal(bodyBytes, &chainResp); err != nil {
		return nil, fmt.Errorf("JSON unmarshal error: %w", err)
	}

	return chainResp.Chains, nil
}

func fetchIBCConnections(chain Chain, offset, limit int) (*ConnectionResponse, error) {
	baseUrl := fmt.Sprintf("https://rest.cosmos.directory/%s", chain.Path)
	if chain.baseUrl != "" {
		baseUrl = chain.baseUrl
	}

	url := fmt.Sprintf("%s/ibc/core/connection/v1/connections?pagination.limit=%d&pagination.offset=%d", baseUrl, limit, offset)

	var resp *http.Response
	var err error
	if err := retryWithBackoff(5, func() error {
		resp, err = http.Get(url)
		if err != nil {
			return fmt.Errorf("GET error: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			defer resp.Body.Close()
			return fmt.Errorf("unexpected status: %s for chainPath=%s with url=%s", resp.Status, chain.Path, url)
		}

		return nil
	}); err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	var connections ConnectionResponse
	if err := json.Unmarshal(bodyBytes, &connections); err != nil {
		return nil, fmt.Errorf("JSON unmarshal error: %w", err)
	}

	fmt.Printf("Fetched %d connections for chain %s\n", len(connections.Connections), chain.Path)
	time.Sleep(500 * time.Millisecond) // Be nice to the server

	return &connections, nil
}

func fetchIBCChannelsForConnection(chain Chain, connectionID string, offset, limit int) (*ChannelResponse, error) {
	baseUrl := fmt.Sprintf("https://rest.cosmos.directory/%s", chain.Path)
	if chain.baseUrl != "" {
		baseUrl = chain.baseUrl
	}

	url := fmt.Sprintf("%s/ibc/core/channel/v1/connections/%s/channels?pagination.limit=%d&pagination.offset=%d", baseUrl, connectionID, limit, offset)

	var resp *http.Response
	var err error
	if err := retryWithBackoff(5, func() error {
		resp, err = http.Get(url)
		if err != nil {
			return fmt.Errorf("GET error: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			defer resp.Body.Close()
			return fmt.Errorf("unexpected status: %s for chainPath=%s with url=%s", resp.Status, chain.Path, url)
		}

		return nil
	}); err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	var channels ChannelResponse
	if err := json.Unmarshal(bodyBytes, &channels); err != nil {
		return nil, fmt.Errorf("JSON unmarshal error: %w", err)
	}

	fmt.Printf("Fetched %d channels for connection %s on chain %s\n", len(channels.Channels), connectionID, chain.Path)
	time.Sleep(500 * time.Millisecond) // Be nice to the server

	return &channels, nil
}

func retryWithBackoff(retries int, f func() error) error {
	for i := range retries {
		if err := f(); err != nil {
			log.Printf("Error: %v. Retrying in %d seconds...", err, i*2)
			time.Sleep(time.Duration(i*5) * time.Second)
		} else {
			return nil
		}
	}
	return fmt.Errorf("retries exhausted")
}
