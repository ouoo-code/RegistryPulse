package registry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// BearerChallenge is the authentication challenge advertised by a registry.
type BearerChallenge struct {
	Realm   string
	Service string
	Scope   string
}

// ParseBearerChallenge parses a WWW-Authenticate header without assuming a
// particular parameter order. Quoted commas are handled correctly.
func ParseBearerChallenge(header string) (BearerChallenge, error) {
	header = strings.TrimSpace(header)
	if len(header) < len("Bearer") || !strings.EqualFold(header[:len("Bearer")], "Bearer") {
		return BearerChallenge{}, errors.New("bearer challenge not found")
	}
	rest := strings.TrimSpace(header[len("Bearer"):])
	if rest == "" {
		return BearerChallenge{}, errors.New("bearer challenge has no parameters")
	}
	challenge := BearerChallenge{}
	for _, part := range splitHeader(rest) {
		key, value, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		value = strings.Trim(strings.TrimSpace(value), "\"")
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "realm":
			challenge.Realm = value
		case "service":
			challenge.Service = value
		case "scope":
			challenge.Scope = value
		}
	}
	if challenge.Realm == "" {
		return BearerChallenge{}, errors.New("bearer challenge has no realm")
	}
	return challenge, nil
}

func splitHeader(header string) []string {
	var result []string
	start, quoted := 0, false
	for i, r := range header {
		switch r {
		case '"':
			quoted = !quoted
		case ',':
			if !quoted {
				result = append(result, strings.TrimSpace(header[start:i]))
				start = i + 1
			}
		}
	}
	result = append(result, strings.TrimSpace(header[start:]))
	return result
}

type Client struct {
	BaseURL     string
	HTTPClient  *http.Client
	UserAgent   string
	Credentials *Credentials
}

type Credentials struct {
	AuthType string
	Username string
	Secret   string
}

type Manifest struct {
	SchemaVersion int          `json:"schemaVersion"`
	MediaType     string       `json:"mediaType"`
	Config        Descriptor   `json:"config"`
	Layers        []Descriptor `json:"layers"`
}

type Descriptor struct {
	MediaType string `json:"mediaType"`
	Digest    string `json:"digest"`
	Size      int64  `json:"size"`
}

type Result struct {
	ManifestStatus      int
	ManifestMethod      string
	ManifestDigest      string
	ManifestContentType string
	ManifestSize        int64
	Manifest            Manifest
	BlobStatus          int
	BlobBytes           int64
}

func (c Client) client() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return http.DefaultClient
}

func (c Client) request(ctx context.Context, method, path string, headers http.Header, token string) (*http.Response, error) {
	base := strings.TrimRight(c.BaseURL, "/")
	req, err := http.NewRequestWithContext(ctx, method, base+path, nil)
	if err != nil {
		return nil, err
	}
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	for key, values := range headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	c.applyCredentials(req, token)
	return c.client().Do(req)
}

func (c Client) applyCredentials(req *http.Request, token string) {
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
		return
	}
	if c.Credentials == nil || c.Credentials.Secret == "" {
		return
	}
	switch strings.ToLower(c.Credentials.AuthType) {
	case "basic":
		req.SetBasicAuth(c.Credentials.Username, c.Credentials.Secret)
	case "bearer", "token":
		req.Header.Set("Authorization", "Bearer "+c.Credentials.Secret)
	}
}

func (c Client) token(ctx context.Context, challenge BearerChallenge) (string, error) {
	u, err := url.Parse(challenge.Realm)
	if err != nil {
		return "", err
	}
	query := u.Query()
	if challenge.Service != "" {
		query.Set("service", challenge.Service)
	}
	if challenge.Scope != "" {
		query.Set("scope", challenge.Scope)
	}
	u.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", err
	}
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	c.applyCredentials(req, "")
	resp, err := c.client().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("token endpoint returned %d", resp.StatusCode)
	}
	var body struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&body); err != nil {
		return "", err
	}
	if body.Token == "" {
		body.Token = body.AccessToken
	}
	if body.Token == "" {
		return "", errors.New("token response has no token")
	}
	return body.Token, nil
}

func (c Client) authenticated(ctx context.Context, method, path string, headers http.Header) (*http.Response, error) {
	resp, err := c.request(ctx, method, path, headers, "")
	if err != nil || resp.StatusCode != http.StatusUnauthorized {
		return resp, err
	}
	challenge, parseErr := ParseBearerChallenge(resp.Header.Get("WWW-Authenticate"))
	resp.Body.Close()
	if parseErr != nil {
		return nil, parseErr
	}
	token, err := c.token(ctx, challenge)
	if err != nil {
		return nil, err
	}
	return c.request(ctx, method, path, headers, token)
}

