package test

import (
	"archive/zip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nimling/nimo-api/internal"
)

func runCollectionsConvert(t *testing.T, spec string, dir string) {
	t.Helper()

	cmd := internal.NewConvertCommand()
	cmd.SetArgs([]string{
		spec,
		"-o", dir,
		"--inline",
		"--postman-output", dir,
		"--hurl-output", dir,
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("convert failed: %v", err)
	}
}

func readHurlArchive(t *testing.T, path string) map[string]string {
	t.Helper()

	archive, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("hurl archive was not written: %v", err)
	}
	defer archive.Close()

	files := map[string]string{}
	for _, file := range archive.File {
		reader, err := file.Open()
		if err != nil {
			t.Fatalf("failed to open %s: %v", file.Name, err)
		}
		content, err := io.ReadAll(reader)
		reader.Close()
		if err != nil {
			t.Fatalf("failed to read %s: %v", file.Name, err)
		}
		files[file.Name] = string(content)
	}

	return files
}

func TestConvertKeepsOAuthFlows(t *testing.T) {
	dir := "../../tmp/test-collections-flows"
	defer os.RemoveAll("../../tmp")

	runCollectionsConvert(t, "../examples/security_spec.yml", dir)

	data, err := os.ReadFile(filepath.Join(dir, "spec.json"))
	if err != nil {
		t.Fatalf("spec.json was not written: %v", err)
	}

	var spec struct {
		Components struct {
			SecuritySchemes map[string]struct {
				Flows struct {
					ClientCredentials struct {
						TokenURL string            `json:"tokenUrl"`
						Scopes   map[string]string `json:"scopes"`
					} `json:"clientCredentials"`
				} `json:"flows"`
			} `json:"securitySchemes"`
		} `json:"components"`
	}
	if err := json.Unmarshal(data, &spec); err != nil {
		t.Fatalf("spec.json is not JSON: %v", err)
	}

	flow := spec.Components.SecuritySchemes["oauth2"].Flows.ClientCredentials
	if flow.TokenURL != "https://login.example.com/oauth/token" {
		t.Errorf("tokenUrl = %q, the flow was not carried into spec.json", flow.TokenURL)
	}
	if len(flow.Scopes) != 2 {
		t.Errorf("scopes = %v, expected the two scopes of the flow", flow.Scopes)
	}
}

func TestConvertWritesPostmanCollection(t *testing.T) {
	dir := "../../tmp/test-collections-postman"
	defer os.RemoveAll("../../tmp")

	runCollectionsConvert(t, "../examples/security_spec.yml", dir)

	data, err := os.ReadFile(filepath.Join(dir, "postman.json"))
	if err != nil {
		t.Fatalf("postman.json was not written: %v", err)
	}

	var collection struct {
		Info struct {
			Name   string `json:"name"`
			Schema string `json:"schema"`
		} `json:"info"`
		Variable []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		} `json:"variable"`
		Item []struct {
			Name string `json:"name"`
			Item []struct {
				Name    string `json:"name"`
				Request struct {
					Method string `json:"method"`
					URL    struct {
						Raw string `json:"raw"`
					} `json:"url"`
					Body struct {
						Raw string `json:"raw"`
					} `json:"body"`
					Auth struct {
						Type string `json:"type"`
					} `json:"auth"`
				} `json:"request"`
			} `json:"item"`
		} `json:"item"`
	}
	if err := json.Unmarshal(data, &collection); err != nil {
		t.Fatalf("postman.json is not JSON: %v", err)
	}

	if collection.Info.Schema != "https://schema.getpostman.com/json/collection/v2.1.0/collection.json" {
		t.Errorf("schema = %q, expected the v2.1 collection schema", collection.Info.Schema)
	}
	if collection.Info.Name != "Security API" {
		t.Errorf("name = %q", collection.Info.Name)
	}

	variables := map[string]string{}
	for _, variable := range collection.Variable {
		variables[variable.Key] = variable.Value
	}
	if variables["baseUrl"] != "https://api.example.com/v1" {
		t.Errorf("baseUrl = %q", variables["baseUrl"])
	}
	if variables["tokenUrl"] != "https://login.example.com/oauth/token" {
		t.Errorf("tokenUrl = %q", variables["tokenUrl"])
	}
	if _, ok := variables["audience"]; !ok {
		t.Errorf("the audience variable is missing: %v", variables)
	}

	requests := map[string]string{}
	auth := map[string]string{}
	bodies := map[string]string{}
	for _, folder := range collection.Item {
		for _, item := range folder.Item {
			key := item.Request.Method + " " + item.Request.URL.Raw
			requests[key] = folder.Name
			auth[key] = item.Request.Auth.Type
			bodies[key] = item.Request.Body.Raw
		}
	}

	if len(requests) != 3 {
		t.Fatalf("requests = %v, expected one per operation", requests)
	}
	if requests["GET {{baseUrl}}/things/:thingId"] != "Things" {
		t.Errorf("the path parameter request is missing or in the wrong folder: %v", requests)
	}
	if auth["GET {{baseUrl}}/things/:thingId"] != "oauth2" {
		t.Errorf("auth = %q, expected oauth2", auth["GET {{baseUrl}}/things/:thingId"])
	}
	if auth["POST {{baseUrl}}/reports"] != "apikey" {
		t.Errorf("auth = %q, expected apikey", auth["POST {{baseUrl}}/reports"])
	}

	var body map[string]interface{}
	if err := json.Unmarshal([]byte(bodies["POST {{baseUrl}}/things"]), &body); err != nil {
		t.Fatalf("the generated body is not JSON: %v", err)
	}
	if body["email"] != "user@example.com" || body["kind"] != "small" || body["owner"] != "owner-1" || body["name"] != "string" {
		t.Errorf("body = %v, expected values from format, enum, example and type", body)
	}
	if !strings.Contains(bodies["POST {{baseUrl}}/reports"], "report-1") {
		t.Errorf("body = %q, expected the example of the request body", bodies["POST {{baseUrl}}/reports"])
	}
}

