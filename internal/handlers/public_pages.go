package handlers

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"html/template"
	"net/http"
	neturl "net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"patrn.ink/internal/config"
	"patrn.ink/internal/models"
)

type publicPageData struct {
	PageTitle          string
	Eyebrow            string
	Heading            string
	Body               string
	Detail             string
	Code               string
	Domain             string
	PreviewTitle       string
	PreviewDescription string
	ActionURL          string
	ActionLabel        string
	SecondaryURL       string
	SecondaryLabel     string
	PasswordError      string
	AgeError           string
	AgeLabel           string
	AgeLevel           int
	ShowPasswordForm   bool
	ShowAgeForm        bool
	BrandImageURL      string
	FaviconURL         string
}

const ageProofTTL = 10 * time.Minute

var publicPageTemplate = template.Must(template.New("public-page").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title>{{.PageTitle}}</title>
  <link rel="icon" type="image/x-icon" href="{{.FaviconURL}}" />
  <link rel="apple-touch-icon" href="{{.FaviconURL}}" />
  <style>
    :root {
      color-scheme: dark;
      --bg: #101917;
      --bg-alt: #162321;
      --surface: rgba(21, 33, 31, 0.96);
      --surface-2: rgba(16, 26, 24, 0.98);
      --border: rgba(164, 201, 190, 0.14);
      --text: #ebf4f1;
      --text-muted: #b7c9c3;
      --text-soft: #7b928b;
      --primary: #46c9b8;
      --primary-strong: #11766e;
      --primary-soft: rgba(70, 201, 184, 0.14);
      --warning: #ffc770;
      --error: #ff8f82;
      --success: #5fd6a2;
      --shadow: 0 22px 60px rgba(0, 0, 0, 0.38);
    }

    * { box-sizing: border-box; }

    body {
      margin: 0;
      min-height: 100vh;
      background:
        radial-gradient(circle at top left, rgba(70, 201, 184, 0.18), transparent 28%),
        radial-gradient(circle at top right, rgba(17, 118, 110, 0.12), transparent 24%),
        linear-gradient(180deg, #101917 0%, #0b1210 100%);
      color: var(--text);
      font-family: "Segoe UI", Inter, system-ui, sans-serif;
      display: flex;
      align-items: center;
      justify-content: center;
      padding: 32px 18px;
    }

    .shell {
      width: min(100%, 980px);
      display: grid;
      gap: 18px;
    }

    .brand {
      display: inline-flex;
      align-items: center;
      gap: 12px;
      color: var(--text);
      text-decoration: none;
      font-weight: 700;
      letter-spacing: -0.02em;
    }

    .brand-logo {
      width: 46px;
      height: 46px;
      border-radius: 16px;
      display: block;
      box-shadow:
        0 16px 32px rgba(0, 0, 0, 0.22),
        0 0 0 1px rgba(255, 255, 255, 0.04);
    }

    .panel {
      background: linear-gradient(180deg, rgba(14, 20, 19, 0.96), rgba(8, 12, 11, 0.98));
      border: 1px solid var(--border);
      border-radius: 30px;
      box-shadow: var(--shadow);
      overflow: hidden;
    }

    .panel-grid {
      display: grid;
      gap: 0;
    }

    @media (min-width: 880px) {
      .panel-grid {
        grid-template-columns: 1.05fr 0.95fr;
      }
    }

    .main {
      padding: 34px 28px 30px;
      background:
        radial-gradient(circle at top right, rgba(70, 201, 184, 0.1), transparent 30%),
        linear-gradient(180deg, rgba(10, 18, 17, 0.94), rgba(8, 12, 11, 0.94));
    }

    .side {
      padding: 28px;
      border-top: 1px solid var(--border);
      background: rgba(7, 10, 10, 0.94);
    }

    @media (min-width: 880px) {
      .side {
        border-top: 0;
        border-left: 1px solid var(--border);
      }
    }

    .eyebrow {
      display: inline-flex;
      align-items: center;
      gap: 8px;
      border: 1px solid rgba(70, 201, 184, 0.18);
      background: var(--primary-soft);
      color: var(--primary);
      border-radius: 999px;
      padding: 8px 12px;
      font-size: 13px;
      font-weight: 600;
      margin-bottom: 18px;
    }

    h1 {
      margin: 0;
      font-size: clamp(2rem, 4vw, 3.5rem);
      line-height: 0.98;
      letter-spacing: -0.04em;
    }

    .body {
      margin-top: 18px;
      font-size: 1.03rem;
      line-height: 1.7;
      color: var(--text-muted);
      max-width: 42rem;
    }

    .detail {
      margin-top: 16px;
      color: var(--text-soft);
      font-size: 0.95rem;
      line-height: 1.6;
    }

    .actions {
      margin-top: 24px;
      display: flex;
      flex-wrap: wrap;
      gap: 12px;
    }

    .button {
      appearance: none;
      border: 0;
      border-radius: 16px;
      padding: 14px 18px;
      font-size: 0.96rem;
      font-weight: 700;
      cursor: pointer;
      text-decoration: none;
      display: inline-flex;
      align-items: center;
      justify-content: center;
      transition: transform 0.18s ease, opacity 0.18s ease, background 0.18s ease;
    }

    .button:hover { transform: translateY(-1px); }

    .button-primary {
      color: #081312;
      background: linear-gradient(135deg, var(--primary-strong), var(--primary));
      box-shadow: 0 18px 36px rgba(70, 201, 184, 0.18);
    }

    .button-secondary {
      color: var(--text);
      background: rgba(255, 255, 255, 0.04);
      border: 1px solid var(--border);
    }

    .meta-card {
      border: 1px solid var(--border);
      background: rgba(255, 255, 255, 0.025);
      border-radius: 24px;
      padding: 22px 20px;
    }

    .meta-label {
      color: var(--text-soft);
      font-size: 0.74rem;
      text-transform: uppercase;
      letter-spacing: 0.16em;
      margin-bottom: 8px;
    }

    .meta-title {
      margin: 0;
      font-size: 1.15rem;
      line-height: 1.35;
      color: var(--text);
    }

    .meta-description {
      margin-top: 10px;
      color: var(--text-muted);
      line-height: 1.65;
      font-size: 0.95rem;
    }

    .meta-list {
      display: grid;
      gap: 14px;
      margin-top: 18px;
    }

    .meta-item {
      padding-top: 14px;
      border-top: 1px solid rgba(255, 255, 255, 0.06);
    }

    .meta-item:first-child {
      border-top: 0;
      padding-top: 0;
    }

    .meta-item strong {
      display: block;
      color: var(--text);
      font-size: 0.95rem;
      margin-bottom: 4px;
    }

    .meta-item span {
      color: var(--text-muted);
      font-size: 0.92rem;
      line-height: 1.55;
      word-break: break-word;
    }

    form {
      margin-top: 24px;
      display: grid;
      gap: 14px;
    }

    label {
      display: block;
      font-size: 0.9rem;
      color: var(--text-muted);
      margin-bottom: 8px;
    }

    input[type="password"] {
      width: 100%;
      border-radius: 16px;
      border: 1px solid var(--border);
      background: rgba(255, 255, 255, 0.04);
      color: var(--text);
      padding: 15px 16px;
      font-size: 1rem;
      outline: none;
    }

    input[type="password"]:focus {
      border-color: rgba(70, 201, 184, 0.36);
      box-shadow: 0 0 0 4px rgba(70, 201, 184, 0.08);
    }

    .error {
      border: 1px solid rgba(255, 143, 130, 0.2);
      background: rgba(255, 143, 130, 0.08);
      color: var(--error);
      border-radius: 16px;
      padding: 13px 14px;
      font-size: 0.92rem;
      line-height: 1.55;
    }

    .helper {
      color: var(--text-soft);
      font-size: 0.88rem;
      line-height: 1.6;
    }
  </style>
</head>
<body>
  <main class="shell">
    <a class="brand" href="{{.SecondaryURL}}">
      <img class="brand-logo" src="{{.BrandImageURL}}" alt="patrn.ink" />
      <span>patrn.ink</span>
    </a>

    <section class="panel">
      <div class="panel-grid">
        <div class="main">
          <div class="eyebrow">{{.Eyebrow}}</div>
          <h1>{{.Heading}}</h1>
          <p class="body">{{.Body}}</p>
          {{if .Detail}}<p class="detail">{{.Detail}}</p>{{end}}

          {{if .ShowPasswordForm}}
            <form method="post" action="{{.ActionURL}}">
              {{if .PasswordError}}<div class="error">{{.PasswordError}}</div>{{end}}
              <div>
                <label for="password">Enter password</label>
                <input id="password" name="password" type="password" placeholder="Password" autocomplete="current-password" required />
              </div>
              <button class="button button-primary" type="submit">{{.ActionLabel}}</button>
            </form>
          {{end}}

          {{if .ShowAgeForm}}
            <form method="post" action="{{.ActionURL}}">
              {{if .AgeError}}<div class="error">{{.AgeError}}</div>{{end}}
              <input type="hidden" name="confirmed" value="true" />
              <input type="hidden" name="age_level" value="{{.AgeLevel}}" />
              <button class="button button-primary" type="submit">{{.ActionLabel}}</button>
            </form>
          {{end}}

          {{if and (not .ShowPasswordForm) (not .ShowAgeForm)}}
            <div class="actions">
              {{if .ActionURL}}<a class="button button-primary" href="{{.ActionURL}}">{{.ActionLabel}}</a>{{end}}
              {{if .SecondaryURL}}<a class="button button-secondary" href="{{.SecondaryURL}}">{{.SecondaryLabel}}</a>{{end}}
            </div>
          {{end}}
        </div>

        <aside class="side">
          <div class="meta-card">
            <div class="meta-label">Link snapshot</div>
            <p class="meta-title">{{.PreviewTitle}}</p>
            {{if .PreviewDescription}}<p class="meta-description">{{.PreviewDescription}}</p>{{end}}

            <div class="meta-list">
              <div class="meta-item">
                <strong>Short code</strong>
                <span>/{{.Code}}</span>
              </div>
              {{if .Domain}}
                <div class="meta-item">
                  <strong>Destination</strong>
                  <span>{{.Domain}}</span>
                </div>
              {{end}}
            </div>
          </div>
        </aside>
      </div>
    </section>
  </main>
</body>
</html>`))

func prefersHTML(c *gin.Context) bool {
	return strings.Contains(c.GetHeader("Accept"), "text/html")
}

func isHTMLFormPost(c *gin.Context) bool {
	contentType := c.ContentType()
	return c.Request.Method == http.MethodPost &&
		(strings.Contains(contentType, "application/x-www-form-urlencoded") ||
			strings.Contains(contentType, "multipart/form-data"))
}

func forwardedHeaderValue(value string) string {
	if value == "" {
		return ""
	}
	parts := strings.Split(value, ",")
	return strings.TrimSpace(parts[0])
}

func requestOrigin(c *gin.Context) string {
	proto := forwardedHeaderValue(c.GetHeader("X-Forwarded-Proto"))
	if proto == "" {
		if c.Request.TLS != nil {
			proto = "https"
		} else {
			proto = "http"
		}
	}

	host := forwardedHeaderValue(c.GetHeader("X-Forwarded-Host"))
	if host == "" {
		host = c.Request.Host
	}
	if host == "" {
		return config.AppConfig.BaseURL
	}

	return proto + "://" + host
}

func renderPublicPage(c *gin.Context, status int, data publicPageData) {
	if data.SecondaryURL == "" {
		data.SecondaryURL = config.AppConfig.FrontendURL
	}
	if data.SecondaryLabel == "" {
		data.SecondaryLabel = "Back to patrn.ink"
	}
	origin := requestOrigin(c)
	if data.BrandImageURL == "" {
		data.BrandImageURL = origin + "/brand/patrn.ink-transparent_512.png"
	}
	if data.FaviconURL == "" {
		data.FaviconURL = origin + "/brand/favicon.ico"
	}

	var buf bytes.Buffer
	if err := publicPageTemplate.Execute(&buf, data); err != nil {
		c.String(status, data.Heading+"\n\n"+data.Body)
		return
	}

	c.Data(status, "text/html; charset=utf-8", buf.Bytes())
}

func renderUnavailablePage(c *gin.Context, status int, code, heading, body, detail string, link *models.Link) {
	renderPublicPage(c, status, buildPublicPageData(code, link, publicPageData{
		PageTitle:   heading + " · patrn.ink",
		Eyebrow:     "Link unavailable",
		Heading:     heading,
		Body:        body,
		Detail:      detail,
		ActionURL:   config.AppConfig.FrontendURL,
		ActionLabel: "Go to patrn.ink",
	}))
}

func renderPasswordGate(c *gin.Context, code string, link *models.Link, errorMessage string, status int) {
	publicBaseURL := requestOrigin(c)
	renderPublicPage(c, status, buildPublicPageData(code, link, publicPageData{
		PageTitle:        "Password required · patrn.ink",
		Eyebrow:          "Protected link",
		Heading:          "This link is password protected",
		Body:             "Enter the password to continue to the destination. This extra step helps keep access controlled without changing the shareable link.",
		Detail:           "If someone shared this link with you, ask them for the password.",
		ActionURL:        publicBaseURL + "/" + code + "/verify",
		ActionLabel:      "Unlock and continue",
		ShowPasswordForm: true,
		PasswordError:    errorMessage,
	}))
}

func renderAgeGate(c *gin.Context, code string, link *models.Link, ageLabel, errorMessage string, status int) {
	publicBaseURL := requestOrigin(c)
	renderPublicPage(c, status, buildPublicPageData(code, link, publicPageData{
		PageTitle:      "Age confirmation required · patrn.ink",
		Eyebrow:        "Age-gated link",
		Heading:        "Age confirmation required",
		Body:           "This destination is restricted content. Confirm that you are " + ageLabel + " or older before continuing.",
		Detail:         "Only continue if you meet the stated age requirement for this destination.",
		ActionURL:      publicBaseURL + "/" + code + "/verify-age",
		ActionLabel:    "I confirm I am " + ageLabel + " or older",
		ShowAgeForm:    true,
		AgeLabel:       ageLabel,
		AgeLevel:       int(link.AgeVerification),
		AgeError:       errorMessage,
		SecondaryURL:   config.AppConfig.FrontendURL,
		SecondaryLabel: "Back to patrn.ink",
	}))
}

func ageLabelForLevel(level models.AgeVerification) string {
	ageLabels := map[models.AgeVerification]string{
		models.AgeVerification13Plus: "13+",
		models.AgeVerification18Plus: "18+",
		models.AgeVerification21Plus: "21+",
	}
	return ageLabels[level]
}

func ageProofCookieName(code string) string {
	var builder strings.Builder
	builder.WriteString("patrn_age_gate_")
	for _, r := range code {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
		} else {
			builder.WriteByte('_')
		}
	}
	return builder.String()
}

func issueAgeProof(c *gin.Context, code string, level models.AgeVerification) {
	expiresAt := time.Now().Add(ageProofTTL).Unix()
	payload := fmt.Sprintf("%s|%d|%d", code, int(level), expiresAt)
	signature := signPublicPayload(payload)
	value := base64.RawURLEncoding.EncodeToString([]byte(payload + "|" + signature))

	http.SetCookie(c.Writer, &http.Cookie{
		Name:     ageProofCookieName(code),
		Value:    value,
		Path:     "/",
		MaxAge:   int(ageProofTTL.Seconds()),
		HttpOnly: true,
		Secure:   config.AppConfig.Environment == "production",
		SameSite: http.SameSiteLaxMode,
	})
}

func hasValidAgeProof(c *gin.Context, code string, requiredLevel models.AgeVerification) bool {
	if requiredLevel == models.AgeVerificationNone {
		return true
	}

	value, err := c.Cookie(ageProofCookieName(code))
	if err != nil || value == "" {
		return false
	}

	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return false
	}

	parts := strings.Split(string(decoded), "|")
	if len(parts) != 4 {
		return false
	}

	if parts[0] != code {
		return false
	}

	level, err := strconv.Atoi(parts[1])
	if err != nil || level < int(requiredLevel) {
		return false
	}

	expiresAt, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil || time.Now().Unix() > expiresAt {
		return false
	}

	payload := strings.Join(parts[:3], "|")
	expectedSignature := signPublicPayload(payload)
	return hmac.Equal([]byte(parts[3]), []byte(expectedSignature))
}

func signPublicPayload(payload string) string {
	mac := hmac.New(sha256.New, []byte(config.AppConfig.JWTSecret))
	_, _ = mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func buildPublicPageData(code string, link *models.Link, data publicPageData) publicPageData {
	data.Code = code
	if link == nil {
		if data.PreviewTitle == "" {
			data.PreviewTitle = "This short link is unavailable"
		}
		return data
	}

	domain := extractPublicPageDomain(link.LongURL)
	data.Domain = domain

	if data.PreviewTitle == "" {
		if link.Title != "" {
			data.PreviewTitle = link.Title
		} else if domain != "" {
			data.PreviewTitle = domain
		} else {
			data.PreviewTitle = "Protected destination"
		}
	}

	if data.PreviewDescription == "" {
		if link.Description != "" {
			data.PreviewDescription = link.Description
		} else if link.ExpiresAt != nil {
			data.PreviewDescription = "Link access changes over time and may be limited by expiration or other rules."
		}
	}

	return data
}

func extractPublicPageDomain(rawURL string) string {
	parsed, err := neturl.Parse(rawURL)
	if err != nil {
		return ""
	}
	return parsed.Host
}

func formatPublicTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Local().Format("January 2, 2006 at 3:04 PM MST")
}
