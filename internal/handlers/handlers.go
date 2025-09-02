package handlers

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/dropbox/dropbox-sdk-go-unofficial/v6/dropbox/files"
	"github.com/dropbox/dropbox-sdk-go-unofficial/v6/dropbox/sharing"
	"go.ngs.io/dropbox-mcp/internal/auth"
	"go.ngs.io/dropbox-mcp/internal/config"
	"go.ngs.io/dropbox-mcp/internal/dropbox"
)

type Handler struct {
	config *config.Config
}

func NewHandler() (*Handler, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	return &Handler{config: cfg}, nil
}

func (h *Handler) HandleAuth(params json.RawMessage) (interface{}, error) {
	var args struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
	}
	
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}
	
	if args.ClientID == "" {
		args.ClientID = os.Getenv("DROPBOX_CLIENT_ID")
	}
	if args.ClientSecret == "" {
		args.ClientSecret = os.Getenv("DROPBOX_CLIENT_SECRET")
	}
	
	if args.ClientID == "" || args.ClientSecret == "" {
		return nil, fmt.Errorf("client_id and client_secret are required (provide as parameters or environment variables)")
	}
	
	authConfig := auth.OAuthConfig{
		ClientID:     args.ClientID,
		ClientSecret: args.ClientSecret,
	}
	
	result, err := auth.StartOAuthFlow(authConfig)
	if err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}
	
	h.config.ClientID = args.ClientID
	h.config.ClientSecret = args.ClientSecret
	h.config.UpdateTokens(result.AccessToken, result.RefreshToken, result.ExpiresAt)
	
	if err := h.config.Save(); err != nil {
		return nil, fmt.Errorf("failed to save configuration: %w", err)
	}
	
	return map[string]interface{}{
		"status":  "authenticated",
		"message": "Successfully authenticated with Dropbox",
	}, nil
}

func (h *Handler) HandleCheckAuth(params json.RawMessage) (interface{}, error) {
	if !h.config.IsTokenValid() {
		return map[string]interface{}{
			"authenticated": false,
			"message":       "Not authenticated. Please run dropbox_auth first.",
		}, nil
	}
	
	if err := auth.ValidateToken(h.config.AccessToken); err != nil {
		return map[string]interface{}{
			"authenticated": false,
			"message":       "Token is invalid or expired. Please re-authenticate.",
		}, nil
	}
	
	return map[string]interface{}{
		"authenticated": true,
		"message":       "Authenticated with Dropbox",
		"expires_at":    h.config.ExpiresAt,
	}, nil
}

func (h *Handler) HandleList(params json.RawMessage) (interface{}, error) {
	var args struct {
		Path string `json:"path"`
	}
	
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}
	
	client, err := dropbox.NewClient(h.config)
	if err != nil {
		return nil, err
	}
	
	entries, err := client.ListFolder(args.Path)
	if err != nil {
		return nil, err
	}
	
	result := make([]map[string]interface{}, 0, len(entries))
	for _, entry := range entries {
		item := map[string]interface{}{}
		
		switch e := entry.(type) {
		case *files.FileMetadata:
			item["name"] = e.Name
			item["path"] = e.PathDisplay
			item["type"] = "file"
			item["size"] = e.Size
			item["modified"] = e.ServerModified
			item["rev"] = e.Rev
		case *files.FolderMetadata:
			item["name"] = e.Name
			item["path"] = e.PathDisplay
			item["type"] = "folder"
		}
		
		result = append(result, item)
	}
	
	return result, nil
}

func (h *Handler) HandleSearch(params json.RawMessage) (interface{}, error) {
	var args struct {
		Query string `json:"query"`
		Path  string `json:"path"`
	}
	
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}
	
	if args.Query == "" {
		return nil, fmt.Errorf("query parameter is required")
	}
	
	client, err := dropbox.NewClient(h.config)
	if err != nil {
		return nil, err
	}
	
	matches, err := client.Search(args.Query, args.Path)
	if err != nil {
		return nil, err
	}
	
	result := make([]map[string]interface{}, 0, len(matches))
	for _, match := range matches {
		metadata := match.Metadata.Metadata
		item := map[string]interface{}{}
		
		switch m := metadata.(type) {
		case *files.FileMetadata:
			item["name"] = m.Name
			item["path"] = m.PathDisplay
			item["type"] = "file"
			item["size"] = m.Size
			item["modified"] = m.ServerModified
		case *files.FolderMetadata:
			item["name"] = m.Name
			item["path"] = m.PathDisplay
			item["type"] = "folder"
		}
		
		result = append(result, item)
	}
	
	return result, nil
}

