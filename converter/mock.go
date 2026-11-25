package converter

import (
	"fmt"
	"sort"
	"strings"
)

type mockOperation struct {
	Path           string
	Method         string
	Tag            string
	Name           string
	Operation      *Operation
	PathParameters []*Parameter
	Query          []*Parameter
	Header         []*Parameter
	Body           interface{}
	HasBody        bool
	Scheme         *SecurityScheme
	Scopes         []string
	Status         string
}

var mockMethodOrder = []OperationMethod{MethodGET, MethodPOST, MethodPUT, MethodPATCH, MethodDELETE, MethodHEAD, MethodOPTIONS, MethodTRACE}

func (n *OpenAPIConverter) getMockOperations() []*mockOperation {
	paths := make([]string, 0, len(n.doc.Paths))
	for path := range n.doc.Paths {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	var operations []*mockOperation
	for _, path := range paths {
		pathItem := n.doc.Paths[path]
		record := pathItem.Operations()

		for _, method := range mockMethodOrder {
			op, ok := record[method]
			if !ok || op == nil {
				continue
			}

			mock := &mockOperation{
				Path:      path,
				Method:    string(method),
				Tag:       "default",
				Name:      string(method) + " " + path,
				Operation: op,
				Status:    "200",
			}

			if op.Tags != nil && len(*op.Tags) > 0 {
				mock.Tag = (*op.Tags)[0]
			}

			if op.Summary != nil && *op.Summary != "" {
				mock.Name = *op.Summary
			}

			seen := map[string]bool{}
			for _, parameter := range append(append([]*Parameter{}, op.Parameters...), pathItem.Parameters...) {
				parameter = n.getMockParameter(parameter)
				if parameter == nil || seen[parameter.Name+"@"+parameter.In] {
					continue
				}
				seen[parameter.Name+"@"+parameter.In] = true

				switch parameter.In {
				case "path":
					mock.PathParameters = append(mock.PathParameters, parameter)
				case "query":
					mock.Query = append(mock.Query, parameter)
				case "header":
					mock.Header = append(mock.Header, parameter)
				}
			}

			mock.Body, mock.HasBody = n.getMockBody(op.RequestBody)
			mock.Scheme, mock.Scopes = n.getMockSecurity(op)

			var statuses []string
			for status := range op.Responses {
				if strings.HasPrefix(status, "2") && len(status) == 3 {
					statuses = append(statuses, status)
				}
			}
			sort.Strings(statuses)
			if len(statuses) > 0 {
				mock.Status = statuses[0]
			}

			operations = append(operations, mock)
		}
	}

	return operations
}

func (n *OpenAPIConverter) getMockParameter(parameter *Parameter) *Parameter {
	if parameter == nil {
		return nil
	}

	if parameter.Ref != nil && n.doc.Components != nil {
		name := (*parameter.Ref)[strings.LastIndex(*parameter.Ref, "/")+1:]
		return n.doc.Components.Parameters[name]
	}

	return parameter
}

func (n *OpenAPIConverter) getMockParameterValue(parameter *Parameter) string {
	if parameter.Example != nil {
		return fmt.Sprint(parameter.Example)
	}

	value := n.getMockValue(parameter.Schema, 0)
	if value == nil {
		return ""
	}

	return fmt.Sprint(value)
}

func (n *OpenAPIConverter) getMockBody(body *RequestBody) (interface{}, bool) {
	if body == nil {
		return nil, false
	}

	content, ok := body.Content["application/json"]
	if !ok || content == nil {
		return nil, false
	}

	if content.Example != nil {
		return content.Example, true
	}

	names := make([]string, 0, len(content.Examples))
	for name := range content.Examples {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if example := content.Examples[name]; example != nil && example.Value != nil {
			return example.Value, true
		}
	}

	value := n.getMockValue(content.Schema, 0)

	return value, value != nil
}

func (n *OpenAPIConverter) getMockSecurity(op *Operation) (*SecurityScheme, []string) {
	requirements := op.Security
	if requirements == nil && n.doc.Security != nil {
		requirements = *n.doc.Security
	}

	if n.doc.Components == nil {
		return nil, nil
	}

	for _, requirement := range requirements {
		names := make([]string, 0, len(requirement))
		for name := range requirement {
			names = append(names, name)
		}
		sort.Strings(names)

		for _, name := range names {
			if scheme, ok := n.doc.Components.SecuritySchemes[name]; ok && scheme != nil {
				return scheme, requirement[name]
			}
		}
	}

	return nil, nil
}

func (n *OpenAPIConverter) getMockValue(schema *Schema, depth int) interface{} {
	if schema == nil || depth > 8 {
		return nil
	}

	if schema.Ref != nil {
		if n.doc.Components == nil {
			return nil
		}
		return n.getMockValue(n.doc.Components.GetSchema(*schema.Ref), depth+1)
	}

	if schema.Example != nil {
		return schema.Example
	}

	if schema.Default != nil {
		return schema.Default
	}

	if len(schema.Enum) > 0 {
		return schema.Enum[0]
	}

	if len(schema.OneOf) > 0 {
		return n.getMockValue(schema.OneOf[0], depth+1)
	}

	if len(schema.AnyOf) > 0 {
		return n.getMockValue(schema.AnyOf[0], depth+1)
	}

	if len(schema.AllOf) > 0 {
		merged := map[string]interface{}{}
		for _, part := range schema.AllOf {
			if value, ok := n.getMockValue(part, depth+1).(map[string]interface{}); ok {
				for key, item := range value {
					merged[key] = item
				}
			}
		}
		for name, property := range schema.Properties {
			merged[name] = n.getMockValue(property, depth+1)
		}
		return merged
	}

	if schema.Type.Is("object") || len(schema.Properties) > 0 {
		object := map[string]interface{}{}
		for name, property := range schema.Properties {
			object[name] = n.getMockValue(property, depth+1)
		}
		return object
	}

	if schema.Type.Is("array") {
		item := n.getMockValue(schema.Items, depth+1)
		if item == nil {
			return []interface{}{}
		}
		return []interface{}{item}
	}

	if schema.Type.Is("integer") {
		if schema.Minimum != nil {
			return schema.Minimum
		}
		return 1
	}

	if schema.Type.Is("number") {
		if schema.Minimum != nil {
			return schema.Minimum
		}
		return 1.5
	}

	if schema.Type.Is("boolean") {
		return true
	}

	if schema.Type.Is("string") {
		format := ""
		if schema.Format != nil {
			format = *schema.Format
		}

		switch format {
		case "uuid":
			return "3fa85f64-5717-4562-b3fc-2c963f66afa6"
		case "email":
			return "user@example.com"
		case "date-time":
			return "2026-01-01T12:00:00Z"
		case "date":
			return "2026-01-01"
		case "uri", "url":
			return "https://example.com"
		}

		return "string"
	}

	return nil
}
