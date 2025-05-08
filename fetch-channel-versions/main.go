package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
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

// ChannelResponse represents the structure of the IBC channels query response
type ChannelResponse struct {
	Channels []struct {
		// We'll only parse out the fields we need. The complete response has more fields.
		Version   string `json:"version"`
		ChannelID string `json:"channel_id"`
		State     string `json:"state"`
	} `json:"channels"`
	// Pagination can be helpful if you want to check "total" or "next_key"
	Pagination struct {
		NextKey interface{} `json:"next_key"`
		Total   string      `json:"total"`
	} `json:"pagination"`
}

type ChannelVersion struct {
	AppVersion string `json:"app_version"`
	FeeVersion string `json:"fee_version"`
	Version    string `json:"version"`
}

func main() {
	// 1. Fetch the list of chains (or use the provided chain argument)
	var chains []Chain
	if len(os.Args) > 1 {
		chainPath := os.Args[1]

		fmt.Println("Chain argument provided, will only fetch channels for chain:", chainPath)

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
	file, err := os.Create("out/channel_versions.txt")
	if err != nil {
		log.Fatalf("Failed to create output file: %v", err)
	}
	defer file.Close()

	// 2. For each chain, fetch all IBC channels in pages of 50
	for _, chain := range chains {
		offset := 0
		for {
			channels, err := fetchIBCChannels(chain, offset, 50)
			if err != nil {
				errorMsg := fmt.Sprintf("Failed to fetch channels for chain %s: %v", chain.Path, err)
				log.Println(errorMsg)
				_, _ = file.WriteString(errorMsg + "\n")

				break // Move on to the next chain
			}
			if len(channels.Channels) == 0 {
				// No more channels found, break out of paging loop
				break
			}

			// 3. Write every channel version to our file
			for _, ch := range channels.Channels {
				version := ch.Version
				var feeVersion string
				if strings.HasPrefix(ch.Version, "{") {
					var versionStruct ChannelVersion
					if err := json.Unmarshal([]byte(ch.Version), &versionStruct); err != nil {
						panic(err)
					}
					version = versionStruct.Version
					if version == "" {
						version = versionStruct.AppVersion
					}

					feeVersion = versionStruct.FeeVersion
				}

				_, _ = file.WriteString(fmt.Sprintf("%s, %s, %s, %s, %s\n", chain.Path, ch.ChannelID, ch.State, version, feeVersion))

			}

			// If we got fewer than 50 in this batch, we assume there are no more
			if len(channels.Channels) < 50 {
				break
			}
			offset += 50
		}
	}

	fmt.Println("Done! Wrote channel versions to channel_versions.txt")
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

// fetchIBCChannels fetches a page of up to `limit` channels for a given chain path
// using the REST endpoint at rest.cosmos.directory/{chainPath}.
func fetchIBCChannels(chain Chain, offset, limit int) (*ChannelResponse, error) {
	baseUrl := fmt.Sprintf("https://rest.cosmos.directory/%s", chain.Path)
	if chain.baseUrl != "" {
		baseUrl = chain.baseUrl
	}

	url := fmt.Sprintf("%s/ibc/core/channel/v1/channels?pagination.limit=%d&pagination.offset=%d", baseUrl, limit, offset)

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

	fmt.Printf("Fetched %d channels for chain %s\n", len(channels.Channels), chain.Path)
	time.Sleep(500 * time.Millisecond) // Be nice to the server

	return &channels, nil
}

func retryWithBackoff(retries int, f func() error) error {
	for i := 0; i < retries; i++ {
		if err := f(); err != nil {
			log.Printf("Error: %v. Retrying in %d seconds...", err, i*2)
			time.Sleep(time.Duration(i*5) * time.Second)
		} else {
			return nil
		}
	}
	return fmt.Errorf("retries exhausted")
}
