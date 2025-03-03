package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/joho/godotenv"
	mcp "github.com/metoro-io/mcp-golang"
	"github.com/metoro-io/mcp-golang/transport/stdio"
)

const RaindropAPIBase = "https://api.raindrop.io/rest/v1"

// Raindrop Types
type CreateBookmarkArgs struct {
	URL        string   `json:"url" jsonschema:"required,description=URL to bookmark"`
	Title      string   `json:"title,omitempty" jsonschema:"description=Title for the bookmark"`
	Tags       []string `json:"tags,omitempty" jsonschema:"description=Array of tags"`
	Collection int      `json:"collection,omitempty" jsonschema:"description=Collection ID"`
}

type SearchBookmarksArgs struct {
	Query string   `json:"query" jsonschema:"required,description=Search query"`
	Tags  []string `json:"tags,omitempty" jsonschema:"description=Array of tags to filter by"`
}

// RaindropAPI client
type RaindropClient struct {
	Token string
}

func NewRaindropClient() (*RaindropClient, error) {
	token := os.Getenv("RAINDROP_TOKEN")
	if token == "" {
		return nil, errors.New("RAINDROP_TOKEN is not set")
	}
	return &RaindropClient{Token: token}, nil
}

func (r *RaindropClient) MakeRequest(endpoint string, method string, body interface{}) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s%s", RaindropAPIBase, endpoint)

	var reqBody []byte
	var err error
	if body != nil {
		reqBody, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
	}

	req, err := http.NewRequest(method, url, strings.NewReader(string(reqBody)))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+r.Token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Raindrop API error: %s", resp.Status)
	}

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func main() {
	// Set up logging
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.SetOutput(os.Stderr)

	// Load environment variables
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: .env file not found")
	}

	// Create a new raindrop client
	raindropClient, err := NewRaindropClient()
	if err != nil {
		log.Fatalf("Failed to create Raindrop client: %v", err)
	}

	// Create a new MCP server
	server := mcp.NewServer(stdio.NewStdioServerTransport(), mcp.WithName("Raindrop.io MCP Server"))

	// Register tools
	err = server.RegisterTool("create-bookmark", "Create a new bookmark in Raindrop.io",
		func(args CreateBookmarkArgs) (*mcp.ToolResponse, error) {
			if args.URL == "" {
				return nil, fmt.Errorf("URL is required")
			}

			// Prepare the request body
			body := map[string]interface{}{
				"link":  args.URL,
				"title": args.Title,
				"tags":  args.Tags,
			}

			if args.Collection != 0 {
				body["collection"] = map[string]interface{}{"$id": args.Collection}
			} else {
				body["collection"] = map[string]interface{}{"$id": 0}
			}

			bookmark, err := raindropClient.MakeRequest("/raindrop", "POST", body)
			if err != nil {
				return nil, fmt.Errorf("internal error: %v", err)
			}

			return mcp.NewToolResponse(
				mcp.NewTextContent(fmt.Sprintf("Bookmark created successfully: %s", bookmark["link"])),
			), nil
		})
	if err != nil {
		log.Fatalf("Failed to register create-bookmark tool: %v", err)
	}

	err = server.RegisterTool("search-bookmarks", "Search through your Raindrop.io bookmarks",
		func(args SearchBookmarksArgs) (*mcp.ToolResponse, error) {
			if args.Query == "" {
				return nil, fmt.Errorf("query is required")
			}

			// Build query parameters
			params := url.Values{}
			params.Add("search", args.Query)
			if len(args.Tags) > 0 {
				params.Add("tags", strings.Join(args.Tags, ","))
			}

			endpoint := fmt.Sprintf("/raindrops/0?%s", params.Encode())
			results, err := raindropClient.MakeRequest(endpoint, "GET", nil)
			if err != nil {
				return nil, fmt.Errorf("internal error: %v", err)
			}

			items, ok := results["items"].([]interface{})
			if !ok {
				return nil, fmt.Errorf("unable to parse results")
			}

			var formattedResults strings.Builder
			for _, item := range items {
				bookmark, ok := item.(map[string]interface{})
				if !ok {
					continue
				}

				title, _ := bookmark["title"].(string)
				link, _ := bookmark["link"].(string)

				// Extract tags
				tagList := []string{}
				if tags, ok := bookmark["tags"].([]interface{}); ok {
					for _, t := range tags {
						if tag, ok := t.(string); ok {
							tagList = append(tagList, tag)
						}
					}
				}

				tagsStr := "No tags"
				if len(tagList) > 0 {
					tagsStr = strings.Join(tagList, ", ")
				}

				formattedResults.WriteString(fmt.Sprintf("\nTitle: %s\nURL: %s\nTags: %s\n---", title, link, tagsStr))
			}

			var responseText string
			if len(items) > 0 {
				responseText = fmt.Sprintf("Found %d bookmarks:%s", len(items), formattedResults.String())
			} else {
				responseText = "No bookmarks found matching your search."
			}

			return mcp.NewToolResponse(
				mcp.NewTextContent(responseText),
			), nil
		})
	if err != nil {
		log.Fatalf("Failed to register search-bookmarks tool: %v", err)
	}

	// Start the server
	if err := server.Serve(); err != nil {
		log.Fatalf("Server error: %v", err)
	}

	done := make(chan struct{})
	<-done
}
