package invopop

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"gitlab.com/flimzy/testy"
	"resty.dev/v3"
)

func Test_put(t *testing.T) {
	tests := []struct {
		name      string
		responder testy.HTTPResponder
		path      string
		body      interface{}
		err       string
	}{
		{
			name: "unmarshalable body",
			body: json.RawMessage("this is not JSON"),
			err:  `json: error calling MarshalJSON for type json.RawMessage: invalid character 'h' in literal true (expecting 'r')`,
		},
		{
			name: "network error",
			responder: func(*http.Request) (*http.Response, error) {
				return nil, errors.New("network error")
			},
			body: map[string]string{"foo": "bar"},
			err:  `Put "": network error`,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := &Client{
				conn: resty.NewWithClient(testy.HTTPClient(tt.responder)),
			}
			err := c.put(context.Background(), tt.path, tt.body, tt.body)
			assert.EqualError(t, err, tt.err)
		})
	}
}

func TestHTTPClient(t *testing.T) {
	hc := new(http.Client)
	c := &Client{
		conn: resty.NewWithClient(hc),
	}
	assert.Equal(t, hc, c.HTTPClient())
}

func Test_getRaw_HTTPError(t *testing.T) {
	tests := map[string]struct {
		statusCode  int
		body        string
		contentType string
		err         string
	}{
		"json error body": {
			statusCode: http.StatusBadGateway,
			body:       `{"code":"bad_gateway","message":"upstream failure"}`,
			err:        "502: (bad_gateway) upstream failure",
		},
		"plain text body": {
			statusCode:  http.StatusGatewayTimeout,
			body:        "error code: 504",
			contentType: "text/plain; charset=UTF-8",
			err:         "504: error code: 504",
		},
		"empty body": {
			statusCode:  http.StatusGatewayTimeout,
			contentType: "text/plain; charset=UTF-8",
			err:         "504: Gateway Timeout",
		},
		"oversized body": {
			statusCode:  http.StatusGatewayTimeout,
			body:        strings.Repeat("x", 600),
			contentType: "text/plain; charset=UTF-8",
			err:         "504: " + strings.Repeat("x", 512) + "...",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			responder := func(*http.Request) (*http.Response, error) {
				header := make(http.Header)
				if tt.contentType != "" {
					header.Set("Content-Type", tt.contentType)
				}
				return &http.Response{
					StatusCode: tt.statusCode,
					Status:     http.StatusText(tt.statusCode),
					Header:     header,
					Body:       io.NopCloser(strings.NewReader(tt.body)),
				}, nil
			}

			c := &Client{
				conn: resty.NewWithClient(testy.HTTPClient(responder)),
			}

			body, err := c.getRaw(context.Background(), "/test")
			assert.Nil(t, body)
			assert.EqualError(t, err, tt.err)
		})
	}
}
