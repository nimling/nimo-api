package converter

import (
	"fmt"
	"sort"
	"strings"
)

func (n *OpenAPIConverter) ValidateInternalRefs() error {
	known := n.knownComponentKeys()
	missing := map[string][]string{}

	collect := func(loc, ref string) {
		if ref == "" || !strings.HasPrefix(ref, "#/") {
			return
		}
		if _, ok := known[ref]; ok {
			return
		}
		missing[ref] = append(missing[ref], loc)
	}

	if n.doc.Components != nil {
		for name, schema := range n.doc.Components.Schemas {
			walkSchemaRefs(schema, "components/schemas/"+name, collect)
		}
		for name, param := range n.doc.Components.Parameters {
			loc := "components/parameters/" + name
			if param.Ref != nil {
				collect(loc, *param.Ref)
			}
			if param.Schema != nil {
				walkSchemaRefs(param.Schema, loc+"/schema", collect)
			}
		}
		for name, scheme := range n.doc.Components.SecuritySchemes {
			if scheme.Ref != nil {
				collect("components/securitySchemes/"+name, *scheme.Ref)
			}
		}
	}

	for path, item := range n.doc.Paths {
		if item == nil {
			continue
		}
		base := "paths/" + path
		for i, p := range item.Parameters {
			if p == nil {
				continue
			}
			loc := fmt.Sprintf("%s/parameters/%d", base, i)
			if p.Ref != nil {
				collect(loc, *p.Ref)
			}
			if p.Schema != nil {
				walkSchemaRefs(p.Schema, loc+"/schema", collect)
			}
		}
		for method, op := range item.Operations() {
			if op == nil {
				continue
			}
			opBase := base + "/" + string(method)
			if op.Ref != nil {
				collect(opBase, *op.Ref)
			}
			for i, p := range op.Parameters {
				if p == nil {
					continue
				}
				loc := fmt.Sprintf("%s/parameters/%d", opBase, i)
				if p.Ref != nil {
					collect(loc, *p.Ref)
				}
				if p.Schema != nil {
					walkSchemaRefs(p.Schema, loc+"/schema", collect)
				}
			}
			if op.RequestBody != nil {
				rb := opBase + "/requestBody"
				if op.RequestBody.Ref != nil {
					collect(rb, *op.RequestBody.Ref)
				}
				for media, content := range op.RequestBody.Content {
					if content != nil && content.Schema != nil {
						walkSchemaRefs(content.Schema, rb+"/content/"+media+"/schema", collect)
					}
				}
			}
			for code, resp := range op.Responses {
				if resp == nil {
					continue
				}
				rl := opBase + "/responses/" + code
				if resp.Ref != nil {
					collect(rl, *resp.Ref)
				}
				for media, content := range resp.Content {
					if content != nil && content.Schema != nil {
						walkSchemaRefs(content.Schema, rl+"/content/"+media+"/schema", collect)
					}
				}
			}
		}
	}

	if len(missing) == 0 {
		return nil
	}

	keys := make([]string, 0, len(missing))
	for k := range missing {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	b.WriteString("unresolved internal $ref(s) in spec:\n")
	for _, ref := range keys {
		locs := missing[ref]
		sort.Strings(locs)
		fmt.Fprintf(&b, "  %s\n    referenced from:\n", ref)
		for _, loc := range locs {
			fmt.Fprintf(&b, "      - %s\n", loc)
		}
	}
	return fmt.Errorf("%s", b.String())
}

func (n *OpenAPIConverter) knownComponentKeys() map[string]struct{} {
	out := map[string]struct{}{}
	if n.doc.Components == nil {
		return out
	}
	for name := range n.doc.Components.SecuritySchemes {
		out["#/components/securitySchemes/"+name] = struct{}{}
	}
	for name := range n.doc.Components.Parameters {
		out["#/components/parameters/"+name] = struct{}{}
	}
	for name := range n.doc.Components.Schemas {
		out["#/components/schemas/"+name] = struct{}{}
	}
	for name := range n.doc.Components.Responses {
		out["#/components/responses/"+name] = struct{}{}
	}
	return out
}

func walkSchemaRefs(s *Schema, loc string, collect func(string, string)) {
	if s == nil {
		return
	}
	if s.Ref != nil {
		collect(loc, *s.Ref)
	}
	for name, prop := range s.Properties {
		walkSchemaRefs(prop, loc+"/properties/"+name, collect)
	}
	if s.Items != nil {
		walkSchemaRefs(s.Items, loc+"/items", collect)
	}
	for i, sub := range s.AllOf {
		walkSchemaRefs(sub, fmt.Sprintf("%s/allOf/%d", loc, i), collect)
	}
	for i, sub := range s.OneOf {
		walkSchemaRefs(sub, fmt.Sprintf("%s/oneOf/%d", loc, i), collect)
	}
	for i, sub := range s.AnyOf {
		walkSchemaRefs(sub, fmt.Sprintf("%s/anyOf/%d", loc, i), collect)
	}
}
