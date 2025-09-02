# Dropbox MCP Server - Development Guide

## Overview
This is a Model Context Protocol (MCP) server for Dropbox integration, written in Go. It allows Claude to interact with Dropbox files and folders through a standardized protocol.

## Project Structure
```
dropbox-mcp/
├── main.go                 # MCP server implementation with stdio transport
├── go.mod                  # Go module definition
├── internal/
│   ├── auth/              # OAuth 2.0 authentication flow
│   ├── config/            # Configuration and token management
│   ├── dropbox/           # Dropbox API client wrapper
│   └── handlers/          # MCP tool handlers
├── mcp.json               # MCP server metadata
└── README.md              # User documentation
```

## Key Components

### Authentication (internal/auth/auth.go)
- Implements OAuth 2.0 browser-based flow
- Opens local HTTP server to receive callback
- Handles token refresh automatically
- Stores tokens in `~/.dropbox-mcp/config.json`

### MCP Protocol (main.go)
- Uses stdio transport for communication
- Handles JSON-RPC 2.0 messages
- Skips notifications (messages without ID field)
- Implements required MCP methods:
  - `initialize` - Protocol handshake
  - `tools/list` - List available tools
  - `tools/call` - Execute tool functions
  - `prompts/list` - Returns empty list
  - `resources/list` - Returns empty list

### Dropbox Client (internal/dropbox/client.go)
- Wraps Dropbox SDK for Go
- Handles large file uploads (>150MB) with chunked transfer
- Automatic token refresh when needed
- Type assertions for metadata interfaces

## Available Tools

### Authentication
- `dropbox_auth` - Start OAuth flow
- `dropbox_check_auth` - Verify authentication status

### File Operations
- `dropbox_list` - List folder contents
- `dropbox_search` - Search files (note: pagination not supported in current SDK)
- `dropbox_get_metadata` - Get file/folder metadata
- `dropbox_download` - Download file content
- `dropbox_upload` - Upload file (supports base64 and text)
- `dropbox_create_folder` - Create new folder
- `dropbox_move` - Move or rename
- `dropbox_copy` - Copy files/folders
- `dropbox_delete` - Delete files/folders

### Sharing
- `dropbox_create_shared_link` - Create shareable link
- `dropbox_list_shared_links` - List existing links
- `dropbox_revoke_shared_link` - Revoke shared link

### Version Control
- `dropbox_get_revisions` - Get file version history
- `dropbox_restore_file` - Restore to specific version

## Building and Testing

### Build
```bash
go build -o dropbox-mcp
```

### Run Tests
```bash
go test ./...
```

### Manual Testing
```bash
# Test with direct stdio
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{}}}' | ./dropbox-mcp

# Check logs in Claude Desktop
tail -f ~/Library/Logs/Claude/mcp-server-dropbox.log
```

## Configuration

### Environment Variables
- `DROPBOX_CLIENT_ID` - Dropbox App key
- `DROPBOX_CLIENT_SECRET` - Dropbox App secret

### Config File
Location: `~/.dropbox-mcp/config.json`
```json
{
  "client_id": "...",
  "client_secret": "...",
  "access_token": "...",
  "refresh_token": "...",
  "expires_at": "2024-01-01T00:00:00Z"
}
```

## Common Issues and Solutions

### Error UI in Claude Desktop
- Ensure notifications are not sending responses
- Check that ID field is properly handled (can be null for notifications)
- Verify `prompts/list` and `resources/list` return empty arrays

### Authentication Failures
- Verify redirect URI is set to `http://localhost:<port>/callback` in Dropbox App
- Check that all required scopes are enabled
- Ensure client ID and secret are correct

### File Upload/Download Issues
- Text vs binary detection uses simple heuristic
- Large files (>150MB) use chunked upload automatically
- Base64 detection checks for newlines and valid encoding

## Development Tips

1. **Error Handling**: Always wrap Dropbox API calls with descriptive error messages
2. **Type Assertions**: Dropbox SDK uses interfaces heavily - use type switches
3. **Token Management**: Token refresh is automatic but check `NeedsRefresh()` 
4. **MCP Protocol**: Remember to skip responses for notification messages
5. **Logging**: Use stderr for debug output to avoid interfering with stdio transport

## Future Improvements

- [ ] Add support for search pagination when SDK supports it
- [ ] Implement file change notifications using webhooks
- [ ] Add batch operations for better performance
- [ ] Support for Paper documents
- [ ] Team folder operations
- [ ] More granular permission handling

## Dependencies

- `github.com/dropbox/dropbox-sdk-go-unofficial/v6` - Dropbox SDK
- `github.com/pkg/browser` - Browser launcher for OAuth
- `golang.org/x/oauth2` - OAuth 2.0 implementation

## Security Notes

- Tokens stored with 0600 permissions
- State parameter used in OAuth flow to prevent CSRF
- Client credentials can be provided via env vars to avoid hardcoding
- Never log or expose access tokens

## References

- [MCP Specification](https://modelcontextprotocol.io/docs)
- [Dropbox API Documentation](https://www.dropbox.com/developers/documentation/http/documentation)
- [Dropbox SDK for Go](https://github.com/dropbox/dropbox-sdk-go-unofficial)