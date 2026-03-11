# image-hosting-server

Lightweight Go image hosting server with API key auth, rate limiting, and immutable CDN caching.

## Features

- API key authentication (constant-time compare)
- IP-based rate limiting (token bucket: 30 req/min, burst 5)
- Extension + MIME sniffing double validation
- Path traversal prevention
- Immutable `Cache-Control` headers for CDN caching
- JSON structured logging

## API

| Method | Path | Auth | Description |
|---|---|---|---|
| `POST` | `/api/upload` | API Key | Upload a file |
| `DELETE` | `/api/delete/{path}` | API Key | Delete a file |
| `GET` | `/api/health` | - | Health check |
| `GET` | `/files/{path}` | - | Serve static files (public) |

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
  -e IMAGE_HOSTING_API_KEY=your-secret-key \
  -e IMAGE_HOSTING_BASE_URL=https://example.com \
  -v /path/to/files:/data/files \
  -p 8080:8080 \
  ghcr.io/babarot/image-hosting-server:latest
```

## Environment Variables

| Variable | Description | Default |
|---|---|---|
| `IMAGE_HOSTING_API_KEY` | API auth key (required) | - |
| `IMAGE_HOSTING_BASE_URL` | Public URL base | `http://localhost:8080` |
| `IMAGE_HOSTING_UPLOAD_DIR` | File storage path (inside container) | `/data/images` |
| `IMAGE_HOSTING_LISTEN_ADDR` | Listen address | `:8080` |
| `GITHUB_CLIENT_ID` | GitHub OAuth client ID | - |
| `GITHUB_CLIENT_SECRET` | GitHub OAuth client secret | - |
| `GITHUB_ALLOWED_USERS` | Comma-separated list of allowed GitHub usernames | - |

## License

MIT
