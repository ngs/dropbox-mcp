# Dropbox MCP Server

A Model Context Protocol (MCP) server implementation for Dropbox integration, written in Go. This server allows AI assistants like Claude to interact with Dropbox through a standardized protocol.

## Features

- **OAuth 2.0 Authentication**: Secure authentication with Dropbox using browser-based OAuth flow
- **File Operations**: List, search, download, upload, move, copy, and delete files
- **Folder Management**: Create folders and navigate directory structures
- **Sharing**: Create, list, and revoke shared links
- **Version Control**: View file revision history and restore previous versions
- **Large File Support**: Automatic chunked upload for files over 150MB

## Prerequisites

- A Dropbox account
- Claude Desktop application
- Go 1.21 or higher (only for building from source)

## Installation

### Option 1: Install with Homebrew (macOS/Linux)

```bash
brew tap ngs/tap
brew install dropbox-mcp-server
```

### Option 2: Install with Go

```bash
go install go.ngs.io/dropbox-mcp-server@latest
```

### Option 3: Download Pre-built Binary

Download the latest release for your platform from the [releases page](https://github.com/ngs/dropbox-mcp-server/releases).

```bash
# Example for macOS (Apple Silicon)
curl -L https://github.com/ngs/dropbox-mcp-server/releases/latest/download/dropbox-mcp-server_darwin_arm64.tar.gz | tar xz
sudo mv dropbox-mcp-server /usr/local/bin/

# Example for macOS (Intel)
curl -L https://github.com/ngs/dropbox-mcp-server/releases/latest/download/dropbox-mcp-server_darwin_amd64.tar.gz | tar xz
sudo mv dropbox-mcp-server /usr/local/bin/

# Example for Linux (x86_64)
curl -L https://github.com/ngs/dropbox-mcp-server/releases/latest/download/dropbox-mcp-server_linux_amd64.tar.gz | tar xz
sudo mv dropbox-mcp-server /usr/local/bin/
```

### Option 4: Build from Source

```bash
git clone https://github.com/ngs/dropbox-mcp-server.git
cd dropbox-mcp-server
go build -o dropbox-mcp-server
```

## Setup

### ⚠️ Security Notice

**IMPORTANT**: Each user must create their own Dropbox App. Never share or embed CLIENT_SECRET in binaries. See [SECURITY.md](SECURITY.md) for details.

### 1. Create a Dropbox App

1. Go to [Dropbox App Console](https://www.dropbox.com/developers/apps)
2. Click "Create app"
3. Choose configuration:
   - **Choose an API**: Select "Scoped access"
   - **Choose the type of access**: Select "Full Dropbox" or "App folder" based on your needs
   - **Name your app**: Enter a unique name for your app
4. After creation, go to the app's settings page
5. Under "OAuth 2", add redirect URI: `http://localhost:8080/callback`
6. Note down your **App key** (Client ID) and **App secret** (Client Secret)
7. In the "Permissions" tab, ensure the following scopes are selected:
   - `files.content.read` - View content of your Dropbox files and folders
   - `files.content.write` - Edit content of your Dropbox files and folders
   - `files.metadata.read` - View information about your Dropbox files and folders
   - `files.metadata.write` - View and edit information about your Dropbox files and folders
   - `sharing.read` - View your shared files and folders
   - `sharing.write` - Create and modify your shared files and folders

### 2. Configure Claude Desktop

#### Option A: Using Claude MCP CLI (Recommended)

If you have Claude MCP CLI installed, you can register the server with a single command:

```bash
# Basic registration (replace with YOUR OWN App credentials)
claude mcp add dropbox dropbox-mcp-server \
  --env DROPBOX_CLIENT_ID=your_own_app_key \
  --env DROPBOX_CLIENT_SECRET=your_own_app_secret

# With custom binary path
claude mcp add dropbox /path/to/dropbox-mcp-server \
  --env DROPBOX_CLIENT_ID=your_own_app_key \
  --env DROPBOX_CLIENT_SECRET=your_own_app_secret
```

⚠️ **Security**: Use credentials from YOUR OWN Dropbox App. Never use shared credentials.

#### Option B: Manual Configuration

Add the following to your Claude Desktop configuration file:

**macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`
**Windows**: `%APPDATA%\Claude\claude_desktop_config.json`
**Linux**: `~/.config/Claude/claude_desktop_config.json`

```json
{
  "mcpServers": {
    "dropbox": {
      "command": "dropbox-mcp-server",
      "env": {
        "DROPBOX_CLIENT_ID": "your_app_key_here",
        "DROPBOX_CLIENT_SECRET": "your_app_secret_here"
      }
    }
  }
}
```

**Note**: 
- If you installed via Homebrew or placed the binary in `/usr/local/bin`, you can use just `"command": "dropbox-mcp-server"`
- If you built from source or downloaded to a custom location, use the full path: `"command": "/path/to/dropbox-mcp-server"`

### 3. Verify Installation

After configuration, restart Claude Desktop and verify the server is connected:

```bash
# List registered MCP servers (if using Claude MCP CLI)
claude mcp list

# Remove a server if needed
claude mcp remove dropbox
```

## Usage

### Initial Authentication

When you first use the Dropbox MCP server in Claude:

1. Use the `dropbox_auth` tool to authenticate
2. Your browser will open to Dropbox's authorization page
3. Log in and authorize the app
4. You'll be redirected to a success page
5. The authentication token will be saved to `~/.dropbox-mcp-server/config.json`

### Available Tools

#### Authentication
- `dropbox_auth` - Authenticate with Dropbox
- `dropbox_check_auth` - Check authentication status

#### File Operations
- `dropbox_list` - List files and folders
- `dropbox_search` - Search for files
- `dropbox_get_metadata` - Get file/folder metadata
- `dropbox_download` - Download file content
- `dropbox_upload` - Upload a file
- `dropbox_create_folder` - Create a new folder
- `dropbox_move` - Move or rename files/folders
- `dropbox_copy` - Copy files/folders
- `dropbox_delete` - Delete files/folders

#### Sharing
- `dropbox_create_shared_link` - Create a shared link
- `dropbox_list_shared_links` - List existing shared links
- `dropbox_revoke_shared_link` - Revoke a shared link

#### Version Control
- `dropbox_get_revisions` - Get file revision history
- `dropbox_restore_file` - Restore a file to a previous version

### Example Commands in Claude

```
"Please authenticate with Dropbox"
"List all files in my Dropbox root folder"
"Search for PDF files containing 'invoice'"
"Upload this text to /Documents/notes.txt"
"Create a shared link for /Photos/vacation.jpg"
"Show me the revision history of /Documents/report.docx"
```

## Configuration

The server stores configuration in `~/.dropbox-mcp-server/config.json`:

```json
{
  "client_id": "your_client_id",
  "client_secret": "your_client_secret",
  "access_token": "your_access_token",
  "refresh_token": "your_refresh_token",
  "expires_at": "2024-01-01T00:00:00Z"
}
```

Tokens are automatically refreshed when they expire.

## Security Considerations

- The configuration file contains sensitive tokens and is stored with 0600 permissions
- Client credentials can be provided via environment variables instead of config file
- OAuth flow uses state parameter to prevent CSRF attacks
- All API calls use HTTPS

## Troubleshooting

### Authentication Issues
- Ensure redirect URI is correctly configured in Dropbox App Console
- Check that client ID and secret are correct
- Try deleting `~/.dropbox-mcp-server/config.json` and re-authenticating

### Permission Errors
- Verify your Dropbox app has the required scopes enabled
- Check file paths are correct (use forward slashes, start with /)

### Connection Issues
- Ensure you have internet connectivity
- Check if Dropbox API is accessible from your network
- Review stderr output for detailed error messages

## Development

### Building from Source

```bash
go mod download
go build -o dropbox-mcp-server
```

### Running Tests

```bash
go test ./...
```

### Project Structure

```
dropbox-mcp-server/
├── main.go                 # MCP server implementation
├── go.mod                  # Go module definition
├── internal/
│   ├── auth/              # OAuth authentication
│   ├── config/            # Configuration management
│   ├── dropbox/           # Dropbox API client
│   └── handlers/          # MCP tool handlers
├── mcp.json               # MCP server metadata
└── README.md              # This file
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

MIT License - see LICENSE file for details

## Support

For issues and questions, please open an issue on GitHub.