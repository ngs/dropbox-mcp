package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"go.ngs.io/dropbox-mcp-server/internal/handlers"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

func main() {
	var (
		versionFlag = flag.Bool("version", false, "Print version information")
		helpFlag    = flag.Bool("h", false, "Print help message")
		help2Flag   = flag.Bool("help", false, "Print help message")
	)
	flag.Parse()

	if *versionFlag {
		fmt.Printf("dropbox-mcp-server version %s (commit: %s, built: %s)\n", version, commit, date)
		os.Exit(0)
	}

	if *helpFlag || *help2Flag {
		fmt.Println("dropbox-mcp-server - MCP server for Dropbox integration")
		fmt.Println("\nUsage:")
		fmt.Println("  dropbox-mcp-server [options]")
		fmt.Println("\nOptions:")
		fmt.Println("  -h, --help     Show this help message")
		fmt.Println("  --version      Show version information")
		fmt.Println("\nThis tool is designed to be used with Claude Desktop.")
		fmt.Println("See https://github.com/ngs/dropbox-mcp-server for more information.")
		os.Exit(0)
	}

	handler, err := handlers.NewHandler()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize handler: %v\n", err)
		os.Exit(1)
	}

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()

		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to parse request: %v\n", err)
			continue
		}

		// Skip notifications (requests without ID)
		if req.ID == nil {
			// Notifications don't require a response
			if strings.HasPrefix(req.Method, "notifications/") {
				continue
			}
		}

		var resp Response
		resp.JSONRPC = "2.0"
		resp.ID = req.ID

		switch req.Method {
		case "initialize":
			resp.Result = handleInitialize()
		case "tools/list":
			resp.Result = handleListTools()
		case "tools/call":
			resp.Result = handleToolCall(handler, req.Params)
		case "prompts/list":
			resp.Result = handleListPrompts()
		case "resources/list":
			resp.Result = handleListResources()
		default:
			// Only send error response for non-notification methods
			if !strings.HasPrefix(req.Method, "notifications/") {
				resp.Error = &Error{
					Code:    -32601,
					Message: fmt.Sprintf("Method not found: %s", req.Method),
				}
			} else {
				continue // Skip sending response for notifications
			}
		}

		output, err := json.Marshal(resp)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to marshal response: %v\n", err)
			continue
		}

		fmt.Println(string(output))
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		fmt.Fprintf(os.Stderr, "Scanner error: %v\n", err)
	}
}

func handleInitialize() interface{} {
	return map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]interface{}{
			"tools": map[string]interface{}{},
		},
		"serverInfo": map[string]interface{}{
			"name":    "dropbox-mcp-server",
			"version": VERSION,
		},
	}
}