func TestConvertWritesHurlArchive(t *testing.T) {
	dir := "../../tmp/test-collections-hurl"
	defer os.RemoveAll("../../tmp")

	runCollectionsConvert(t, "../examples/security_spec.yml", dir)

	files := readHurlArchive(t, filepath.Join(dir, "hurl.zip"))
	if len(files) != 2 {
		t.Fatalf("files = %d, expected api.hurl and variables.env", len(files))
	}

	requests := files["api.hurl"]
	for _, expected := range []string{
		"POST {{token_url}}\n[FormParams]\ngrant_type: client_credentials\nclient_id: {{client_id}}\nclient_secret: {{client_secret}}\naudience: {{audience}}\nscope: things:read things:write\n",
		"token: jsonpath \"$.access_token\"",
		"GET {{base_url}}/things/{{thingId}}\nAuthorization: Bearer {{token}}\n\nHTTP 200",
		"POST {{base_url}}/things\nAuthorization: Bearer {{token}}\nContent-Type: application/json\n{",
		"HTTP 201",
		"POST {{base_url}}/reports\nX-Report-Key: {{api_key}}\nContent-Type: application/json\n",
	} {
		if !strings.Contains(requests, expected) {
			t.Errorf("api.hurl is missing %q in:\n%s", expected, requests)
		}
	}

	variables := files["variables.env"]
	for _, expected := range []string{
		"base_url=https://api.example.com/v1\n",
		"token_url=https://login.example.com/oauth/token\n",
		"client_id=\n",
		"client_secret=\n",
		"audience=\n",
		"thingId=3fa85f64-5717-4562-b3fc-2c963f66afa6\n",
		"api_key=\n",
	} {
		if !strings.Contains(variables, expected) {
			t.Errorf("variables.env is missing %q in:\n%s", expected, variables)
		}
	}
}

func TestConvertWritesNoCollectionsWithoutFlags(t *testing.T) {
	dir := "../../tmp/test-collections-none"
	defer os.RemoveAll("../../tmp")

	cmd := internal.NewConvertCommand()
	cmd.SetArgs([]string{"../examples/security_spec.yml", "-o", dir, "--inline"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("convert failed: %v", err)
	}

	for _, name := range []string{"postman.json", "hurl.zip"} {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Errorf("%s was written without its flag", name)
		}
	}
}
