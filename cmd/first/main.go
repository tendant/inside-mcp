package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type RootParams struct{}
type ResourceParams struct {
	Path string `json:"path,omitempty"` // optional; defaults to root
}

func main() {
	// Determine root directory:
	// 1) MCP_FS_ROOT env var, else 2) current working directory.
	rootDir := os.Getenv("MCP_FS_ROOT")
	if rootDir == "" {
		wd, err := os.Getwd()
		if err != nil {
			log.Fatalf("cannot determine working directory: %v", err)
		}
		rootDir = wd
	}
	rootDir = absOrDie(rootDir)

	// Helper to resolve paths under root, blocking traversal outside root.
	resolveWithinRoot := func(p string) (string, error) {
		if p == "" || p == "." {
			p = ""
		}
		joined := filepath.Join(rootDir, p)
		abs, err := filepath.Abs(joined)
		if err != nil {
			return "", err
		}
		rel, err := filepath.Rel(rootDir, abs)
		if err != nil {
			return "", err
		}
		if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			return "", fmt.Errorf("path outside root: %q", p)
		}
		return abs, nil
	}

	// Init server
	srv := mcp.NewServer(&mcp.Implementation{
		Name:    "FileSystem MCP",
		Version: "0.2.0",
	}, nil)

	// Tool: list_roots  — show the single configured root
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_roots",
		Description: "Show the configured root directory for file browsing",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ RootParams) (*mcp.CallToolResult, interface{}, error) {
		text := fmt.Sprintf("Root: %s", rootDir)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, []string{rootDir}, nil
	})

	// Tool: list_resources — list entries at a path relative to root (or root if empty)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_resources",
		Description: "List files/directories under the configured root. Args: { path?: string }",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args ResourceParams) (*mcp.CallToolResult, interface{}, error) {
		target, err := resolveWithinRoot(args.Path)
		if err != nil {
			return nil, nil, err
		}
		entries, err := os.ReadDir(target)
		if err != nil {
			return nil, nil, err
		}

		names := make([]string, 0, len(entries))
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() {
				name += string(os.PathSeparator)
			}
			names = append(names, name)
		}

		// Human-friendly content + structured result
		lines := strings.Join(names, "\n")
		text := fmt.Sprintf("Listing for %s:\n%s", relOrSame(rootDir, target), lines)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, names, nil
	})

	log.Printf("Starting FileSystem MCP server with root: %s", rootDir)
	if err := srv.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal("server error:", err)
	}
}

func absOrDie(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		log.Fatalf("cannot resolve path %q: %v", p, err)
	}
	return abs
}

func relOrSame(base, p string) string {
	if r, err := filepath.Rel(base, p); err == nil && r != "." {
		return r
	}
	return p
}
