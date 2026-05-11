package converter

const (
	MethodGET    OperationMethod = "GET"
	MethodDELETE OperationMethod = "DELETE"
	MethodPUT    OperationMethod = "PUT"
	MethodPOST   OperationMethod = "POST"
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
}

type PathItem struct {
	Parameters  []*Parameter `yaml:"parameters,omitempty"`
	Get         *Operation   `yaml:"get,omitempty"`
	Post        *Operation   `yaml:"post,omitempty"`
	Put         *Operation   `yaml:"put,omitempty"`
	Delete      *Operation   `yaml:"delete,omitempty"`
	Summary     *string      `yaml:"summary,omitempty"`
	Description *string      `yaml:"description,omitempty"`
}

type Schema struct {
	Ref         *string            `yaml:"$ref,omitempty"`
	Type        *string            `yaml:"type,omitempty"`
	Description *string            `yaml:"description,omitempty"`
	Properties  map[string]*Schema `yaml:"properties,omitempty"`
	Required    []*string          `yaml:"required,omitempty"`
	Format      *string            `yaml:"format,omitempty"`
	Example     interface{}        `yaml:"example,omitempty"`
	Nullable    *bool              `yaml:"nullable,omitempty"`
	Items       *Schema            `yaml:"items,omitempty"`

	AllOf []*Schema `yaml:"allOf,omitempty"`
	OneOf []*Schema `yaml:"oneOf,omitempty"`
	AnyOf []*Schema `yaml:"anyOf,omitempty"`
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
