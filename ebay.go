package ebay

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/pkg/errors"
)

// eBay URLs.
const (
	BaseURL        = "https://api.ebay.com/"
	SandboxBaseURL = "https://api.sandbox.ebay.com/"
)

// ScopeRoot scope definition.
// eBay API docs: https://developer.ebay.com/api-docs/static/oauth-scopes.html
const (
	ScopeRoot            = "https://api.ebay.com/oauth/api_scope"
)

// BuyAPI regroups the eBay Buy APIs.
//
// eBay API docs: https://developer.ebay.com/api-docs/buy/static/buy-landing.html
type BuyAPI struct {
	Browse *BrowseService
}

// Client manages communication with the eBay API.
type Client struct {
	client  *http.Client // Used to make actual API requests.
	baseURL *url.URL     // Base URL for API requests.

	// eBay APIs.
	Buy BuyAPI
}

// NewClient returns a new eBay API client.
// If a nil httpClient is provided, http.DefaultClient will be used.
func NewClient(httpclient *http.Client) *Client {
	return newClient(httpclient, BaseURL)
}

// NewSandboxClient returns a new eBay sandbox API client.
// If a nil httpClient is provided, http.DefaultClient will be used.
func NewSandboxClient(httpclient *http.Client) *Client {
	return newClient(httpclient, SandboxBaseURL)
}

// NewCustomClient returns a new custom eBay API client.
// BaseURL should have a trailing slash.
// If a nil httpClient is provided, http.DefaultClient will be used.
func NewCustomClient(httpclient *http.Client, baseURL string) (*Client, error) {
	if !strings.HasSuffix(baseURL, "/") {
		return nil, fmt.Errorf("BaseURL %s must have a trailing slash", baseURL)
	}
	return newClient(httpclient, baseURL), nil
}

func newClient(httpclient *http.Client, baseURL string) *Client {
	if httpclient == nil {
		httpclient = http.DefaultClient
	}
	url, _ := url.Parse(baseURL)
	c := &Client{client: httpclient, baseURL: url}
	c.Buy = BuyAPI{
		Browse: (*BrowseService)(&service{c}),
	}
	return c
}

type service struct {
	client *Client
}

// Opt describes functional options for the eBay API.
type Opt func(*http.Request)

// NewRequest creates an API request.
// url should always be specified without a preceding slash.
func (c *Client) NewRequest(method, url string, body interface{}, opts ...Opt) (*http.Request, error) {
	if strings.HasPrefix(url, "/") {
		return nil, errors.New("url should always be specified without a preceding slash")
	}
	u, err := c.baseURL.Parse(url)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	var buf io.ReadWriter
	if body != nil {
		buf = new(bytes.Buffer)
		enc := json.NewEncoder(buf)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(body); err != nil {
			return nil, err
		}
	}

	req, err := http.NewRequest(method, u.String(), buf)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	for _, opt := range opts {
		opt(req)
	}
	return req, nil
}

// Do sends an API request and stores the JSON decoded value into v.
func (c *Client) Do(ctx context.Context, req *http.Request, v interface{}) error {
	dump, _ := httputil.DumpRequest(req, true)
	resp, err := c.client.Do(req.WithContext(ctx))
	if err != nil {
		return errors.WithStack(err)
	}
	defer resp.Body.Close()
	if err := CheckResponse(req, resp, string(dump)); err != nil {
		return err
	}
	if v == nil {
		return nil
	}
	return errors.WithStack(json.NewDecoder(resp.Body).Decode(v))
}

// Error describes one error caused by an eBay API request.
//
// eBay API docs: https://developer.ebay.com/api-docs/static/handling-error-messages.html
type Error struct {
	ErrorID     int      `json:"errorId,omitempty"`
	Domain      string   `json:"domain,omitempty"`
	SubDomain   string   `json:"subDomain,omitempty"`
	Category    string   `json:"category,omitempty"`
	Message     string   `json:"message,omitempty"`
	LongMessage string   `json:"longMessage,omitempty"`
	InputRefIds []string `json:"inputRefIds,omitempty"`
	OuputRefIds []string `json:"outputRefIds,omitempty"`
	Parameters  []struct {
		Name  string `json:"name,omitempty"`
		Value string `json:"value,omitempty"`
	} `json:"parameters,omitempty"`
}

// ErrorData describes one or more errors caused by an eBay API request.
//
// eBay API docs: https://developer.ebay.com/api-docs/static/handling-error-messages.html
type ErrorData struct {
	Errors []Error `json:"errors,omitempty"`

	response    *http.Response
	requestDump string
}

func (e *ErrorData) Error() string {
	return fmt.Sprintf("%d\n%s\n%+v", e.response.StatusCode, e.requestDump, e.Errors)
}

// CheckResponse checks the API response for errors, and returns them if present.
func CheckResponse(req *http.Request, resp *http.Response, dump string) error {
	if s := resp.StatusCode; 200 <= s && s < 300 {
		return nil
	}
	errorData := &ErrorData{response: resp, requestDump: dump}
	_ = json.NewDecoder(resp.Body).Decode(errorData)
	return errorData
}

// IsError allows to check if err contains specific error codes returned by the eBay API.
//
// eBay API docs: https://developer.ebay.com/devzone/xml/docs/Reference/ebay/Errors/errormessages.htm
func IsError(err error, codes ...int) bool {
	if err == nil {
		return false
	}
	errData, ok := err.(*ErrorData)
	if !ok {
		return false
	}
	for _, e := range errData.Errors {
		for _, code := range codes {
			if e.ErrorID == code {
				return true
			}
		}
	}
	return false
}
