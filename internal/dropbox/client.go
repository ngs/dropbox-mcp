package dropbox

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/dropbox/dropbox-sdk-go-unofficial/v6/dropbox"
	"github.com/dropbox/dropbox-sdk-go-unofficial/v6/dropbox/files"
	"github.com/dropbox/dropbox-sdk-go-unofficial/v6/dropbox/sharing"
	"go.ngs.io/dropbox-mcp/internal/auth"
	"go.ngs.io/dropbox-mcp/internal/config"
)

type Client struct {
	filesClient   files.Client
	sharingClient sharing.Client
	config        *config.Config
}

func NewClient(cfg *config.Config) (*Client, error) {
	if cfg.NeedsRefresh() && cfg.RefreshToken != "" {
		authConfig := auth.OAuthConfig{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
		}
		result, err := auth.RefreshToken(authConfig, cfg.RefreshToken)
		if err != nil {
			return nil, fmt.Errorf("failed to refresh token: %w", err)
		}
		cfg.UpdateTokens(result.AccessToken, result.RefreshToken, result.ExpiresAt)
		if err := cfg.Save(); err != nil {
			return nil, fmt.Errorf("failed to save updated config: %w", err)
		}
	}

	if !cfg.IsTokenValid() {
		return nil, fmt.Errorf("invalid or expired token")
	}

	dbxConfig := dropbox.Config{
		Token: cfg.AccessToken,
	}

	return &Client{
		filesClient:   files.New(dbxConfig),
		sharingClient: sharing.New(dbxConfig),
		config:        cfg,
	}, nil
}

func (c *Client) ListFolder(path string) ([]files.IsMetadata, error) {
	if path == "" {
		path = ""
	}

	arg := files.NewListFolderArg(path)
	arg.Recursive = false
	arg.IncludeDeleted = false

	res, err := c.filesClient.ListFolder(arg)
	if err != nil {
		return nil, fmt.Errorf("failed to list folder: %w", err)
	}

	entries := res.Entries
	for res.HasMore {
		arg := files.NewListFolderContinueArg(res.Cursor)
		res, err = c.filesClient.ListFolderContinue(arg)
		if err != nil {
			return nil, fmt.Errorf("failed to continue listing: %w", err)
		}
		entries = append(entries, res.Entries...)
	}

	return entries, nil
}

func (c *Client) Search(query string, path string) ([]*files.SearchMatchV2, error) {
	options := files.NewSearchOptions()
	if path != "" {
		options.Path = path
	}
	options.MaxResults = 100

	arg := files.NewSearchV2Arg(query)
	arg.Options = options

	res, err := c.filesClient.SearchV2(arg)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	matches := res.Matches
	// Note: Search pagination is not currently supported in the SDK version we're using
	// Only return first page of results

	return matches, nil
}

func (c *Client) GetMetadata(path string) (files.IsMetadata, error) {
	arg := files.NewGetMetadataArg(path)
	return c.filesClient.GetMetadata(arg)
}

func (c *Client) Download(path string) ([]byte, error) {
	arg := files.NewDownloadArg(path)
	_, content, err := c.filesClient.Download(arg)
	if err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}
	defer content.Close()

	data, err := io.ReadAll(content)
	if err != nil {
		return nil, fmt.Errorf("failed to read content: %w", err)
	}

	return data, nil
}

func (c *Client) Upload(path, content string, mode string) (*files.FileMetadata, error) {
	var data []byte
	
	if strings.Contains(content, "\n") || !isBase64(content) {
		data = []byte(content)
	} else {
		decoded, err := base64.StdEncoding.DecodeString(content)
		if err != nil {
			data = []byte(content)
		} else {
			data = decoded
		}
	}

	commitInfo := files.NewCommitInfo(path)
	if mode == "overwrite" {
		commitInfo.Mode = &files.WriteMode{Tagged: dropbox.Tagged{Tag: "overwrite"}}
	} else {
		commitInfo.Mode = &files.WriteMode{Tagged: dropbox.Tagged{Tag: "add"}}
	}
	commitInfo.Autorename = true
	now := time.Now().UTC()
	commitInfo.ClientModified = &now

	reader := bytes.NewReader(data)
	
	if len(data) > 150*1024*1024 {
		return c.uploadLarge(commitInfo, reader)
	}

	arg := files.NewUploadArg(path)
	arg.Mode = commitInfo.Mode
	arg.Autorename = commitInfo.Autorename
	arg.ClientModified = commitInfo.ClientModified
	return c.filesClient.Upload(arg, reader)
}

