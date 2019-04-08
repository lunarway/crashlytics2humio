package main

import (
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAuthenticate(t *testing.T) {
	tt := []struct {
		name        string
		serverToken string
		path        string
		status      int
	}{
		{
			name:        "missing token",
			serverToken: "token",
			path:        "/webhook",
			status:      http.StatusUnauthorized,
		},
		{
			name:        "empty token",
			serverToken: "token",
			path:        "/webhook?token=",
			status:      http.StatusUnauthorized,
		},
		{
			name:        "whitespace token",
			serverToken: "token",
			path:        "/webhook?token=%20%20",
			status:      http.StatusUnauthorized,
		},
		{
			name:        "wrong token",
			serverToken: "token",
			path:        "/webhook?token=wrong-token",
			status:      http.StatusUnauthorized,
		},
		{
			name:        "correct token",
			serverToken: "token",
			path:        "/webhook?token=token",
			status:      http.StatusOK,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			handler := func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			w := httptest.NewRecorder()
			authenticate(tc.serverToken, handler)(w, req)

			assert.Equal(t, tc.status, w.Result().StatusCode, "status code not as expected")
		})
	}
}

func TestWebhookHandler(t *testing.T) {
	tt := []struct {
		name   string
		method string
		body   io.Reader
		code   int
		pushes []Push
	}{
		{
			name:   "GET request",
			method: http.MethodGet,
			body:   nil,
			code:   http.StatusBadRequest,
			pushes: nil,
		},
		{
			name:   "POST request with empty payload",
			method: http.MethodPost,
			body:   strings.NewReader(``),
			code:   http.StatusBadRequest,
			pushes: nil,
		},
		{
			name:   "POST request with invalid payload",
			method: http.MethodPost,
			body:   strings.NewReader(`some payload`),
			code:   http.StatusBadRequest,
			pushes: nil,
		},
		{
			name:   "POST request with verification payload",
			method: http.MethodPost,
			body: strings.NewReader(`{
				"event": "verification",
				"payload_type": "none"
			}`),
			code:   http.StatusOK,
			pushes: nil,
		},
		{
			name:   "POST request with issue impact change payload",
			method: http.MethodPost,
			body: strings.NewReader(`{
				"event": "issue_impact_change",
				"payload_type": "issue",
				"payload": {
					"display_id": 123 ,
					"title": "Issue Title" ,
					"method": "methodName of issue",
					"impact_level": 2,
					"crashes_count": 54,
					"impacted_devices_count": 16,
					"url": "http://crashlytics.com/full/url/to/issue"
				}
			}`),
			code: http.StatusOK,
			pushes: []Push{
				{
					Type:      "issue",
					Timestamp: 0,
					Data: map[string]interface{}{
						"display_id":             float64(123),
						"title":                  "Issue Title",
						"method":                 "methodName of issue",
						"impact_level":           float64(2),
						"crashes_count":          float64(54),
						"impacted_devices_count": float64(16),
						"url":                    "http://crashlytics.com/full/url/to/issue",
					},
				},
			},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			req := httptest.NewRequest(tc.method, "/webhook", tc.body)
			pushRecorder := pushRecorder{}
			webhookHandler(func() time.Time {
				return time.Unix(0, 0)
			}, &pushRecorder)(recorder, req)
			assert.Equal(t, tc.code, recorder.Code, "status code not as expected")
			t.Logf("Expected\n%#+v", tc.pushes)
			t.Logf("Got\n%#+v", pushRecorder.Pushes)
			assert.ElementsMatch(t, tc.pushes, pushRecorder.Pushes, "pushes not as expected")
		})
	}
}

func TestWebhookHandler_pushFail(t *testing.T) {
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(`{
		"event": "issue_impact_change",
		"payload_type": "issue",
		"payload": {
			"title": "Issue Title"
		}
	}`))
	pusher := pushFailer{
		Err: errors.New("some unknown error"),
	}
	webhookHandler(func() time.Time {
		return time.Unix(0, 0)
	}, &pusher)(recorder, req)
	assert.Equal(t, http.StatusOK, recorder.Code, "status code not as expected")
}

var _ Pusher = &pushRecorder{}

type pushRecorder struct {
	Pushes []Push
}

func (p *pushRecorder) Push(push Push) error {
	p.Pushes = append(p.Pushes, push)
	return nil
}

var _ Pusher = &pushFailer{}

type pushFailer struct {
	Err error
}

func (p *pushFailer) Push(push Push) error {
	return p.Err
}

func TestHumioPusher_Push(t *testing.T) {
	doRecoder := doRecorder{}
	pusher := humioPusher{
		url: url.URL{
			Host:   "localhost:8080",
			Scheme: "http",
		},
		httpClient:  &doRecoder,
		ingestToken: "token",
	}

	err := pusher.Push(Push{
		Type:      "issue",
		Timestamp: 1,
		Data: map[string]interface{}{
			"title": "Issue Title",
		},
	})

	if !assert.NoError(t, err, "unexpected push error") {
		return
	}
	expectedPayload := `[{"events":[{"timestamp":1,"attributes":{"title":"Issue Title"}}]}]
`
	assert.Equal(t, expectedPayload, string(doRecoder.body), "request body not as expected")
	authorization := doRecoder.req.Header.Get("Authorization")
	assert.Equal(t, "Bearer token", authorization, "authorization header not as expected")
	contentType := doRecoder.req.Header.Get("Content-Type")
	assert.Equal(t, "application/json", contentType, "content-type header not as expected")
	assert.Equal(t, "http://localhost:8080/api/v1/ingest/humio-structured", doRecoder.req.URL.String(), "url not as expected")
}

type doRecorder struct {
	body []byte
	req  *http.Request
}

func (r *doRecorder) Do(req *http.Request) (*http.Response, error) {
	data, _ := ioutil.ReadAll(req.Body)
	r.body = data
	r.req = req
	return &http.Response{
		StatusCode: http.StatusOK,
	}, nil
}

func TestValidateURL(t *testing.T) {
	tt := []struct {
		name   string
		input  string
		output string
		err    error
	}{
		{
			name:   "http url",
			input:  "http://cloud.humio.com",
			output: "http://cloud.humio.com",
			err:    nil,
		},
		{
			name:   "https url",
			input:  "https://cloud.humio.com",
			output: "https://cloud.humio.com",
			err:    nil,
		},
		{
			name:   "trailing slash",
			input:  "https://cloud.humio.com/",
			output: "https://cloud.humio.com/",
			err:    nil,
		},
		{
			name:   "scheme missing",
			input:  "cloud.humio.com",
			output: "",
			err:    errors.New("schema required"),
		},
		{
			name:   "tcp schema",
			input:  "tcp://cloud.humio.com",
			output: "",
			err:    errors.New("only schemes http(s) are supported"),
		},
		{
			name:   "relatve path",
			input:  "http://cloud.humio.com/../asd",
			output: "http://cloud.humio.com/asd",
			err:    nil,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			output, err := validateURL(tc.input)
			if tc.err != nil {
				assert.EqualError(t, err, tc.err.Error(), "output error not as expected")
			} else {
				assert.NoError(t, err, "no output error expected")
			}
			assert.Equal(t, tc.output, output.String(), "output not as expected")
		})
	}
}
