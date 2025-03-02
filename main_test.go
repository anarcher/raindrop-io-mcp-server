package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestNewRaindropClient(t *testing.T) {
	// Save original env var and restore after test
	originalToken := os.Getenv("RAINDROP_TOKEN")
	defer os.Setenv("RAINDROP_TOKEN", originalToken)

	// Test when token is not set
	os.Setenv("RAINDROP_TOKEN", "")
	client, err := NewRaindropClient()
	if err == nil {
		t.Error("Expected error when token is not set, got nil")
	}
	if client != nil {
		t.Error("Expected nil client when token is not set")
	}

	// Test when token is set
	os.Setenv("RAINDROP_TOKEN", "test-token")
	client, err = NewRaindropClient()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if client == nil {
		t.Error("Expected client to be created")
	}
	if client.Token != "test-token" {
		t.Errorf("Expected token to be 'test-token', got '%s'", client.Token)
	}
}

func TestMakeRequest(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check authorization header
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("Expected Authorization header with token, got: %s", r.Header.Get("Authorization"))
		}

		// Check content type
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type header to be application/json, got: %s", r.Header.Get("Content-Type"))
		}

		// Test different endpoints and methods
		switch {
		case r.URL.Path == "/rest/v1/test" && r.Method == "GET":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status": "success", "method": "GET"}`))
		case r.URL.Path == "/rest/v1/test" && r.Method == "POST":
			// Read request body
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("Error reading request body: %v", err)
			}
			
			// Check if body contains expected data
			if !strings.Contains(string(body), `"test":"data"`) {
				t.Errorf("Expected request body to contain test data, got: %s", string(body))
			}
			
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status": "success", "method": "POST"}`))
		case r.URL.Path == "/rest/v1/error":
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error": "test error"}`))
		default:
			t.Errorf("Unexpected request to %s with method %s", r.URL.Path, r.Method)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Use the test server URL instead of the real API
	testAPIBase := server.URL + "/rest/v1"

	client := &RaindropClient{Token: "test-token"}

	// Create custom makeRequest function for testing
	makeTestRequest := func(endpoint string, method string, body interface{}) (map[string]interface{}, error) {
		url := testAPIBase + endpoint
		
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
	
		req.Header.Set("Authorization", "Bearer "+client.Token)
		req.Header.Set("Content-Type", "application/json")
	
		httpClient := &http.Client{}
		resp, err := httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
	
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("API error: %s", resp.Status)
		}
	
		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		if err != nil {
			return nil, err
		}
	
		return result, nil
	}
	
	// Test GET request
	result, err := makeTestRequest("/test", "GET", nil)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result["status"] != "success" || result["method"] != "GET" {
		t.Errorf("Unexpected result: %v", result)
	}

	// Test POST request with body
	body := map[string]string{"test": "data"}
	result, err = makeTestRequest("/test", "POST", body)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result["status"] != "success" || result["method"] != "POST" {
		t.Errorf("Unexpected result: %v", result)
	}

	// Test error response
	_, err = makeTestRequest("/error", "GET", nil)
	if err == nil {
		t.Error("Expected error for error response, got nil")
	}
}

func TestCreateJSONSchema(t *testing.T) {
	schema := createJSONSchema()
	
	// Check if schema contains the expected tools
	if _, ok := schema["create-bookmark"]; !ok {
		t.Error("Expected schema to contain 'create-bookmark'")
	}
	
	if _, ok := schema["search-bookmarks"]; !ok {
		t.Error("Expected schema to contain 'search-bookmarks'")
	}
	
	// Check create-bookmark schema properties
	createSchema, ok := schema["create-bookmark"].(map[string]interface{})
	if !ok {
		t.Error("Expected create-bookmark schema to be a map")
	} else {
		properties, ok := createSchema["properties"].(map[string]interface{})
		if !ok {
			t.Error("Expected create-bookmark schema to have properties")
		} else {
			if _, ok := properties["url"]; !ok {
				t.Error("Expected create-bookmark schema to have url property")
			}
			if _, ok := properties["title"]; !ok {
				t.Error("Expected create-bookmark schema to have title property")
			}
			if _, ok := properties["tags"]; !ok {
				t.Error("Expected create-bookmark schema to have tags property")
			}
			if _, ok := properties["collection"]; !ok {
				t.Error("Expected create-bookmark schema to have collection property")
			}
		}
		
		required, ok := createSchema["required"].([]string)
		if !ok {
			t.Error("Expected create-bookmark schema to have required fields")
		} else if len(required) != 1 || required[0] != "url" {
			t.Errorf("Expected create-bookmark schema to require 'url', got %v", required)
		}
	}
}

func TestMustMarshal(t *testing.T) {
	// Test with valid data
	data := map[string]string{"test": "data"}
	result := mustMarshal(data)
	expected := []byte(`{"test":"data"}`)
	
	if !bytes.Equal(result, expected) {
		t.Errorf("Expected %s, got %s", expected, result)
	}
	
	// Cannot test the panic case easily in a unit test
}

// Test for Request/Response types
func TestRequestResponseTypes(t *testing.T) {
	// Test Request unmarshaling
	reqJSON := `{"jsonrpc":"2.0","id":1,"method":"test.method","params":{"key":"value"}}`
	var req Request
	if err := json.Unmarshal([]byte(reqJSON), &req); err != nil {
		t.Errorf("Failed to unmarshal Request: %v", err)
	}
	
	if req.JSONRPC != "2.0" || req.ID != 1 || req.Method != "test.method" {
		t.Errorf("Request unmarshal incorrect, got: %+v", req)
	}
	
	// Test Response marshaling
	resp := Response{
		JSONRPC: "2.0",
		ID:      1,
		Result:  json.RawMessage(`{"success":true}`),
	}
	
	data, err := json.Marshal(resp)
	if err != nil {
		t.Errorf("Failed to marshal Response: %v", err)
	}
	
	expected := `{"jsonrpc":"2.0","id":1,"result":{"success":true}}`
	if string(data) != expected {
		t.Errorf("Response marshal incorrect,\nexpected: %s\ngot: %s", expected, string(data))
	}
}