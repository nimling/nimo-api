package converter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type postmanVariable struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Type  string `json:"type,omitempty"`
}

type postmanQuery struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	Disabled bool   `json:"disabled,omitempty"`
}

type postmanURL struct {
	Raw      string            `json:"raw"`
	Host     []string          `json:"host"`
	Path     []string          `json:"path"`
	Query    []postmanQuery    `json:"query,omitempty"`
	Variable []postmanVariable `json:"variable,omitempty"`
}

type postmanBody struct {
	Mode    string                 `json:"mode"`
	Raw     string                 `json:"raw"`
	Options map[string]interface{} `json:"options"`
}

type postmanRequest struct {
	Method      string                 `json:"method"`
	Header      []postmanVariable      `json:"header"`
	URL         postmanURL             `json:"url"`
	Body        *postmanBody           `json:"body,omitempty"`
	Auth        map[string]interface{} `json:"auth"`
	Description string                 `json:"description,omitempty"`
}

type postmanItem struct {
	Name    string          `json:"name"`
	Request *postmanRequest `json:"request,omitempty"`
	Item    []*postmanItem  `json:"item,omitempty"`
}

type postmanCollection struct {
	Info struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Schema      string `json:"schema"`
	} `json:"info"`
	Item     []*postmanItem    `json:"item"`
	Variable []postmanVariable `json:"variable"`
}

func (n *OpenAPIConverter) WritePostmanCollection(outputDir string) error {
	collection := postmanCollection{}
	collection.Info.Name = n.doc.Info.Title
	collection.Info.Description = n.doc.Info.Description
	collection.Info.Schema = "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"
	collection.Variable = []postmanVariable{
		{Key: "baseUrl", Value: strings.TrimSuffix(n.doc.Servers[0].URL, "/")},
	}

	variables := map[string]bool{"baseUrl": true}
	putVariable := func(key string, value string) {
		if variables[key] {
			return
		}
		variables[key] = true
		collection.Variable = append(collection.Variable, postmanVariable{Key: key, Value: value})
	}

	folders := map[string]*postmanItem{}

	for _, op := range n.getMockOperations() {
		folder, ok := folders[op.Tag]
		if !ok {
			folder = &postmanItem{Name: op.Tag}
			folders[op.Tag] = folder
			collection.Item = append(collection.Item, folder)
		}

		request := &postmanRequest{
			Method: op.Method,
			Header: []postmanVariable{},
		}

		if op.Operation.Description != nil {
			request.Description = *op.Operation.Description
		}

		segments := []string{}
		for _, segment := range strings.Split(strings.Trim(op.Path, "/"), "/") {
			if strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}") {
				segment = ":" + strings.Trim(segment, "{}")
			}
			segments = append(segments, segment)
		}

		request.URL = postmanURL{
			Raw:  "{{baseUrl}}/" + strings.Join(segments, "/"),
			Host: []string{"{{baseUrl}}"},
			Path: segments,
		}

		for _, parameter := range op.PathParameters {
			request.URL.Variable = append(request.URL.Variable, postmanVariable{
				Key:   parameter.Name,
				Value: n.getMockParameterValue(parameter),
			})
		}

		var query []string
		for _, parameter := range op.Query {
			value := n.getMockParameterValue(parameter)
			request.URL.Query = append(request.URL.Query, postmanQuery{
				Key:      parameter.Name,
				Value:    value,
				Disabled: !parameter.Required,
			})
			if parameter.Required {
				query = append(query, parameter.Name+"="+value)
			}
		}
		if len(query) > 0 {
			request.URL.Raw += "?" + strings.Join(query, "&")
		}

		for _, parameter := range op.Header {
			request.Header = append(request.Header, postmanVariable{
				Key:   parameter.Name,
				Value: n.getMockParameterValue(parameter),
			})
		}

		if op.HasBody {
			raw, err := getMockJSON(op.Body)
			if err != nil {
				return fmt.Errorf("failed to write the body of %s %s: %w", op.Method, op.Path, err)
			}

			request.Header = append(request.Header, postmanVariable{Key: "Content-Type", Value: "application/json"})
			request.Body = &postmanBody{
				Mode:    "raw",
				Raw:     raw,
				Options: map[string]interface{}{"raw": map[string]string{"language": "json"}},
			}
		}

		request.Auth = map[string]interface{}{"type": "noauth"}
		if op.Scheme != nil {
			switch {
			case op.Scheme.Type == "oauth2" && op.Scheme.Flows != nil && op.Scheme.Flows.ClientCredentials != nil:
				putVariable("tokenUrl", op.Scheme.Flows.ClientCredentials.TokenURL)
				putVariable("clientId", "")
				putVariable("clientSecret", "")
				request.Auth = map[string]interface{}{
					"type": "oauth2",
					"oauth2": []postmanVariable{
						{Key: "grant_type", Value: "client_credentials", Type: "string"},
						{Key: "accessTokenUrl", Value: "{{tokenUrl}}", Type: "string"},
						{Key: "clientId", Value: "{{clientId}}", Type: "string"},
						{Key: "clientSecret", Value: "{{clientSecret}}", Type: "string"},
						{Key: "scope", Value: strings.Join(op.Scopes, " "), Type: "string"},
						{Key: "addTokenTo", Value: "header", Type: "string"},
					},
				}
			case op.Scheme.Type == "apiKey":
				putVariable("apiKey", "")
				request.Auth = map[string]interface{}{
					"type": "apikey",
					"apikey": []postmanVariable{
						{Key: "key", Value: op.Scheme.Name, Type: "string"},
						{Key: "value", Value: "{{apiKey}}", Type: "string"},
						{Key: "in", Value: op.Scheme.In, Type: "string"},
					},
				}
			case op.Scheme.Type == "http" && op.Scheme.Scheme == "basic":
				putVariable("username", "")
				putVariable("password", "")
				request.Auth = map[string]interface{}{
					"type": "basic",
					"basic": []postmanVariable{
						{Key: "username", Value: "{{username}}", Type: "string"},
						{Key: "password", Value: "{{password}}", Type: "string"},
					},
				}
			default:
				putVariable("token", "")
				request.Auth = map[string]interface{}{
					"type": "bearer",
					"bearer": []postmanVariable{
						{Key: "token", Value: "{{token}}", Type: "string"},
					},
				}
			}
		}

		folder.Item = append(folder.Item, &postmanItem{Name: op.Name, Request: request})
	}

	data, err := getMockJSON(collection)
	if err != nil {
		return fmt.Errorf("failed to marshal the Postman collection: %w", err)
	}

	outputPath := filepath.Join(outputDir, fmt.Sprintf("%spostman.json", n.FilePrefix))
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create the Postman output directory: %w", err)
	}

	if err := os.WriteFile(outputPath, []byte(data), 0644); err != nil {
		return fmt.Errorf("failed to write the Postman collection: %w", err)
	}
	LogWrite(outputPath, len(data))

	fmt.Printf("Successfully wrote Postman collection to %s\n", outputPath)
	return nil
}

func getMockJSON(value interface{}) (string, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return "", err
	}

	return strings.TrimRight(buffer.String(), "\n"), nil
}
