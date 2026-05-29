# OpenAPI Specification Guidelines

This document describes the OpenAPI specification requirements and features supported by the openapi-converter tool.

## Table of Contents
- [Supported OpenAPI Version](#supported-openapi-version)
- [Required Fields](#required-fields)
- [Validation Rules](#validation-rules)
- [External References](#external-references)
- [Response Merging](#response-merging)
- [Path Structure](#path-structure)
- [Best Practices](#best-practices)

## Supported OpenAPI Version

The converter supports **OpenAPI 3.1.x** specifications in YAML format.

## Required Fields

The following fields are mandatory for successful conversion:

### Document Level
- `openapi`: Version specification (must be 3.1.x)
- `info`: API metadata
  - `title`: API title (required)
  - `description`: API description (required)
  - `version`: API version (required)
- `servers`: At least one server definition
  - `url`: Server URL (required)
- `paths`: At least one path definition

### Path Level
Each path must have:
- **Path string**: Must start with `/`
- **Summary and Description**: Required either at:
  - Path level (applies to all operations), OR
  - Operation level (each operation must have both)

### Operation Level
Each operation (GET, POST, PUT, DELETE) must have:
- `operationId`: Unique identifier for the operation (required)
- `summary`: Brief description (required if not at path level)
- `description`: Detailed description (required if not at path level)
- `responses`: At least one response definition

## Validation Rules

The converter enforces these validation rules:

1. **Path Format**
   - All paths must start with `/`
   - Paths must have at least one segment
   - Empty paths are not allowed

2. **Documentation Completeness**
   - Every path must have both `summary` and `description`
   - These can be defined at path level (inherited by all operations)
   - Or each operation must define its own

3. **Operation Requirements**
   - Every operation must have a unique `operationId`
   - Operations without `operationId` will fail validation

4. **Common Prefix Detection**
   - The converter automatically detects common path prefixes
   - Can be overridden with `--common-prefix` flag

## External References

The converter supports external file references using `$ref`:

### Supported Reference Types
- Components from external files
- Schema definitions
- Parameter definitions
- Response definitions

### Reference Format
```yaml
$ref: './schemas/User.yml'
$ref: '../common/Error.yml#/components/schemas/Error'
```

### How References Are Resolved
1. **Relative Paths**: Resolved relative to the current file's directory
2. **Absolute Paths**: Resolved from the file system root
3. **URL References**: Currently not supported
4. **Circular References**: Detected and will cause validation errors

### Example with External References
```yaml
paths:
  /users:
    get:
      summary: List users
      description: Get all users from the system
      operationId: listUsers
      responses:
        '200':
          description: Success
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: './schemas/User.yml'
        '400':
          $ref: '../common/responses/BadRequest.yml'
```

## Inlining Internal References

The converter normally keeps `#/components/...` refs intact in the emitted spec for reusability. Some downstream renderers cannot dereference these on their own and end up showing empty panels for shared examples, responses, or schemas. Use these flags to flatten internal refs at every consumption site and drop the now-orphaned components map.

| Flag | Effect |
| --- | --- |
| `--inline-examples` | Inline every `$ref: '#/components/examples/X'` inside `requestBody`, `responses`, and `parameters`; drop `components.examples`. |
| `--inline-responses` | Inline every `$ref: '#/components/responses/X'` inside operation `responses`; drop `components.responses`. |
| `--inline-schemas` | Inline every `$ref: '#/components/schemas/X'` inside schemas, parameters, request bodies, and responses; drop `components.schemas`. Self-referential schemas keep the inner `$ref` so circular cycles do not expand forever. |
| `--flat` | Shortcut for all three above. |

Source YAML stays unchanged; only the emitted `spec.json` differs. Examples reused once produce zero duplication, but inlining shared responses and schemas grows the output proportionally to how often they are referenced.

## Response Merging

The `--merge-responses-inline` flag enables automatic merging of `allOf` response definitions:

### Before Merging
```yaml
schema:
  allOf:
    - $ref: '#/components/schemas/BaseResponse'
    - type: object
      properties:
        data:
          $ref: '#/components/schemas/User'
```

### After Merging
```yaml
schema:
  type: object
  properties:
    success:
      type: boolean
    message:
      type: string
    data:
      $ref: '#/components/schemas/User'
```

## Path Structure

### Parameter Handling
Path parameters use OpenAPI standard notation:
```yaml
/users/{userId}/posts/{postId}:
  parameters:
    - name: userId
      in: path
      required: true
      schema:
        type: string
    - name: postId
      in: path
      required: true
      schema:
        type: integer
```

### Query Parameters
```yaml
parameters:
  - name: limit
    in: query
    schema:
      type: integer
      default: 10
  - name: offset
    in: query
    schema:
      type: integer
      default: 0
```

## Best Practices

### 1. Organize Your Specifications
```
api/
├── index.yml           # Main specification
├── paths/              # Path definitions
│   ├── users.yml
│   └── posts.yml
├── schemas/            # Schema definitions
│   ├── User.yml
│   └── Post.yml
└── responses/          # Common responses
    ├── Error.yml
    └── Success.yml
```

### 2. Use Descriptive OperationIds
```yaml
operationId: getUserById        # Good
operationId: getUser            # Ambiguous
operationId: get                # Bad
```

### 3. Provide Comprehensive Documentation
```yaml
paths:
  /users:
    summary: User Management
    description: Endpoints for managing system users
    get:
      summary: List all users
      description: |
        Retrieves a paginated list of all users in the system.
        Requires authentication. Returns user objects with public fields only.
      operationId: listUsers
```

### 4. Define Reusable Components
```yaml
components:
  schemas:
    Pagination:
      type: object
      properties:
        page:
          type: integer
        limit:
          type: integer
        total:
          type: integer
  
  parameters:
    PageParam:
      name: page
      in: query
      schema:
        type: integer
        default: 1
```

### 5. Use Consistent Status Codes
- `200`: Successful GET, PUT
- `201`: Successful POST (created)
- `204`: Successful DELETE (no content)
- `400`: Bad request
- `401`: Unauthorized
- `404`: Not found
- `500`: Server error

## Conversion Output

### Nginx Configuration
The converter generates Nginx location blocks with:
- Path pattern matching
- Method restrictions
- Upstream proxy configuration
- Security headers

### VitePress Documentation
The converter generates:
- Markdown documentation for each endpoint
- Interactive API documentation
- Type definitions and examples
- Navigation structure

## Troubleshooting

### Common Validation Errors

1. **Missing summary/description**
   ```
   Error: path '/users' is missing required 'summary' and 'description' fields
   ```
   Solution: Add both fields to either the path or all its operations

2. **Missing operationId**
   ```
   Error: path '/users' GET operation is missing required 'operationId'
   ```
   Solution: Add unique operationId to each operation

3. **Invalid path format**
   ```
   Error: path 'users' must start with /
   ```
   Solution: Ensure all paths begin with forward slash

4. **Unresolved references**
   ```
   Error: unable to resolve reference './schemas/User.yml'
   ```
   Solution: Check file paths and ensure referenced files exist