func (h *Handler) HandleGetMetadata(params json.RawMessage) (interface{}, error) {
	var args struct {
		Path string `json:"path"`
	}
	
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}
	
	if args.Path == "" {
		return nil, fmt.Errorf("path parameter is required")
	}
	
	client, err := dropbox.NewClient(h.config)
	if err != nil {
		return nil, err
	}
	
	metadata, err := client.GetMetadata(args.Path)
	if err != nil {
		return nil, err
	}
	
	result := map[string]interface{}{}
	
	switch m := metadata.(type) {
	case *files.FileMetadata:
		result["name"] = m.Name
		result["path"] = m.PathDisplay
		result["type"] = "file"
		result["size"] = m.Size
		result["modified"] = m.ServerModified
		result["rev"] = m.Rev
		result["content_hash"] = m.ContentHash
	case *files.FolderMetadata:
		result["name"] = m.Name
		result["path"] = m.PathDisplay
		result["type"] = "folder"
		result["id"] = m.Id
	}
	
	return result, nil
}

func (h *Handler) HandleDownload(params json.RawMessage) (interface{}, error) {
	var args struct {
		Path string `json:"path"`
	}
	
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}
	
	if args.Path == "" {
		return nil, fmt.Errorf("path parameter is required")
	}
	
	client, err := dropbox.NewClient(h.config)
	if err != nil {
		return nil, err
	}
	
	data, err := client.Download(args.Path)
	if err != nil {
		return nil, err
	}
	
	if isTextContent(data) {
		return map[string]interface{}{
			"content": string(data),
			"type":    "text",
		}, nil
	}
	
	return map[string]interface{}{
		"content": base64.StdEncoding.EncodeToString(data),
		"type":    "base64",
	}, nil
}

func (h *Handler) HandleUpload(params json.RawMessage) (interface{}, error) {
	var args struct {
		Path    string `json:"path"`
		Content string `json:"content"`
		Mode    string `json:"mode"`
	}
	
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}
	
	if args.Path == "" {
		return nil, fmt.Errorf("path parameter is required")
	}
	if args.Content == "" {
		return nil, fmt.Errorf("content parameter is required")
	}
	
	if args.Mode == "" {
		args.Mode = "add"
	}
	
	client, err := dropbox.NewClient(h.config)
	if err != nil {
		return nil, err
	}
	
	metadata, err := client.Upload(args.Path, args.Content, args.Mode)
	if err != nil {
		return nil, err
	}
	
	return map[string]interface{}{
		"name":     metadata.Name,
		"path":     metadata.PathDisplay,
		"size":     metadata.Size,
		"modified": metadata.ServerModified,
		"rev":      metadata.Rev,
	}, nil
}

func (h *Handler) HandleCreateFolder(params json.RawMessage) (interface{}, error) {
	var args struct {
		Path string `json:"path"`
	}
	
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}
	
	if args.Path == "" {
		return nil, fmt.Errorf("path parameter is required")
	}
	
	client, err := dropbox.NewClient(h.config)
	if err != nil {
		return nil, err
	}
	
	metadata, err := client.CreateFolder(args.Path)
	if err != nil {
		return nil, err
	}
	
	return map[string]interface{}{
		"name": metadata.Name,
		"path": metadata.PathDisplay,
		"id":   metadata.Id,
	}, nil
}

func (h *Handler) HandleMove(params json.RawMessage) (interface{}, error) {
	var args struct {
		FromPath string `json:"from_path"`
		ToPath   string `json:"to_path"`
	}
	
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}
	
	if args.FromPath == "" || args.ToPath == "" {
		return nil, fmt.Errorf("from_path and to_path parameters are required")
	}
	
	client, err := dropbox.NewClient(h.config)
	if err != nil {
		return nil, err
	}
	
	metadata, err := client.Move(args.FromPath, args.ToPath)
	if err != nil {
		return nil, err
	}
	
	result := map[string]interface{}{}
	
	switch m := metadata.(type) {
	case *files.FileMetadata:
		result["name"] = m.Name
		result["path"] = m.PathDisplay
		result["type"] = "file"
		result["size"] = m.Size
		result["modified"] = m.ServerModified
	case *files.FolderMetadata:
		result["name"] = m.Name
		result["path"] = m.PathDisplay
		result["type"] = "folder"
	}
	
	return result, nil
}

func (h *Handler) HandleCopy(params json.RawMessage) (interface{}, error) {
	var args struct {
		FromPath string `json:"from_path"`
		ToPath   string `json:"to_path"`
	}
	
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}
	
	if args.FromPath == "" || args.ToPath == "" {
		return nil, fmt.Errorf("from_path and to_path parameters are required")
	}
	
	client, err := dropbox.NewClient(h.config)
	if err != nil {
		return nil, err
	}
	
	metadata, err := client.Copy(args.FromPath, args.ToPath)
	if err != nil {
		return nil, err
	}
	
	result := map[string]interface{}{}
	
	switch m := metadata.(type) {
	case *files.FileMetadata:
		result["name"] = m.Name
		result["path"] = m.PathDisplay
		result["type"] = "file"
		result["size"] = m.Size
		result["modified"] = m.ServerModified
	case *files.FolderMetadata:
		result["name"] = m.Name
		result["path"] = m.PathDisplay
		result["type"] = "folder"
	}
	
	return result, nil
}

