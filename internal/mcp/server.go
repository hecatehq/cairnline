package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/hecatehq/cairnline/internal/core"
)

type ToolHandler func(context.Context, json.RawMessage) (CallToolResult, error)
type ResourceProvider func(context.Context) ([]Resource, error)
type ResourceReader func(context.Context, string) (ReadResourceResult, bool, error)

type Server struct {
	info              ServerInfo
	tools             map[string]registeredTool
	resourceProviders []ResourceProvider
	resourceReaders   []ResourceReader
	resourceTemplates []ResourceTemplate
	extensions        map[string]json.RawMessage

	writeMu sync.Mutex
}

type registeredTool struct {
	descriptor Tool
	handler    ToolHandler
}

func NewServer(name, version, description string) *Server {
	return &Server{
		info: ServerInfo{
			Name:        name,
			Version:     version,
			Description: description,
		},
		tools: make(map[string]registeredTool),
	}
}

func (s *Server) RegisterTool(tool Tool, handler ToolHandler) {
	s.tools[tool.Name] = registeredTool{descriptor: tool, handler: handler}
}

func (s *Server) RegisterResourceProvider(provider ResourceProvider, reader ResourceReader) {
	s.resourceProviders = append(s.resourceProviders, provider)
	s.resourceReaders = append(s.resourceReaders, reader)
}

func (s *Server) RegisterResourceTemplates(templates []ResourceTemplate) {
	s.resourceTemplates = append(s.resourceTemplates, templates...)
}

// DeclareExtension advertises a protocol extension in the server's initialize
// capabilities under its extension id (for example "io.modelcontextprotocol/ui").
// config is the extension's capability object, passed through verbatim; pass nil
// to declare an extension with no configuration. Declaring an extension lets a
// host negotiate it during initialize; Cairnline ships with none declared.
func (s *Server) DeclareExtension(id string, config json.RawMessage) {
	if s.extensions == nil {
		s.extensions = make(map[string]json.RawMessage)
	}
	if config == nil {
		config = json.RawMessage("{}")
	}
	s.extensions[id] = config
}

func (s *Server) Serve(ctx context.Context, in io.Reader, out io.Writer) error {
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	go func() {
		<-ctx.Done()
		if closer, ok := in.(io.Closer); ok {
			_ = closer.Close()
		}
	}()

	var wg sync.WaitGroup
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		msg := make([]byte, len(line))
		copy(msg, line)
		wg.Add(1)
		go func() {
			defer wg.Done()
			if response, ok := s.HandleMessage(ctx, msg); ok {
				s.write(out, response)
			}
		}()
	}
	wg.Wait()

	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("mcp scanner: %w", err)
	}
	return nil
}

// HandleMessage processes a single JSON-RPC message and returns the encoded
// response. The boolean is false for notifications, which produce no response.
// Embedding hosts that mount Cairnline's MCP surface on their own transport call
// HandleMessage per inbound message instead of Serve, which owns the stdio loop.
func (s *Server) HandleMessage(ctx context.Context, raw []byte) ([]byte, bool) {
	var req Request
	if err := json.Unmarshal(raw, &req); err != nil {
		return encodeResponse(errorResponse(nil, NewError(ErrCodeParseError, "parse error: "+err.Error()))), true
	}
	if req.JSONRPC != "2.0" {
		return encodeResponse(errorResponse(req.ID, NewError(ErrCodeInvalidRequest, "jsonrpc must be \"2.0\""))), true
	}
	result, rpcErr := s.dispatch(ctx, req)
	if req.IsNotification() {
		return nil, false
	}
	if rpcErr != nil {
		return encodeResponse(errorResponse(req.ID, rpcErr)), true
	}
	return encodeResponse(successResponse(req.ID, result)), true
}

func (s *Server) dispatch(ctx context.Context, req Request) (any, *RPCError) {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req.Params)
	case "notifications/initialized":
		return struct{}{}, nil
	case "tools/list":
		return s.handleListTools(), nil
	case "tools/call":
		return s.handleCallTool(ctx, req.Params)
	case "resources/list":
		return s.handleListResources(ctx)
	case "resources/templates/list":
		return s.handleListResourceTemplates(), nil
	case "resources/read":
		return s.handleReadResource(ctx, req.Params)
	case "ping":
		return struct{}{}, nil
	default:
		return nil, NewError(ErrCodeMethodNotFound, "method not found: "+req.Method)
	}
}

