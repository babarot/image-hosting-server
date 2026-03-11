# image-hosting-server

Lightweight Go image hosting server with API key auth, rate limiting, and immutable CDN caching. Includes a browser-based upload UI with GitHub OAuth.

## Features

- API key authentication (constant-time compare)
- IP-based rate limiting (token bucket: 30 req/min, burst 5)
- Extension + MIME sniffing double validation
- Path traversal prevention
- Immutable `Cache-Control` headers for CDN caching
- JSON structured logging
- **Web UI** with drag & drop upload, clipboard paste, and progress display
- **GitHub OAuth** for browser-based authentication

## Web UI

When `GITHUB_CLIENT_ID` is set, a browser-based upload interface is available at `/login`.

<table>
<tr><th>/login</th><th>/ui</th></tr>
<tr>
<td>
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://assets.babarot.dev/files/2026/03/5e40fe943323e228.png">
    <source media="(prefers-color-scheme: light)" srcset="https://assets.babarot.dev/files/2026/03/1d710f0e2d7da0e5.png">
    <img alt="Login" src="https://assets.babarot.dev/files/2026/03/5e40fe943323e228.png" width="250">
  </picture>
</td>
<td>
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://assets.babarot.dev/files/2026/03/336d091b960cb596.png">
    <source media="(prefers-color-scheme: light)" srcset="https://assets.babarot.dev/files/2026/03/8d91995749d59b36.png">
    <img alt="Upload UI" src="https://assets.babarot.dev/files/2026/03/8d91995749d59b36.png" width="250">
  </picture>
</td>
</tr>
</table>


### Setup

1. Create a GitHub OAuth App at [github.com/settings/developers](https://github.com/settings/developers)
   - **Homepage URL**: `https://your-domain.com`
   - **Authorization callback URL**: `https://your-domain.com/auth/callback`
2. Set `GITHUB_CLIENT_ID`, `GITHUB_CLIENT_SECRET`, and `ALLOWED_USERS` environment variables
3. Access `https://your-domain.com/login` in your browser

### Flow

1. Click "Sign in with GitHub" on the login page
2. Authorize the OAuth App on GitHub
3. Drag & drop files (or click / paste) on the upload page
4. Copy the returned URL with the copy button

### Features

- Drag & drop, click to select, and clipboard paste (`Cmd+V`)
- Upload progress bar
- One-click URL copy
- Thumbnail preview for image files
- Dark / light mode (follows OS preference, styled with [terminal.css](https://terminalcss.xyz/))
- Mobile responsive

### Auth Design

- **Upload** (`POST /api/upload`): accepts API key **or** session cookie — CLI and browser both work
- **Delete** (`DELETE /api/delete/{path}`): API key only — no browser delete to prevent accidents
- Sessions are stored in memory (server restart = re-login)
- OAuth state tokens are single-use with 10-minute expiry

## API

| Method | Path | Auth | Description |
|---|---|---|---|
| `POST` | `/api/upload` | API Key or Session | Upload a file |
| `DELETE` | `/api/delete/{path}` | API Key | Delete a file |
| `GET` | `/api/health` | - | Health check |
| `GET` | `/files/{path}` | - | Serve static files (public) |
| `GET` | `/login` | - | Login page (OAuth enabled only) |
| `GET` | `/ui` | Session | Upload UI (OAuth enabled only) |

### Upload

```bash
curl -X POST https://example.com/api/upload \
  -H "X-API-Key: $API_KEY" \
  -F "file=@image.png"
```

- Filename is replaced with a 16-char hex random value
- Subdirectory defaults to `YYYY/MM`
- Allowed formats: jpg, png, gif, webp, svg, avif, ico, pdf
- Max size: 20MB

## Docker

```bash
docker pull ghcr.io/babarot/image-hosting-server:latest
```

```bash
docker run -d \
  -e API_KEY=your-secret-key \
  -e BASE_URL=https://example.com \
  -e GITHUB_CLIENT_ID=your-client-id \
  -e GITHUB_CLIENT_SECRET=your-client-secret \
  -e ALLOWED_USERS=your-github-username \
  -v /path/to/files:/data/files \
  -p 8080:8080 \
  ghcr.io/babarot/image-hosting-server:latest
```

## Environment Variables

| Variable | Description | Default | Required |
|---|---|---|---|
| `API_KEY` | API auth key | - | Yes |
| `BASE_URL` | Public URL base | `http://localhost:8080` | No |
| `UPLOAD_DIR` | File storage path | `/data/images` | No |
| `LISTEN_ADDR` | Listen address | `:8080` | No |
| `GITHUB_CLIENT_ID` | GitHub OAuth client ID | - | For Web UI |
| `GITHUB_CLIENT_SECRET` | GitHub OAuth client secret | - | For Web UI |
| `ALLOWED_USERS` | Comma-separated allowed GitHub usernames | - | For Web UI |

When `GITHUB_CLIENT_ID` is not set, the server runs in API-only mode (no UI routes registered).

## License

MIT