func (h *Handler) HandleDelete(params json.RawMessage) (interface{}, error) {
	var args struct {
		Path string `json:"path"`
	}
	
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}
	
	if args.Path == "" {
		return nil, fmt.Errorf("path parameter is required")
	}
	
	client, err := dropbox.NewClient(h.config)
	if err != nil {
		return nil, err
	}
	
	if err := client.Delete(args.Path); err != nil {
		return nil, err
	}
	
	return map[string]interface{}{
		"status":  "success",
		"message": fmt.Sprintf("Successfully deleted %s", args.Path),
	}, nil
}

func (h *Handler) HandleCreateSharedLink(params json.RawMessage) (interface{}, error) {
	var args struct {
		Path     string                 `json:"path"`
		Settings map[string]interface{} `json:"settings"`
	}
	
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}
	
	if args.Path == "" {
		return nil, fmt.Errorf("path parameter is required")
	}
	
	client, err := dropbox.NewClient(h.config)
	if err != nil {
		return nil, err
	}
	
	url, err := client.CreateSharedLink(args.Path, args.Settings)
	if err != nil {
		return nil, err
	}
	
	return map[string]interface{}{
		"url":  url,
		"path": args.Path,
	}, nil
}

func (h *Handler) HandleListSharedLinks(params json.RawMessage) (interface{}, error) {
	var args struct {
		Path string `json:"path"`
	}
	
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}
	
	client, err := dropbox.NewClient(h.config)
	if err != nil {
		return nil, err
	}
	
	links, err := client.ListSharedLinks(args.Path)
	if err != nil {
		return nil, err
	}
	
	result := make([]map[string]interface{}, 0, len(links))
	for _, link := range links {
		item := map[string]interface{}{}
		
		switch l := link.(type) {
		case *sharing.FileLinkMetadata:
			item["url"] = l.Url
			item["name"] = l.Name
			item["path"] = l.PathLower
			if l.Expires != nil {
				item["expires"] = l.Expires.UTC().Format(time.RFC3339)
			}
		case *sharing.FolderLinkMetadata:
			item["url"] = l.Url
			item["name"] = l.Name
			item["path"] = l.PathLower
			if l.Expires != nil {
				item["expires"] = l.Expires.UTC().Format(time.RFC3339)
			}
		}
		
		result = append(result, item)
	}
	
	return result, nil
}

func (h *Handler) HandleRevokeSharedLink(params json.RawMessage) (interface{}, error) {
	var args struct {
		URL string `json:"url"`
	}
	
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}
	
	if args.URL == "" {
		return nil, fmt.Errorf("url parameter is required")
	}
	
	client, err := dropbox.NewClient(h.config)
	if err != nil {
		return nil, err
	}
	
	if err := client.RevokeSharedLink(args.URL); err != nil {
		return nil, err
	}
	
	return map[string]interface{}{
		"status":  "success",
		"message": "Shared link revoked successfully",
	}, nil
}

func (h *Handler) HandleGetRevisions(params json.RawMessage) (interface{}, error) {
	var args struct {
		Path string `json:"path"`
	}
	
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}
	
	if args.Path == "" {
		return nil, fmt.Errorf("path parameter is required")
	}
	
	client, err := dropbox.NewClient(h.config)
	if err != nil {
		return nil, err
	}
	
	revisions, err := client.GetRevisions(args.Path)
	if err != nil {
		return nil, err
	}
	
	result := make([]map[string]interface{}, 0, len(revisions))
	for _, rev := range revisions {
		result = append(result, map[string]interface{}{
			"rev":      rev.Rev,
			"size":     rev.Size,
			"modified": rev.ServerModified,
		})
	}
	
	return result, nil
}

func (h *Handler) HandleRestoreFile(params json.RawMessage) (interface{}, error) {
	var args struct {
		Path string `json:"path"`
		Rev  string `json:"rev"`
	}
	
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}
	
	if args.Path == "" || args.Rev == "" {
		return nil, fmt.Errorf("path and rev parameters are required")
	}
	
	client, err := dropbox.NewClient(h.config)
	if err != nil {
		return nil, err
	}
	
	metadata, err := client.RestoreFile(args.Path, args.Rev)
	if err != nil {
		return nil, err
	}
	
	return map[string]interface{}{
		"name":     metadata.Name,
		"path":     metadata.PathDisplay,
		"size":     metadata.Size,
		"modified": metadata.ServerModified,
		"rev":      metadata.Rev,
	}, nil
}

func isTextContent(data []byte) bool {
	if len(data) == 0 {
		return true
	}
	
	for _, b := range data {
		if b == 0 {
			return false
		}
		if b < 32 && b != '\t' && b != '\n' && b != '\r' {
			return false
		}
	}
	
	return true
}