func handleListTools() interface{} {
	tools := []ToolDefinition{
		{
			Name:        "dropbox_auth",
			Description: "Authenticate with Dropbox using OAuth 2.0",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"client_id": map[string]interface{}{
						"type":        "string",
						"description": "Dropbox App Client ID (optional if DROPBOX_CLIENT_ID env var is set)",
					},
					"client_secret": map[string]interface{}{
						"type":        "string",
						"description": "Dropbox App Client Secret (optional if DROPBOX_CLIENT_SECRET env var is set)",
					},
				},
			},
		},
		{
			Name:        "dropbox_check_auth",
			Description: "Check current authentication status",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "dropbox_list",
			Description: "List files and folders in a Dropbox directory",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Path to list (empty string for root)",
						"default":     "",
					},
				},
			},
		},
		{
			Name:        "dropbox_search",
			Description: "Search for files and folders in Dropbox",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "Search query",
					},
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Path to search in (optional)",
					},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "dropbox_get_metadata",
			Description: "Get metadata for a file or folder",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Path to the file or folder",
					},
				},
				"required": []string{"path"},
			},
		},
		{
			Name:        "dropbox_download",
			Description: "Download a file from Dropbox",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Path to the file to download",
					},
				},
				"required": []string{"path"},
			},
		},
		{
			Name:        "dropbox_upload",
			Description: "Upload a file to Dropbox",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Path where the file will be uploaded",
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "File content (text or base64 encoded)",
					},
					"mode": map[string]interface{}{
						"type":        "string",
						"description": "Upload mode: 'add' or 'overwrite'",
						"default":     "add",
						"enum":        []string{"add", "overwrite"},
					},
				},
				"required": []string{"path", "content"},
			},
		},
		{
			Name:        "dropbox_create_folder",
			Description: "Create a new folder in Dropbox",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Path of the folder to create",
					},
				},
				"required": []string{"path"},
			},
		},
		{
			Name:        "dropbox_move",
			Description: "Move or rename a file or folder",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"from_path": map[string]interface{}{
						"type":        "string",
						"description": "Source path",
					},
					"to_path": map[string]interface{}{
						"type":        "string",
						"description": "Destination path",
					},
				},
				"required": []string{"from_path", "to_path"},
			},
		},
		{
			Name:        "dropbox_copy",
			Description: "Copy a file or folder",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"from_path": map[string]interface{}{
						"type":        "string",
						"description": "Source path",
					},
					"to_path": map[string]interface{}{
						"type":        "string",
						"description": "Destination path",
					},
				},
				"required": []string{"from_path", "to_path"},
			},
		},
		{
			Name:        "dropbox_delete",
			Description: "Delete a file or folder",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Path to delete",
					},
				},
				"required": []string{"path"},
			},
		},
		{
			Name:        "dropbox_create_shared_link",
			Description: "Create a shared link for a file or folder",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Path to share",
					},
					"settings": map[string]interface{}{
						"type":        "object",
						"description": "Sharing settings",
						"properties": map[string]interface{}{
							"expires": map[string]interface{}{
								"type":        "string",
								"description": "Expiration time (ISO 8601 format)",
							},
							"password": map[string]interface{}{
								"type":        "string",
								"description": "Password for the shared link",
							},
						},
					},
				},
				"required": []string{"path"},
			},
		},
		{
			Name:        "dropbox_list_shared_links",
			Description: "List shared links for a file or folder",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Path to list shared links for (optional)",
					},
				},
			},
		},
		{
			Name:        "dropbox_revoke_shared_link",
			Description: "Revoke a shared link",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"url": map[string]interface{}{
						"type":        "string",
						"description": "Shared link URL to revoke",
					},
				},
				"required": []string{"url"},
			},
		},
		{
			Name:        "dropbox_get_revisions",
			Description: "Get version history of a file",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Path to the file",
					},
				},
				"required": []string{"path"},
			},
		},
		{
			Name:        "dropbox_restore_file",
			Description: "Restore a file to a specific version",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Path to the file",
					},
					"rev": map[string]interface{}{
						"type":        "string",
						"description": "Revision ID to restore",
					},
				},
				"required": []string{"path", "rev"},
			},
		},
	}

	return map[string]interface{}{
		"tools": tools,
	}
}

func handleToolCall(handler *handlers.Handler, params json.RawMessage) interface{} {
	var toolCall struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}

	if err := json.Unmarshal(params, &toolCall); err != nil {
		return map[string]interface{}{
			"error": map[string]interface{}{
				"code":    -32602,
				"message": fmt.Sprintf("Invalid params: %v", err),
			},
		}
	}

	// Map of tool names to handler functions
	toolHandlers := map[string]func(json.RawMessage) (interface{}, error){
		"dropbox_auth":               handler.HandleAuth,
		"dropbox_check_auth":         handler.HandleCheckAuth,
		"dropbox_list":               handler.HandleList,
		"dropbox_search":             handler.HandleSearch,
		"dropbox_get_metadata":       handler.HandleGetMetadata,
		"dropbox_download":           handler.HandleDownload,
		"dropbox_upload":             handler.HandleUpload,
		"dropbox_create_folder":      handler.HandleCreateFolder,
		"dropbox_move":               handler.HandleMove,
		"dropbox_copy":               handler.HandleCopy,
		"dropbox_delete":             handler.HandleDelete,
		"dropbox_create_shared_link": handler.HandleCreateSharedLink,
		"dropbox_list_shared_links":  handler.HandleListSharedLinks,
		"dropbox_revoke_shared_link": handler.HandleRevokeSharedLink,
		"dropbox_get_revisions":      handler.HandleGetRevisions,
		"dropbox_restore_file":       handler.HandleRestoreFile,
	}

	handlerFunc, exists := toolHandlers[toolCall.Name]
	if !exists {
		return map[string]interface{}{
			"error": map[string]interface{}{
				"code":    -32601,
				"message": fmt.Sprintf("Unknown tool: %s", toolCall.Name),
			},
		}
	}

	result, err := handlerFunc(toolCall.Arguments)

	if err != nil {
		return map[string]interface{}{
			"error": map[string]interface{}{
				"code":    -32603,
				"message": err.Error(),
			},
		}
	}

	return map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": toJSON(result),
			},
		},
	}
}

func toJSON(v interface{}) string {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("Error marshaling result: %v", err)
	}
	return string(data)
}

func handleListPrompts() interface{} {
	return map[string]interface{}{
		"prompts": []interface{}{},
	}
}

func handleListResources() interface{} {
	return map[string]interface{}{
		"resources": []interface{}{},
	}
}
