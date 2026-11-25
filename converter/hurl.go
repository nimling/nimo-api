package converter

import (
	"archive/zip"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var hurlPathParameter = regexp.MustCompile(`\{([^}]+)\}`)

func (n *OpenAPIConverter) WriteHurlArchive(outputDir string) error {
	var requests strings.Builder
	var variables strings.Builder

	seen := map[string]bool{}
	putVariable := func(key string, value string) {
		if seen[key] {
			return
		}
		seen[key] = true
		variables.WriteString(key + "=" + value + "\n")
	}

	putVariable("base_url", strings.TrimSuffix(n.doc.Servers[0].URL, "/"))

	operations := n.getMockOperations()

	for _, op := range operations {
		if op.Scheme == nil || op.Scheme.Type != "oauth2" || op.Scheme.Flows == nil || op.Scheme.Flows.ClientCredentials == nil {
			continue
		}

		flow := op.Scheme.Flows.ClientCredentials
		putVariable("token_url", flow.TokenURL)
		putVariable("client_id", "")
		putVariable("client_secret", "")

		scopes := make([]string, 0, len(flow.Scopes))
		for scope := range flow.Scopes {
			scopes = append(scopes, scope)
		}
		sort.Strings(scopes)

		requests.WriteString("# Access token\n")
		requests.WriteString("POST {{token_url}}\n")
		requests.WriteString("[FormParams]\n")
		requests.WriteString("grant_type: client_credentials\n")
		requests.WriteString("client_id: {{client_id}}\n")
		requests.WriteString("client_secret: {{client_secret}}\n")
		if len(scopes) > 0 {
			requests.WriteString("scope: " + strings.Join(scopes, " ") + "\n")
		}
		requests.WriteString("\nHTTP 200\n")
		requests.WriteString("[Captures]\n")
		requests.WriteString("token: jsonpath \"$.access_token\"\n\n\n")
		seen["token"] = true
		break
	}

	for _, op := range operations {
		for _, parameter := range op.PathParameters {
			putVariable(parameter.Name, n.getMockParameterValue(parameter))
		}

		requests.WriteString("# " + op.Name + "\n")
		requests.WriteString(op.Method + " {{base_url}}" + hurlPathParameter.ReplaceAllString(op.Path, "{{$1}}") + "\n")

		for _, parameter := range op.Header {
			requests.WriteString(parameter.Name + ": " + n.getMockParameterValue(parameter) + "\n")
		}

		basic := false
		if op.Scheme != nil {
			switch {
			case op.Scheme.Type == "apiKey" && op.Scheme.In == "header":
				putVariable("api_key", "")
				requests.WriteString(op.Scheme.Name + ": {{api_key}}\n")
			case op.Scheme.Type == "apiKey":
			case op.Scheme.Type == "http" && op.Scheme.Scheme == "basic":
				putVariable("username", "")
				putVariable("password", "")
				basic = true
			default:
				putVariable("token", "")
				requests.WriteString("Authorization: Bearer {{token}}\n")
			}
		}

		if op.HasBody {
			requests.WriteString("Content-Type: application/json\n")
		}

		var query []string
		for _, parameter := range op.Query {
			if parameter.Required {
				query = append(query, parameter.Name+": "+n.getMockParameterValue(parameter))
			}
		}
		if op.Scheme != nil && op.Scheme.Type == "apiKey" && op.Scheme.In == "query" {
			putVariable("api_key", "")
			query = append(query, op.Scheme.Name+": {{api_key}}")
		}
		if len(query) > 0 {
			requests.WriteString("[QueryStringParams]\n" + strings.Join(query, "\n") + "\n")
		}

		if basic {
			requests.WriteString("[BasicAuth]\n{{username}}: {{password}}\n")
		}

		if op.HasBody {
			raw, err := getMockJSON(op.Body)
			if err != nil {
				return fmt.Errorf("failed to write the body of %s %s: %w", op.Method, op.Path, err)
			}
			requests.WriteString(raw + "\n")
		}

		requests.WriteString("\nHTTP " + op.Status + "\n\n\n")
	}

	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)

	files := []struct {
		Name    string
		Content string
	}{
		{Name: "api.hurl", Content: strings.TrimRight(requests.String(), "\n") + "\n"},
		{Name: "variables.env", Content: variables.String()},
	}

	for _, file := range files {
		entry, err := writer.CreateHeader(&zip.FileHeader{Name: file.Name, Method: zip.Deflate})
		if err != nil {
			return fmt.Errorf("failed to add %s to the hurl archive: %w", file.Name, err)
		}
		if _, err := entry.Write([]byte(file.Content)); err != nil {
			return fmt.Errorf("failed to write %s to the hurl archive: %w", file.Name, err)
		}
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to close the hurl archive: %w", err)
	}

	outputPath := filepath.Join(outputDir, fmt.Sprintf("%shurl.zip", n.FilePrefix))
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create the hurl output directory: %w", err)
	}

	if err := os.WriteFile(outputPath, archive.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write the hurl archive: %w", err)
	}
	LogWrite(outputPath, archive.Len())

	fmt.Printf("Successfully wrote hurl archive to %s\n", outputPath)
	return nil
}
