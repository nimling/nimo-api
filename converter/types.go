package converter

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

const (
	MethodGET     OperationMethod = "GET"
	MethodDELETE  OperationMethod = "DELETE"
	MethodPUT     OperationMethod = "PUT"
	MethodPOST    OperationMethod = "POST"
	MethodPATCH   OperationMethod = "PATCH"
	MethodOPTIONS OperationMethod = "OPTIONS"
	MethodHEAD    OperationMethod = "HEAD"
	MethodTRACE   OperationMethod = "TRACE"
)

type OperationMethod string
type OperationRecord = map[OperationMethod]*Operation

type OpenAPIDoc struct {
	OpenAPIVersion *string              `yaml:"openapi"`
	Info           *Info                `yaml:"info"`
	Servers        []*Server            `yaml:"servers,omitempty"`
	Components     *Components          `yaml:"components,omitempty"`
	Paths          map[string]*PathItem `yaml:"paths,omitempty"`
	Security       *SecurityRequirement `yaml:"security,omitempty"`
}

type SecurityRequirement []map[string][]string
type ReferenceRegister map[string]string

type Info struct {
	Title          string  `yaml:"title"`
	Description    string  `yaml:"description"`
	TermsOfService string  `yaml:"termsOfService"`
	Contact        Contact `yaml:"contact"`
	License        License `yaml:"license"`
	Version        string  `yaml:"version"`
}

type Contact struct {
	Email string `yaml:"email"`
}

type License struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
}

type Server struct {
	URL         string `yaml:"url"`
	Description string `yaml:"description"`
}

type Component struct {
	FilePath   string
	Name       string
	Identifier string
	Type       string
}

type Components struct {
	SecuritySchemes map[string]*SecurityScheme `yaml:"securitySchemes,omitempty"`
	Parameters      map[string]*Parameter      `yaml:"parameters,omitempty"`
	Schemas         map[string]*Schema         `yaml:"schemas,omitempty"`
	Responses       map[string]*Response       `yaml:"responses,omitempty"`
	Examples        map[string]*Example        `yaml:"examples,omitempty"`
	Register        ReferenceRegister          `yaml:"-"`
}

type Example struct {
	Ref           *string     `yaml:"$ref,omitempty"`
	Summary       string      `yaml:"summary,omitempty"`
	Description   string      `yaml:"description,omitempty"`
	Value         interface{} `yaml:"value,omitempty"`
	ExternalValue string      `yaml:"externalValue,omitempty"`
}

type SecurityScheme struct {
	Ref          *string `yaml:"$ref,omitempty"`
	Type         string  `yaml:"type,omitempty"`
	Scheme       string  `yaml:"scheme,omitempty"`
	BearerFormat string  `yaml:"bearerFormat,omitempty"`
	In           string  `yaml:"in,omitempty"`
	Name         string  `yaml:"name,omitempty"`
	Description  string  `yaml:"description"`
}

type Parameter struct {
	Ref         *string             `yaml:"$ref,omitempty"`
	Name        string              `yaml:"name,omitempty"`
	In          string              `yaml:"in,omitempty"`
	Required    bool                `yaml:"required,omitempty"`
	Schema      *Schema             `yaml:"schema,omitempty"`
	Description string              `yaml:"description,omitempty"`
	Example     interface{}         `yaml:"example,omitempty"`
	Examples    map[string]*Example `yaml:"examples,omitempty"`
	Style       string              `yaml:"style,omitempty"`
	Explode     *bool               `yaml:"explode,omitempty"`
}

type PathItem struct {
	Parameters  []*Parameter `yaml:"parameters,omitempty"`
	Get         *Operation   `yaml:"get,omitempty"`
	Post        *Operation   `yaml:"post,omitempty"`
	Put         *Operation   `yaml:"put,omitempty"`
	Patch       *Operation   `yaml:"patch,omitempty"`
	Delete      *Operation   `yaml:"delete,omitempty"`
	Options     *Operation   `yaml:"options,omitempty"`
	Head        *Operation   `yaml:"head,omitempty"`
	Trace       *Operation   `yaml:"trace,omitempty"`
	Summary     *string      `yaml:"summary,omitempty"`
	Description *string      `yaml:"description,omitempty"`
}