func (c *Client) uploadLarge(commitInfo *files.CommitInfo, reader io.Reader) (*files.FileMetadata, error) {
	const chunkSize = 4 * 1024 * 1024

	sessionArg := files.NewUploadSessionStartArg()
	sessionArg.Close = false
	session, err := c.filesClient.UploadSessionStart(sessionArg, bytes.NewReader([]byte{}))
	if err != nil {
		return nil, fmt.Errorf("failed to start upload session: %w", err)
	}

	offset := uint64(0)
	buffer := make([]byte, chunkSize)

	for {
		n, err := reader.Read(buffer)
		if n > 0 {
			cursor := files.NewUploadSessionCursor(session.SessionId, offset)
			appendArg := files.NewUploadSessionAppendArg(cursor)
			
			if err := c.filesClient.UploadSessionAppendV2(appendArg, bytes.NewReader(buffer[:n])); err != nil {
				return nil, fmt.Errorf("failed to append chunk: %w", err)
			}
			offset += uint64(n)
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read chunk: %w", err)
		}
	}

	cursor := files.NewUploadSessionCursor(session.SessionId, offset)
	finishArg := files.NewUploadSessionFinishArg(cursor, commitInfo)
	
	return c.filesClient.UploadSessionFinish(finishArg, nil)
}

func (c *Client) CreateFolder(path string) (*files.FolderMetadata, error) {
	arg := files.NewCreateFolderArg(path)
	arg.Autorename = false
	
	result, err := c.filesClient.CreateFolderV2(arg)
	if err != nil {
		return nil, fmt.Errorf("failed to create folder: %w", err)
	}
	
	// result.Metadata is already a *files.FolderMetadata
	return result.Metadata, nil
}

func (c *Client) Move(fromPath, toPath string) (files.IsMetadata, error) {
	arg := files.NewRelocationArg(fromPath, toPath)
	arg.Autorename = false
	arg.AllowOwnershipTransfer = false
	
	result, err := c.filesClient.MoveV2(arg)
	if err != nil {
		return nil, fmt.Errorf("move failed: %w", err)
	}
	
	return result.Metadata, nil
}

func (c *Client) Copy(fromPath, toPath string) (files.IsMetadata, error) {
	arg := files.NewRelocationArg(fromPath, toPath)
	arg.Autorename = false
	
	result, err := c.filesClient.CopyV2(arg)
	if err != nil {
		return nil, fmt.Errorf("copy failed: %w", err)
	}
	
	return result.Metadata, nil
}

func (c *Client) Delete(path string) error {
	arg := files.NewDeleteArg(path)
	
	_, err := c.filesClient.DeleteV2(arg)
	if err != nil {
		return fmt.Errorf("delete failed: %w", err)
	}
	
	return nil
}

func (c *Client) CreateSharedLink(path string, settings map[string]interface{}) (string, error) {
	arg := sharing.NewCreateSharedLinkWithSettingsArg(path)
	
	if settings != nil {
		linkSettings := &sharing.SharedLinkSettings{}
		
		if expires, ok := settings["expires"].(string); ok {
			t, err := time.Parse(time.RFC3339, expires)
			if err == nil {
				linkSettings.Expires = &t
			}
		}
		
		if password, ok := settings["password"].(string); ok {
			linkSettings.LinkPassword = password
		}
		
		arg.Settings = linkSettings
	}
	
	result, err := c.sharingClient.CreateSharedLinkWithSettings(arg)
	if err != nil {
		if strings.Contains(err.Error(), "shared_link_already_exists") {
			links, listErr := c.ListSharedLinks(path)
			if listErr == nil && len(links) > 0 {
				switch l := links[0].(type) {
				case *sharing.FileLinkMetadata:
					return l.Url, nil
				case *sharing.FolderLinkMetadata:
					return l.Url, nil
				}
			}
		}
		return "", fmt.Errorf("failed to create shared link: %w", err)
	}
	
	if metadata, ok := result.(*sharing.FileLinkMetadata); ok {
		return metadata.Url, nil
	} else if metadata, ok := result.(*sharing.FolderLinkMetadata); ok {
		return metadata.Url, nil
	}
	return "", fmt.Errorf("unexpected shared link type")
}

func (c *Client) ListSharedLinks(path string) ([]sharing.IsSharedLinkMetadata, error) {
	arg := sharing.NewListSharedLinksArg()
	arg.Path = path
	
	result, err := c.sharingClient.ListSharedLinks(arg)
	if err != nil {
		return nil, fmt.Errorf("failed to list shared links: %w", err)
	}
	
	return result.Links, nil
}

func (c *Client) RevokeSharedLink(url string) error {
	arg := sharing.NewRevokeSharedLinkArg(url)
	
	err := c.sharingClient.RevokeSharedLink(arg)
	if err != nil {
		return fmt.Errorf("failed to revoke shared link: %w", err)
	}
	
	return nil
}

func (c *Client) GetRevisions(path string) ([]*files.FileMetadata, error) {
	arg := files.NewListRevisionsArg(path)
	arg.Limit = 100
	
	result, err := c.filesClient.ListRevisions(arg)
	if err != nil {
		return nil, fmt.Errorf("failed to get revisions: %w", err)
	}
	
	return result.Entries, nil
}

func (c *Client) RestoreFile(path, rev string) (*files.FileMetadata, error) {
	arg := files.NewRestoreArg(path, rev)
	
	return c.filesClient.Restore(arg)
}

func isBase64(s string) bool {
	_, err := base64.StdEncoding.DecodeString(s)
	return err == nil
}