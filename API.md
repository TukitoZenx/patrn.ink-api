# API Documentation

Complete API reference for patrn.ink URL Shortener.

## Base URL

```
http://localhost:8080
```

## Authentication

All protected endpoints require a JWT token in the `Authorization` header:

```
Authorization: Bearer <your_jwt_token>
```

Get your JWT token by authenticating via Google OAuth:

1. Visit `/auth/google/login`
2. Complete OAuth flow
3. Receive JWT token in response

---

## Endpoints

### 🔐 Authentication

#### Google Login

```http
GET /auth/google/login
```

Redirects to Google OAuth consent page.

#### OAuth Callback

```http
GET /auth/google/callback?code=<code>&state=<state>
```

Handles OAuth callback and returns JWT token.

**Response:**

```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user": {
    "id": "123456789",
    "email": "user@example.com",
    "name": "John Doe",
    "picture": "https://lh3.googleusercontent.com/...",
    "created_at": "2026-01-18T12:00:00Z"
  }
}
```

---

### 🔗 URL Management

#### Create Short URL

```http
POST /api/shorten
Authorization: Bearer <token>
Content-Type: application/json
```

**Request Body:**

```json
{
  "long_url": "https://example.com/very/long/url",
  "custom_code": "my-link", // Optional
  "expires_in": 168 // Optional, hours
}
```

**Response:** `201 Created`

```json
{
  "short_url": "http://localhost:8080/my-link",
  "short_code": "my-link",
  "long_url": "https://example.com/very/long/url",
  "qr_code_url": "http://localhost:8080/my-link/qr",
  "expires_at": "2026-01-25T12:00:00Z"
}
```

**Validation:**

- `long_url`: Required, must be valid URL
- `custom_code`: 3-20 alphanumeric characters, dashes, underscores
- `expires_in`: Positive integer (hours from now)

#### Get All Links

```http
GET /api/links
Authorization: Bearer <token>
```

**Response:** `200 OK`

```json
{
  "links": [
    {
      "short_code": "abc123",
      "long_url": "https://example.com",
      "user_id": "user123",
      "custom_alias": false,
      "clicks": 42,
      "created_at": "2026-01-18T12:00:00Z",
      "expires_at": null,
      "is_active": true
    }
  ]
}
```

#### Get Link Details

```http
GET /api/links/:code
Authorization: Bearer <token>
```

**Response:** `200 OK`

```json
{
  "short_code": "abc123",
  "long_url": "https://example.com",
  "user_id": "user123",
  "custom_alias": false,
  "clicks": 42,
  "created_at": "2026-01-18T12:00:00Z",
  "is_active": true
}
```

#### Update Link

```http
PUT /api/links/:code
Authorization: Bearer <token>
Content-Type: application/json
```

**Request Body:**

```json
{
  "long_url": "https://new-url.com", // Optional
  "expires_in": 72 // Optional
}
```

**Response:** `200 OK` (updated link object)

#### Delete Link

```http
DELETE /api/links/:code
Authorization: Bearer <token>
```

**Response:** `200 OK`

```json
{
  "message": "Link deleted successfully"
}
```

#### Get Analytics

```http
GET /api/analytics/:code
Authorization: Bearer <token>
```

**Response:** `200 OK`

```json
{
  "total_clicks": 156,
  "unique_clicks": 89,
  "top_referrers": {
    "google.com": 45,
    "twitter.com": 23,
    "direct": 88
  },
  "clicks_by_date": {
    "2026-01-18": 42,
    "2026-01-17": 38
  },
  "device_types": {
    "Desktop": 89,
    "Mobile": 54,
    "Tablet": 13
  },
  "browser_types": {
    "Chrome": 98,
    "Firefox": 32,
    "Safari": 26
  }
}
```

---

### 🌐 Public Endpoints

#### Redirect

```http
GET /:code
```

Redirects to the long URL.

**Responses:**

- `301 Moved Permanently` - Successful redirect
- `404 Not Found` - Link doesn't exist
- `410 Gone` - Link expired or deactivated

#### Get QR Code

```http
GET /:code/qr
```

Returns PNG image of QR code.

**Response:** `200 OK`

```
Content-Type: image/png
<binary PNG data>
```

---

### 🏥 System Endpoints

#### Health Check

```http
GET /health
```

**Response:** `200 OK`

```json
{
  "status": "healthy",
  "redis": true,
  "dynamodb": true,
  "version": "1.0.0"
}
```

#### Prometheus Metrics

```http
GET /metrics
```

**Response:** `200 OK` (Prometheus format)

---

## Error Responses

All errors follow this format:

```json
{
  "error": "Error message description"
}
```

### Status Codes

| Code | Meaning                               |
| ---- | ------------------------------------- |
| 200  | Success                               |
| 201  | Created                               |
| 400  | Bad Request (validation error)        |
| 401  | Unauthorized (missing/invalid token)  |
| 403  | Forbidden (insufficient permissions)  |
| 404  | Not Found                             |
| 409  | Conflict (custom code already exists) |
| 410  | Gone (link expired)                   |
| 429  | Too Many Requests (rate limited)      |
| 500  | Internal Server Error                 |
| 503  | Service Unavailable                   |

---

## Rate Limiting

Protected endpoints are rate limited:

- Default: **100 requests per 60 seconds** per authenticated user
- Unauthenticated: Per IP address
- When exceeded: `429 Too Many Requests`

**Rate Limit Response:**

```json
{
  "error": "Rate limit exceeded",
  "retry_after": 60
}
```

---

## CORS

Allowed origins configured via `ALLOWED_ORIGINS` environment variable.

Default: `http://localhost:3000,http://localhost:8080`

---

## Example cURL Commands

### Complete Workflow

```bash
# 1. Login (visit in browser)
open http://localhost:8080/auth/google/login

# 2. Create short URL
TOKEN="your_jwt_token"
curl -X POST http://localhost:8080/api/shorten \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"long_url":"https://github.com","custom_code":"github"}'

# 3. Get all your links
curl http://localhost:8080/api/links \
  -H "Authorization: Bearer $TOKEN"

# 4. Access the short URL
curl -L http://localhost:8080/github

# 5. Download QR code
curl http://localhost:8080/github/qr -o qr.png

# 6. Get analytics
curl http://localhost:8080/api/analytics/github \
  -H "Authorization: Bearer $TOKEN"

# 7. Delete link
curl -X DELETE http://localhost:8080/api/links/github \
  -H "Authorization: Bearer $TOKEN"
```