// SchemaType is a schema's `type`. OpenAPI 3.1 adopts JSON Schema 2020-12, where it is either one
// type name or a list of them, and the list is how 3.1 states a nullable field: `[string, "null"]`.
// 3.0 allowed one name only and stated the same thing as `nullable: true`, the `Nullable` field
// below.
//
// Held as a list whichever way it arrived, so nothing downstream asks which spelling it got, and
// written back in the shape it came in: one name marshals as a scalar, several as a sequence.
type SchemaType []string

func (t *SchemaType) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		var name string
		if err := value.Decode(&name); err != nil {
			return err
		}
		*t = SchemaType{name}
		return nil

	case yaml.SequenceNode:
		var names []string
		if err := value.Decode(&names); err != nil {
			return err
		}
		*t = names
		return nil

	default:
		return fmt.Errorf("line %d: `type` must be a name or a list of names", value.Line)
	}
}

func (t SchemaType) MarshalYAML() (interface{}, error) {
	if len(t) == 1 {
		return t[0], nil
	}

	return []string(t), nil
}

// Whether the schema is of this type. A 3.1 list carrying the name alongside `null` is too.
func (t SchemaType) Is(name string) bool {
	for _, held := range t {
		if held == name {
			return true
		}
	}

	return false
}

type Schema struct {
	Ref         *string            `yaml:"$ref,omitempty"`
	Type        SchemaType         `yaml:"type,omitempty"`
	Description *string            `yaml:"description,omitempty"`
	Properties  map[string]*Schema `yaml:"properties,omitempty"`
	Required    []*string          `yaml:"required,omitempty"`
	Format      *string            `yaml:"format,omitempty"`
	Example     interface{}        `yaml:"example,omitempty"`
	Nullable    *bool              `yaml:"nullable,omitempty"`
	Items       *Schema            `yaml:"items,omitempty"`

	Enum                 []interface{} `yaml:"enum,omitempty"`
	Default              interface{}   `yaml:"default,omitempty"`
	Minimum              interface{}   `yaml:"minimum,omitempty"`
	Maximum              interface{}   `yaml:"maximum,omitempty"`
	MinLength            interface{}   `yaml:"minLength,omitempty"`
	MaxLength            interface{}   `yaml:"maxLength,omitempty"`
	Pattern              *string       `yaml:"pattern,omitempty"`
	AdditionalProperties interface{}   `yaml:"additionalProperties,omitempty"`

	AllOf []*Schema `yaml:"allOf,omitempty"`
	OneOf []*Schema `yaml:"oneOf,omitempty"`
	AnyOf []*Schema `yaml:"anyOf,omitempty"`

	// Specification extensions (`x-` keys) and any keyword the struct does not name
	// ride through conversion untouched instead of being silently dropped.
	Extensions map[string]interface{} `yaml:",inline"`
}

type Operation struct {
	Ref         *string               `yaml:"$ref,omitempty"`
	OperationID *string               `yaml:"operationId,omitempty"`
	Summary     *string               `yaml:"summary,omitempty"`
	Description *string               `yaml:"description,omitempty"`
	Parameters  []*Parameter          `yaml:"parameters,omitempty"`
	Security    []map[string][]string `yaml:"security,omitempty"`
	Responses   map[string]*Response  `yaml:"responses,omitempty"`
	RequestBody *RequestBody          `yaml:"requestBody,omitempty"`
	Tags        *[]string             `yaml:"tags,omitempty"`
	Method      string                `yaml:"-"`

	// Specification extensions (`x-` keys) ride through conversion untouched
	// instead of being silently dropped.
	Extensions map[string]interface{} `yaml:",inline"`
}

type RequestBody struct {
	Ref      *string                     `yaml:"$ref,omitempty"`
	Required *bool                       `yaml:"required,omitempty"`
	Content  map[string]*ResponseContent `yaml:"content,omitempty"`
}

type Response struct {
	Ref         *string                     `yaml:"$ref,omitempty"`
	Description *string                     `yaml:"description,omitempty"`
	Content     map[string]*ResponseContent `yaml:"content,omitempty"`
}

type ResponseContent struct {
	Schema   *Schema             `yaml:"schema,omitempty"`
	Example  interface{}         `yaml:"example,omitempty"`
	Examples map[string]*Example `yaml:"examples,omitempty"`
}
