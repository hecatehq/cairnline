package mcp

import "encoding/json"

const DeclaredProtocolVersion = "2025-11-25"

type Request struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method"`
	Params  json.RawMessage  `json:"params,omitempty"`
}

func (r Request) IsNotification() bool { return r.ID == nil }

type Response struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id"`
	Result  json.RawMessage  `json:"result,omitempty"`
	Error   *RPCError        `json:"error,omitempty"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *RPCError) Error() string { return e.Message }

const (
	ErrCodeParseError     = -32700
	ErrCodeInvalidRequest = -32600
	ErrCodeMethodNotFound = -32601
	ErrCodeInvalidParams  = -32602
	ErrCodeInternalError  = -32603
)

func NewError(code int, message string) *RPCError {
	return &RPCError{Code: code, Message: message}
}

type InitializeParams struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ClientCapabilities `json:"capabilities,omitempty"`
	ClientInfo      ClientInfo         `json:"clientInfo,omitempty"`
}

// ClientCapabilities carries the capability declarations a client sends during
// initialize. Extensions mirrors the server-side extension map so a host can
// negotiate optional protocol extensions; Cairnline parses it today and
// declares no extensions of its own yet.
type ClientCapabilities struct {
	Extensions map[string]json.RawMessage `json:"extensions,omitempty"`
}

type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type InitializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      ServerInfo         `json:"serverInfo"`
}

type ServerCapabilities struct {
	Tools      *ToolsCapability           `json:"tools,omitempty"`
	Resources  *ResourcesCapability       `json:"resources,omitempty"`
	Extensions map[string]json.RawMessage `json:"extensions,omitempty"`
}

type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

type ResourcesCapability struct {
	Subscribe   bool `json:"subscribe,omitempty"`
	ListChanged bool `json:"listChanged,omitempty"`
}

type ServerInfo struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description,omitempty"`
}

type Tool struct {
	Name         string           `json:"name"`
	Title        string           `json:"title,omitempty"`
	Description  string           `json:"description,omitempty"`
	InputSchema  json.RawMessage  `json:"inputSchema"`
	OutputSchema json.RawMessage  `json:"outputSchema,omitempty"`
	Annotations  *ToolAnnotations `json:"annotations,omitempty"`
	Meta         map[string]any   `json:"_meta,omitempty"`
}

type ToolAnnotations struct {
	ReadOnlyHint    *bool `json:"readOnlyHint,omitempty"`
	DestructiveHint *bool `json:"destructiveHint,omitempty"`
	IdempotentHint  *bool `json:"idempotentHint,omitempty"`
}

func BoolPtr(v bool) *bool { return &v }

type ListToolsResult struct {
	Tools []Tool `json:"tools"`
}

type Resource struct {
	URI         string         `json:"uri"`
	Name        string         `json:"name"`
	Title       string         `json:"title,omitempty"`
	Description string         `json:"description,omitempty"`
	MimeType    string         `json:"mimeType,omitempty"`
	Meta        map[string]any `json:"_meta,omitempty"`
}

type ResourceTemplate struct {
	URITemplate string `json:"uriTemplate"`
	Name        string `json:"name"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

type ListResourcesResult struct {
	Resources []Resource `json:"resources"`
}

type ListResourceTemplatesResult struct {
	ResourceTemplates []ResourceTemplate `json:"resourceTemplates"`
}

type ReadResourceParams struct {
	URI string `json:"uri"`
}

type ReadResourceResult struct {
	Contents []ResourceContent `json:"contents"`
}

// ResourceContent is the body of a resource. Text carries UTF-8 payloads and
// Blob carries base64-encoded binary payloads; the two are mutually exclusive
// per the MCP spec. ResourceContent also backs embedded-resource tool results
// (see Content.Resource).
type ResourceContent struct {
	URI      string         `json:"uri"`
	MimeType string         `json:"mimeType,omitempty"`
	Text     string         `json:"text,omitempty"`
	Blob     string         `json:"blob,omitempty"`
	Meta     map[string]any `json:"_meta,omitempty"`
}

type CallToolParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

type CallToolResult struct {
	Content           []Content      `json:"content"`
	StructuredContent any            `json:"structuredContent,omitempty"`
	IsError           bool           `json:"isError,omitempty"`
	Meta              map[string]any `json:"_meta,omitempty"`
}

// Content is a single tool-result content block. Type selects the variant:
// "text" uses Text; "resource" embeds a ResourceContent (text or blob) via
// Resource.
type Content struct {
	Type     string           `json:"type"`
	Text     string           `json:"text,omitempty"`
	Resource *ResourceContent `json:"resource,omitempty"`
}

func TextContent(text string) []Content {
	return []Content{{Type: "text", Text: text}}
}

// ToolErrorPayload is the StructuredContent carried on a failed tool call. It
// gives hosts a machine-readable error alongside the unchanged human-readable
// Content, so a client can branch on Error.Code without parsing prose.
type ToolErrorPayload struct {
	Error ToolErrorDetail `json:"error"`
}

// ToolErrorDetail names the failure class (see the cairnline ErrorCode*
// contract) and echoes the underlying error message.
type ToolErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// EmbeddedResource returns a content block that embeds a resource payload in a
// tool result. The resource may carry either Text or Blob content.
func EmbeddedResource(resource ResourceContent) Content {
	return Content{Type: "resource", Resource: &resource}
}
