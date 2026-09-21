//go:build integration

package apitest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"os"
	"strings"
	"testing"
)

type Client struct {
	BaseURL string
	HTTP    *http.Client
	Token   string
	User    string
	Pwd     string
}

func NewClient(t *testing.T) *Client {
	t.Helper()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("create cookie jar: %v", err)
	}

	baseURL := strings.TrimRight(os.Getenv("API_BASE_URL"), "/")
	if baseURL == "" {
		baseURL = "http://127.0.0.1:59000"
	}

	user := os.Getenv("API_USER")
	if user == "" {
		user = "admin"
	}
	pwd := os.Getenv("API_PWD")
	if pwd == "" {
		pwd = "123456"
	}

	return &Client{
		BaseURL: baseURL,
		HTTP: &http.Client{
			Jar: jar,
		},
		User: user,
		Pwd:  pwd,
	}
}

func (c *Client) Login(t *testing.T) {
	t.Helper()
/*
	c.DoJSON(
		t,
		http.MethodPost,
		"/api/users/login",
		map[string]any{
			"name": c.User,
			"pwd":  c.Pwd,
		},
		http.StatusOK,
	)
	*/
	resp := c.DoJSON(t, http.MethodPost, "/api/users/login", map[string]any{
		"name": c.User,
		"pwd":  c.Pwd,
	}, http.StatusOK)

	authRaw, ok := resp["auth"].(map[string]any)
	if !ok {
		return
	}

	token, _ := authRaw["token"].(string)
	c.Token = token

}

func (c *Client) DoJSON(
	t *testing.T,
	method string,
	path string,
	body any,
	expectedStatus int,
) map[string]any {
	t.Helper()

	var requestBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal %s %s body: %v", method, path, err)
		}
		requestBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.BaseURL+path, requestBody)
	if err != nil {
		t.Fatalf("create %s %s request: %v", method, path, err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read %s %s response: %v", method, path, err)
	}
	if resp.StatusCode != expectedStatus {
		t.Fatalf(
			"%s %s: expected HTTP %d, got %d\nbody: %s",
			method,
			path,
			expectedStatus,
			resp.StatusCode,
			strings.TrimSpace(string(data)),
		)
	}

	if len(bytes.TrimSpace(data)) == 0 {
		return map[string]any{}
	}

	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && trimmed[0] == '[' {
		var result []any
		if err := json.Unmarshal(data, &result); err != nil {
			t.Fatalf("decode %s %s response as JSON array: %v\nbody: %s", method, path, err, string(data))
		}
		return map[string]any{
			"_array": result,
		}
	}

	result := map[string]any{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("decode %s %s response as JSON object: %v\nbody: %s", method, path, err, string(data))
	}

	return result
}

func (c *Client) DoMultipartFile(
	t *testing.T,
	method string,
	path string,
	fileField string,
	filePath string,
	fields map[string]string,
	wantStatus int,
) map[string]any {
	t.Helper()

	var reqBody bytes.Buffer
	writer := multipart.NewWriter(&reqBody)

	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatalf("multipart WriteField(%q): %v", key, err)
		}
	}

	file, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("os.Open(%q): %v", filePath, err)
	}
	defer file.Close()

	part, err := writer.CreateFormFile(fileField, filepath.Base(filePath))
	if err != nil {
		t.Fatalf("CreateFormFile(%q): %v", fileField, err)
	}
	if _, err := io.Copy(part, file); err != nil {
		t.Fatalf("copy multipart file %q: %v", filePath, err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("multipart Close(): %v", err)
	}

	req, err := http.NewRequest(method, c.BaseURL+path, &reqBody)
	if err != nil {
		t.Fatalf("http.NewRequest(): %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	res, err := c.HTTP.Do(req)
	if err != nil {
		t.Fatalf("%s %s failed: %v", method, path, err)
	}
	defer res.Body.Close()

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("io.ReadAll(): %v", err)
	}

	if res.StatusCode != wantStatus {
		t.Fatalf(
			"%s %s: expected HTTP %d, got %d\nbody: %s",
			method,
			path,
			wantStatus,
			res.StatusCode,
			string(resBody),
		)
	}

	if len(bytes.TrimSpace(resBody)) == 0 {
		return map[string]any{}
	}

	var out map[string]any
	if err := json.Unmarshal(resBody, &out); err != nil {
		t.Fatalf("%s %s: invalid JSON response: %v\nbody: %s", method, path, err, string(resBody))
	}

	return out
}
func (c *Client) DeleteIgnore(t *testing.T, path string) {
	t.Helper()

	req, err := http.NewRequest(http.MethodDelete, c.BaseURL+path, nil)
	if err != nil {
		t.Fatalf("create DELETE %s request: %v", path, err)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		t.Fatalf("DELETE %s: %v", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		data, _ := io.ReadAll(resp.Body)
		t.Fatalf("DELETE %s: unexpected HTTP %d\nbody: %s", path, resp.StatusCode, string(data))
	}
}

func TestID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, os.Getpid())
}

func IntFromJSON(t *testing.T, data map[string]any, key string) int {
	t.Helper()

	value, ok := data[key]
	if !ok {
		t.Fatalf("expected key %q in JSON object", key)
	}

	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	default:
		t.Fatalf("expected %q to be a number, got %T", key, value)
	}

	return 0
}

func StringFromJSON(t *testing.T, data map[string]any, key string) string {
	t.Helper()

	value, ok := data[key]
	if !ok {
		t.Fatalf("expected key %q in JSON object", key)
	}

	str, ok := value.(string)
	if !ok {
		t.Fatalf("expected %q to be a string, got %T", key, value)
	}

	return str
}

func IDKey(t *testing.T, body map[string]any) string {
	t.Helper()

	return fmt.Sprintf("%d", IntFromJSON(t, body, "id"))
}

func (c *Client) DoJSONArray(
	t *testing.T,
	method string,
	path string,
	body any,
	wantStatus int,
) []any {
	t.Helper()

	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("json.Marshal(): %v", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.BaseURL+path, reqBody)
	if err != nil {
		t.Fatalf("http.NewRequest(): %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	res, err := c.HTTP.Do(req)
	if err != nil {
		t.Fatalf("%s %s failed: %v", method, path, err)
	}
	defer res.Body.Close()

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("io.ReadAll(): %v", err)
	}
	if res.StatusCode != wantStatus {
		t.Fatalf(
			"%s %s: expected HTTP %d, got %d\nbody: %s",
			method,
			path,
			wantStatus,
			res.StatusCode,
			string(resBody),
		)
	}

	if len(bytes.TrimSpace(resBody)) == 0 {
		return []any{}
	}
	var out []any
	if err := json.Unmarshal(resBody, &out); err != nil {
		t.Fatalf("%s %s: invalid JSON array response: %v\nbody: %s", method, path, err, string(resBody))
	}
	return out
}
func requireObjectField(t *testing.T, object map[string]any, field string) any {
	t.Helper()

	value, ok := object[field]
	if !ok {
		t.Fatalf("response field %q is missing: %s", field, fmt.Sprint(object))
	}
	return value
}
