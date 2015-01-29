package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/rightscale/rsclient/generators/text"
)

// CodeWriter struct exposes methods to generate the go API client code
type CodeWriter struct {
	headerTmpl   *template.Template
	resourceTmpl *template.Template
}

// Data structure used to render resource code
// Note: we don't want to use hashes in structs used to generate the code to guarantee that code
// is always generated in the same order (to allow for diff etc.)
type ResourceData struct {
	Name        string
	Description string
	Actions     []*ResourceAction
	Attributes  []*ResourceAttribute
}

// Data structure used to render resource method
type ResourceAction struct {
	Name          string
	Description   string
	HttpMethod    string
	Path          string
	Url           string
	AllParams     []*ActionParam // All parameters
	PathParams    []*ActionParam // Params used to build URL
	PayloadParams []*ActionParam // Params that should be sent in payload
	QueryParams   []*ActionParam // Params that should be sent in query string
	Return        string
}

// Resource attributes, 'Type' is string representation of go type, e.g. "*time.Time"
type ResourceAttribute struct {
	Name      string
	JsonName  string
	Signature string
}

// Data structure used to render method params
type ActionParam struct {
	Name      string
	QueryName string
	Signature string
}

// Code writer factory
func NewCodeWriter() (*CodeWriter, error) {
	funcMap := template.FuncMap{
		"comment":        comment,
		"now":            time.Now,
		"join":           strings.Join,
		"commandLine":    commandLine,
		"joinParams":     joinParams,
		"joinParamNames": joinParamNames,
		"paramsAsMap":    paramsAsMap,
		"isPointer":      isPointer,
		"isArray":        isArray,
	}
	headerT, err := template.New("header").Funcs(funcMap).Parse(headerTmpl)
	if err != nil {
		return nil, err
	}
	resourceT, err := template.New("resource").Funcs(funcMap).Parse(resourceTmpl)
	if err != nil {
		return nil, err
	}
	return &CodeWriter{
		headerTmpl:   headerT,
		resourceTmpl: resourceT,
	}, nil
}

// Write header text
func (c *CodeWriter) WriteHeader(w io.Writer) error {
	return c.headerTmpl.Execute(w, nil)
}

// Write code for a resource
func (c *CodeWriter) WriteResource(resource *ResourceData, w io.Writer) error {
	return c.resourceTmpl.Execute(w, resource)
}

// Produce line comments by concatenating given strings and producing 80 characters long lines
// wrapped in "/*" "*/"
func comment(elems ...string) string {
	t := strings.Join(elems, "")
	return text.Indent(text.Wrap(t, 77), "// ")
}

// Serialize action parameters
func joinParams(p []*ActionParam) string {
	params := make([]string, len(p))
	for i, param := range p {
		params[i] = fmt.Sprintf("%s %s", param.Name, param.Signature)
	}
	return strings.Join(params, ", ")
}

// Serialize action parameter names
func joinParamNames(p []*ActionParam) string {
	paramNames := make([]string, len(p))
	for i, param := range p {
		paramNames[i] = param.Name
	}
	return strings.Join(paramNames, ", ")
}

// Create map out of parameter names
func paramsAsMap(p []*ActionParam) string {
	params := make([]string, len(p))
	for i, param := range p {
		params[i] = fmt.Sprintf("\"%s\": %s", param.QueryName, param.Name)
	}
	return fmt.Sprintf("map[string]interface{}{\n%s}", strings.Join(params, ",\n"))
}

// Return true if signature contains pointer, false otherwise
func isPointer(sig string) bool {
	return strings.HasPrefix(sig, "*")
}

// Return true if signature contains an array, false otherwise
func isArray(sig string) bool {
	return strings.HasPrefix(sig, "[]")
}

// Command line used to run tool
func commandLine() string {
	return fmt.Sprintf("$ api15gen %s", strings.Join(os.Args[1:], " "))
}

// Inline templates

const headerTmpl = `
//************************************************************************//
//                     RightScale API 1.5 go client
//
{{comment "Generated " (now.Format "Jan 2, 2006 at 3:04pm (PST)")}}
// Command:
{{comment commandLine}}
//
// The content of this file is auto-generated, DO NOT MODIFY
//************************************************************************//

package rsapi15

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

// Href
type Href string
`

const resourceTmpl = `{{define "actionBody"}}` + actionBodyTmpl + `{{end}}
// == {{.Name}} ==

{{comment .Description}}
type {{.Name}} struct { {{range .Attributes}}
{{.Name}} {{.Signature}} ` + "`" + `json:"{{.JsonName}},omitempty"` + "`" + `{{end}}
}
{{range .Actions}}
// {{.HttpMethod}} {{.Path}}
{{comment .Description}}
func (c *Client) {{.Name}}({{joinParams .AllParams}}){{if .Return}} ({{.Return}},{{end}} error{{if .Return}}){{end}} {
	{{template "actionBody" . }}
}
func (c *Client) {{.Name}}G(p *Params){{if .Return}} ({{.Return}},{{end}} error{{if .Return}}){{end}} {
	{{range .AllParams}}{{.Name}} := (*p)["{{.Name}}"].({{.Signature}})
	{{end}}return c.{{.Name}}({{joinParamNames .AllParams}})
}
{{end}}
`

const actionBodyTmpl = `{{if .Return}}var res {{.Return}}
	{{end}}{{if .PayloadParams}}payload := {{paramsAsMap .PayloadParams}}
	b, err := json.Marshal(payload)
	if err != nil {
		{{if .Return}}return res, err{{else}}return err{{end}}
	}
	{{else}}b := []byte{}{{end}}
	body := bytes.NewReader(b)
	req, err := http.NewRequest("{{.HttpMethod}}", c.endpoint+{{.Url}}, body)
	if err != nil {
		return {{if .Return}}res, {{end}}err
	}
	{{if .QueryParams}}{{range .QueryParams}}{{if isArray .Signature}}for _, v := range {{.Name}} {
		req.URL.Query().Add("{{.QueryName}}", v)
	}
	{{else}}req.URL.Query().Set("{{.Name}}", {{.Name}})
	{{end}}{{end}}{{end}}{{if .PayloadParams}}req.Header.Set("Content-Type", "application/json")
	{{end}}ctx := c.beforeRequest(req)
	resp, err := c.client.Do(req)
	c.afterRequest(resp, ctx)
	if err != nil {
		return {{if .Return}}res, {{end}}err
	}
	{{if eq .Return "Href"}}loc := resp.Header.Get("Location")
	if len(loc) == 0 {
		return res, fmt.Errorf("Missing location header in response")
	} else {
		return Href(loc), nil
	}{{else if .Return}}defer resp.Body.Close()
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return res, err
	}
	err = json.Unmarshal(respBody, {{if not (isPointer .Return)}}&{{end}}res)
	return res, err{{else}}return nil{{end}}`