func (s *Server) handleInitialize(raw json.RawMessage) (any, *RPCError) {
	var params InitializeParams
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &params); err != nil {
			return nil, NewError(ErrCodeInvalidParams, "invalid initialize params: "+err.Error())
		}
	}
	// params.Capabilities carries the client's extension declarations; parsing it
	// keeps extension negotiation open even though Cairnline declares none yet.
	return InitializeResult{
		ProtocolVersion: DeclaredProtocolVersion,
		Capabilities:    s.capabilities(),
		ServerInfo:      s.info,
	}, nil
}

func (s *Server) capabilities() ServerCapabilities {
	capabilities := ServerCapabilities{Tools: &ToolsCapability{}}
	if len(s.resourceProviders) > 0 || len(s.resourceTemplates) > 0 {
		capabilities.Resources = &ResourcesCapability{}
	}
	if len(s.extensions) > 0 {
		extensions := make(map[string]json.RawMessage, len(s.extensions))
		for id, config := range s.extensions {
			extensions[id] = config
		}
		capabilities.Extensions = extensions
	}
	return capabilities
}

func (s *Server) handleListTools() ListToolsResult {
	tools := make([]Tool, 0, len(s.tools))
	for _, item := range s.tools {
		tools = append(tools, item.descriptor)
	}
	return ListToolsResult{Tools: tools}
}

func (s *Server) handleListResources(ctx context.Context) (any, *RPCError) {
	resources := make([]Resource, 0)
	for _, provider := range s.resourceProviders {
		items, err := provider(ctx)
		if err != nil {
			return nil, NewError(ErrCodeInternalError, "list resources: "+err.Error())
		}
		resources = append(resources, items...)
	}
	return ListResourcesResult{Resources: resources}, nil
}

func (s *Server) handleListResourceTemplates() ListResourceTemplatesResult {
	templates := make([]ResourceTemplate, len(s.resourceTemplates))
	copy(templates, s.resourceTemplates)
	return ListResourceTemplatesResult{ResourceTemplates: templates}
}

func (s *Server) handleReadResource(ctx context.Context, raw json.RawMessage) (any, *RPCError) {
	var params ReadResourceParams
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &params); err != nil {
			return nil, NewError(ErrCodeInvalidParams, "invalid resource read params: "+err.Error())
		}
	}
	if params.URI == "" {
		return nil, NewError(ErrCodeInvalidParams, "resource uri is required")
	}
	for _, reader := range s.resourceReaders {
		result, ok, err := reader(ctx, params.URI)
		if err != nil {
			return nil, NewError(ErrCodeInternalError, "read resource: "+err.Error())
		}
		if ok {
			return result, nil
		}
	}
	return nil, NewError(ErrCodeInvalidParams, "unknown resource: "+params.URI)
}

func (s *Server) handleCallTool(ctx context.Context, raw json.RawMessage) (any, *RPCError) {
	var params CallToolParams
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &params); err != nil {
			return nil, NewError(ErrCodeInvalidParams, "invalid tool call params: "+err.Error())
		}
	}
	tool, ok := s.tools[params.Name]
	if !ok {
		return nil, NewError(ErrCodeInvalidParams, "unknown tool: "+params.Name)
	}
	result, err := tool.handler(ctx, params.Arguments)
	if err != nil {
		// Single tool-error path for every registered handler: keep the human
		// prose in Content untouched and attach a machine-readable error code
		// so hosts can classify the failure without parsing the message.
		return CallToolResult{
			Content: TextContent(err.Error()),
			StructuredContent: ToolErrorPayload{Error: ToolErrorDetail{
				Code:    core.ClassifyErrorCode(err),
				Message: err.Error(),
			}},
			IsError: true,
		}, nil
	}
	return result, nil
}

func (s *Server) write(out io.Writer, payload []byte) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, _ = out.Write(append(payload, '\n'))
}

func encodeResponse(response Response) []byte {
	raw, err := json.Marshal(response)
	if err != nil {
		raw, _ = json.Marshal(errorResponse(response.ID, NewError(ErrCodeInternalError, "marshal response: "+err.Error())))
	}
	return raw
}

func successResponse(id *json.RawMessage, result any) Response {
	raw, err := json.Marshal(result)
	if err != nil {
		return errorResponse(id, NewError(ErrCodeInternalError, "marshal result: "+err.Error()))
	}
	return Response{JSONRPC: "2.0", ID: id, Result: raw}
}

func errorResponse(id *json.RawMessage, rpcErr *RPCError) Response {
	return Response{JSONRPC: "2.0", ID: id, Error: rpcErr}
}