func (c Client) CheckV2(ctx context.Context) (int, error) {
	status, _, err := c.CheckV2Details(ctx)
	return status, err
}

// CheckV2Details returns the API status and distribution version. A valid
// 401 challenge is treated as a live Registry API.
func (c Client) CheckV2Details(ctx context.Context) (int, string, error) {
	resp, err := c.authenticated(ctx, http.MethodGet, "/v2/", nil)
	if err != nil {
		raw, rawErr := c.request(ctx, http.MethodGet, "/v2/", nil, "")
		if rawErr == nil {
			defer raw.Body.Close()
			if raw.StatusCode == http.StatusUnauthorized {
				return raw.StatusCode, raw.Header.Get("Docker-Distribution-API-Version"), nil
			}
		}
		return 0, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusUnauthorized {
		return resp.StatusCode, resp.Header.Get("Docker-Distribution-API-Version"), fmt.Errorf("registry returned %d", resp.StatusCode)
	}
	return resp.StatusCode, resp.Header.Get("Docker-Distribution-API-Version"), nil
}

// Manifest first uses HEAD, then falls back to GET when the registry does not
// implement HEAD or omits a useful digest. GET is limited to a small body.
func (c Client) Manifest(ctx context.Context, repository, reference string) (Result, error) {
	path := "/v2/" + url.PathEscape(repository) + "/manifests/" + url.PathEscape(reference)
	headers := http.Header{"Accept": []string{"application/vnd.oci.image.manifest.v1+json, application/vnd.docker.distribution.manifest.v2+json, application/vnd.docker.distribution.manifest.list.v2+json"}}
	resp, err := c.authenticated(ctx, http.MethodHead, path, headers)
	if err != nil {
		return Result{}, err
	}
	result := Result{ManifestStatus: resp.StatusCode, ManifestMethod: http.MethodHead}
	digest := resp.Header.Get("Docker-Content-Digest")
	contentType := resp.Header.Get("Content-Type")
	result.ManifestDigest, result.ManifestContentType = digest, contentType
	if rawSize := resp.Header.Get("Content-Length"); rawSize != "" {
		result.ManifestSize, _ = strconv.ParseInt(rawSize, 10, 64)
	}
	resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 && digest != "" {
		result.Manifest.MediaType = contentType
		return result, nil
	}
	resp, err = c.authenticated(ctx, http.MethodGet, path, headers)
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()
	result.ManifestStatus, result.ManifestMethod = resp.StatusCode, http.MethodGet
	result.ManifestContentType = resp.Header.Get("Content-Type")
	result.ManifestDigest = resp.Header.Get("Docker-Content-Digest")
	if rawSize := resp.Header.Get("Content-Length"); rawSize != "" {
		result.ManifestSize, _ = strconv.ParseInt(rawSize, 10, 64)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return result, fmt.Errorf("manifest returned %d", resp.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&result.Manifest); err != nil {
		return result, err
	}
	if result.ManifestDigest == "" {
		result.ManifestDigest = result.Manifest.Config.Digest
	}
	if result.ManifestSize == 0 {
		body, _ := json.Marshal(result.Manifest)
		result.ManifestSize = int64(len(body))
	}
	return result, nil
}

// Blob downloads only a bounded byte range. It never reads an entire layer.
func (c Client) BlobRange(ctx context.Context, repository, digest string, size int64) (int, int64, bool, error) {
	if size <= 0 {
		size = 1 << 20
	}
	if size > 4<<20 {
		size = 4 << 20
	}
	path := "/v2/" + url.PathEscape(repository) + "/blobs/" + url.PathEscape(digest)
	headers := http.Header{"Range": []string{"bytes=0-" + strconv.FormatInt(size-1, 10)}}
	resp, err := c.authenticated(ctx, http.MethodGet, path, headers)
	if err != nil {
		return 0, 0, false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		return resp.StatusCode, 0, false, fmt.Errorf("blob returned %d", resp.StatusCode)
	}
	read, err := io.CopyN(io.Discard, resp.Body, size)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return resp.StatusCode, read, resp.StatusCode == http.StatusPartialContent, err
	}
	return resp.StatusCode, read, resp.StatusCode == http.StatusPartialContent, nil
}
