package converter

import (
	"path/filepath"
	"strings"
)

func (p *PathItem) Operations() OperationRecord {
	if p == nil {
		return OperationRecord{}
	}

	operations := make(OperationRecord)

	if p.Get != nil {
		p.Get.Method = "GET"
		operations[MethodGET] = p.Get
	}

	if p.Delete != nil {
		p.Delete.Method = "DELETE"
		operations[MethodDELETE] = p.Delete
	}

	if p.Put != nil {
		p.Put.Method = "PUT"
		operations[MethodPUT] = p.Put
	}

	if p.Post != nil {
		p.Post.Method = "POST"
		operations[MethodPOST] = p.Post
	}

	if p.Patch != nil {
		p.Patch.Method = "PATCH"
		operations[MethodPATCH] = p.Patch
	}

	if p.Options != nil {
		p.Options.Method = "OPTIONS"
		operations[MethodOPTIONS] = p.Options
	}

	if p.Head != nil {
		p.Head.Method = "HEAD"
		operations[MethodHEAD] = p.Head
	}

	if p.Trace != nil {
		p.Trace.Method = "TRACE"
		operations[MethodTRACE] = p.Trace
	}

	return operations
}

func (p *PathItem) SetMethodOperation(method OperationMethod, operation *Operation) {

	switch method {
	case MethodGET:
		p.Get = operation
	case MethodDELETE:
		p.Delete = operation
	case MethodPUT:
		p.Put = operation
	case MethodPOST:
		p.Post = operation
	case MethodPATCH:
		p.Patch = operation
	case MethodOPTIONS:
		p.Options = operation
	case MethodHEAD:
		p.Head = operation
	case MethodTRACE:
		p.Trace = operation
	}
}

func (c *Components) PutRegister(compType string, filePath string) *Component {
	name := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
	identifier := "#/components/" + compType + "/" + name
	if c.Register == nil {
		c.Register = ReferenceRegister{
			filePath: identifier,
		}
	} else {
		c.Register[filePath] = identifier
	}

	return &Component{
		FilePath:   filePath,
		Name:       name,
		Identifier: identifier,
		Type:       compType,
	}
}

func (c *Components) GetRegister(compType string, compName string) *Component {
	for k, v := range c.Register {
		if strings.TrimPrefix(v, "#/components/"+compType+"/") == compName {
			return &Component{
				FilePath:   k,
				Name:       compName,
				Identifier: v,
				Type:       compType,
			}
		}
	}
	return nil
}
