package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

const RaindropAPIBase = "https://api.raindrop.io/rest/v1"

// MCP Types
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *ErrorResponse  `json:"error,omitempty"`
}

type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Tool Types
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

type ListToolsResponse struct {
	Tools []Tool `json:"tools"`
}

type CallToolRequest struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type CallToolResponse struct {
	Content []ContentBlock `json:"content"`
}

// Raindrop Types
type CreateBookmarkArgs struct {
	URL        string   `json:"url"`
	Title      string   `json:"title,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	Collection int      `json:"collection,omitempty"`
}

type SearchBookmarksArgs struct {
	Query string   `json:"query"`
	Tags  []string `json:"tags,omitempty"`
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
	
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest(method, url, reqBody)
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

// JSON Schema helpers
func createJSONSchema() map[string]interface{} {
	createBookmarkSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"url": map[string]interface{}{
				"type":        "string",
				"description": "URL to bookmark",
			},
			"title": map[string]interface{}{
				"type":        "string",
				"description": "Title for the bookmark (optional)",
			},
			"tags": map[string]interface{}{
				"type":        "array",
				"description": "Tags for the bookmark (optional)",
				"items": map[string]interface{}{
					"type": "string",
				},
			},
			"collection": map[string]interface{}{
				"type":        "number",
				"description": "Collection ID to save to (optional)",
			},
		},
		"required": []string{"url"},
	}

	searchBookmarksSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "Search query",
			},
			"tags": map[string]interface{}{
				"type":        "array",
				"description": "Filter by tags (optional)",
				"items": map[string]interface{}{
					"type": "string",
				},
			},
		},
		"required": []string{"query"},
	}

	return map[string]interface{}{
		"create-bookmark":  createBookmarkSchema,
		"search-bookmarks": searchBookmarksSchema,
	}
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

	// Create JSON schema for tools
	schemas := createJSONSchema()

	// Set up stdin/stdout for MCP communication
	scanner := bufio.NewScanner(os.Stdin)
	writer := json.NewEncoder(os.Stdout)

	// Main loop to process requests
	for scanner.Scan() {
		line := scanner.Text()
		var req Request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			log.Printf("Error parsing request: %v", err)
			continue
		}

		var resp Response
		resp.JSONRPC = "2.0"
		resp.ID = req.ID

		switch req.Method {
		case "mcp.listTools":
			tools := []Tool{
				{
					Name:        "create-bookmark",
					Description: "Create a new bookmark in Raindrop.io",
					InputSchema: mustMarshal(schemas["create-bookmark"]),
				},
				{
					Name:        "search-bookmarks",
					Description: "Search through your Raindrop.io bookmarks",
					InputSchema: mustMarshal(schemas["search-bookmarks"]),
				},
			}

			listResp := ListToolsResponse{Tools: tools}
			resp.Result = mustMarshal(listResp)

		case "mcp.callTool":
			var callReq CallToolRequest
			if err := json.Unmarshal(req.Params, &callReq); err != nil {
				resp.Error = &ErrorResponse{
					Code:    -32602,
					Message: fmt.Sprintf("Invalid params: %v", err),
				}
				break
			}

			switch callReq.Name {
			case "create-bookmark":
				var createArgs CreateBookmarkArgs
				if err := json.Unmarshal(callReq.Arguments, &createArgs); err != nil {
					resp.Error = &ErrorResponse{
						Code:    -32602,
						Message: fmt.Sprintf("Invalid arguments: %v", err),
					}
					break
				}

				if createArgs.URL == "" {
					resp.Error = &ErrorResponse{
						Code:    -32602,
						Message: "URL is required",
					}
					break
				}

				// Prepare the request body
				body := map[string]interface{}{
					"link":  createArgs.URL,
					"title": createArgs.Title,
					"tags":  createArgs.Tags,
				}

				if createArgs.Collection != 0 {
					body["collection"] = map[string]interface{}{"$id": createArgs.Collection}
				} else {
					body["collection"] = map[string]interface{}{"$id": 0}
				}

				bookmark, err := raindropClient.MakeRequest("/raindrop", "POST", body)
				if err != nil {
					resp.Error = &ErrorResponse{
						Code:    -32603,
						Message: fmt.Sprintf("Internal error: %v", err),
					}
					break
				}

				callResp := CallToolResponse{
					Content: []ContentBlock{
						{
							Type: "text",
							Text: fmt.Sprintf("Bookmark created successfully: %s", bookmark["link"]),
						},
					},
				}
				resp.Result = mustMarshal(callResp)

			case "search-bookmarks":
				var searchArgs SearchBookmarksArgs
				if err := json.Unmarshal(callReq.Arguments, &searchArgs); err != nil {
					resp.Error = &ErrorResponse{
						Code:    -32602,
						Message: fmt.Sprintf("Invalid arguments: %v", err),
					}
					break
				}

				if searchArgs.Query == "" {
					resp.Error = &ErrorResponse{
						Code:    -32602,
						Message: "Query is required",
					}
					break
				}

				// Build query parameters
				params := url.Values{}
				params.Add("search", searchArgs.Query)
				if len(searchArgs.Tags) > 0 {
					params.Add("tags", strings.Join(searchArgs.Tags, ","))
				}

				endpoint := fmt.Sprintf("/raindrops/0?%s", params.Encode())
				results, err := raindropClient.MakeRequest(endpoint, "GET", nil)
				if err != nil {
					resp.Error = &ErrorResponse{
						Code:    -32603,
						Message: fmt.Sprintf("Internal error: %v", err),
					}
					break
				}

				items, ok := results["items"].([]interface{})
				if !ok {
					resp.Error = &ErrorResponse{
						Code:    -32603,
						Message: "Unable to parse results",
					}
					break
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

				callResp := CallToolResponse{
					Content: []ContentBlock{
						{
							Type: "text",
							Text: responseText,
						},
					},
				}
				resp.Result = mustMarshal(callResp)

			default:
				resp.Error = &ErrorResponse{
					Code:    -32601,
					Message: fmt.Sprintf("Unknown tool: %s", callReq.Name),
				}
			}

		default:
			resp.Error = &ErrorResponse{
				Code:    -32601,
				Message: fmt.Sprintf("Method not found: %s", req.Method),
			}
		}

		if err := writer.Encode(resp); err != nil {
			log.Printf("Error encoding response: %v", err)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("Error reading input: %v", err)
	}
}

// Helper function to marshal JSON without error checking
func mustMarshal(v interface{}) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}