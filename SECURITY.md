# Security Considerations

## Client Credentials Management

### ⚠️ IMPORTANT: Never embed CLIENT_SECRET in binaries

Embedding CLIENT_SECRET in distributed binaries poses significant security risks:
- Binaries can be reverse-engineered
- Secrets can be extracted using tools like `strings`
- Compromised secrets allow unauthorized access to your Dropbox app

### Recommended Approaches

#### 1. Environment Variables (Current Implementation)
Users provide their own Dropbox App credentials:
```bash
export DROPBOX_CLIENT_ID="your_app_id"
export DROPBOX_CLIENT_SECRET="your_app_secret"
```

#### 2. OAuth 2.0 with PKCE (Recommended for Public Clients)
For public distribution without CLIENT_SECRET:
- Use Authorization Code flow with PKCE
- No CLIENT_SECRET required
- Suitable for desktop applications

#### 3. Configuration File with Proper Permissions
Store credentials in a protected config file:
```bash
chmod 600 ~/.dropbox-mcp/config.json
```

### For Developers

If you fork this project:
1. **Never commit credentials** to version control
2. **Use your own Dropbox App** for testing
3. **Educate users** to create their own Dropbox Apps
4. Consider implementing PKCE flow for public distribution

### For Users

1. **Create your own Dropbox App** at https://www.dropbox.com/developers/apps
2. **Keep your CLIENT_SECRET private**
3. **Use environment variables** or secure config files
4. **Never share** your credentials

### Security Best Practices

- Rotate credentials regularly
- Use app-specific Dropbox Apps with minimal permissions
- Monitor app activity in Dropbox App Console
- Revoke access immediately if credentials are compromised

## Reporting Security Issues

If you discover a security vulnerability, please email security@ngs.io instead of using the public issue tracker.