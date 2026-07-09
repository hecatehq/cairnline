package cairnline

import (
	"context"
	"strings"
	"testing"
)

func TestNewMCPServer_HandleMessageListsTools(t *testing.T) {
	server := NewMCPServer(NewMemoryService(), "test")

	resp, ok := server.HandleMessage(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	if !ok {
		t.Fatal("tools/list should produce a response")
	}
	if !strings.Contains(string(resp), `"coordination.capabilities"`) {
		t.Fatalf("tools/list missing Cairnline tools: %s", resp)
	}
}

func TestNewMCPServer_Initialize(t *testing.T) {
	server := NewMCPServer(NewMemoryService(), "test")

	resp, ok := server.HandleMessage(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25"}}`))
	if !ok {
		t.Fatal("initialize should produce a response")
	}
	if !strings.Contains(string(resp), `"protocolVersion":"2025-11-25"`) {
		t.Fatalf("initialize response = %s", resp)
	}
}
