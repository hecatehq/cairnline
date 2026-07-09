package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"

	cairnline "github.com/hecatehq/cairnline"
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
			s.handleMessage(ctx, msg, out)
		}()
	}
	wg.Wait()

	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("mcp scanner: %w", err)
	}
	return nil
}

func (s *Server) handleMessage(ctx context.Context, raw []byte, out io.Writer) {
	var req Request
	if err := json.Unmarshal(raw, &req); err != nil {
		s.writeResponse(out, errorResponse(nil, NewError(ErrCodeParseError, "parse error: "+err.Error())))
		return
	}
	if req.JSONRPC != "2.0" {
		s.writeResponse(out, errorResponse(req.ID, NewError(ErrCodeInvalidRequest, "jsonrpc must be \"2.0\"")))
		return
	}
	result, rpcErr := s.dispatch(ctx, req)
	if req.IsNotification() {
		return
	}
	if rpcErr != nil {
		s.writeResponse(out, errorResponse(req.ID, rpcErr))
		return
	}
	s.writeResponse(out, successResponse(req.ID, result))
}

func (s *Server) dispatch(ctx context.Context, req Request) (any, *RPCError) {
	switch req.Method {
	case "initialize":
		return InitializeResult{
			ProtocolVersion: DeclaredProtocolVersion,
			Capabilities:    s.capabilities(),
			ServerInfo:      s.info,
		}, nil
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

func (s *Server) capabilities() ServerCapabilities {
	capabilities := ServerCapabilities{Tools: &ToolsCapability{}}
	if len(s.resourceProviders) > 0 || len(s.resourceTemplates) > 0 {
		capabilities.Resources = &ResourcesCapability{}
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
				Code:    cairnline.ClassifyErrorCode(err),
				Message: err.Error(),
			}},
			IsError: true,
		}, nil
	}
	return result, nil
}

func (s *Server) writeResponse(out io.Writer, response Response) {
	raw, err := json.Marshal(response)
	if err != nil {
		raw, _ = json.Marshal(errorResponse(response.ID, NewError(ErrCodeInternalError, "marshal response: "+err.Error())))
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, _ = out.Write(append(raw, '\n'))
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
