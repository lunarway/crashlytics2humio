package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

func main() {
	timeout := flag.Duration("timeout", 10*time.Second, "server request timeouts")
	token := flag.String("crashlytics-auth-token", "", "crashlytics webhook authentication token (required)")
	humioIngestToken := flag.String("humio-ingest-token", "", "humio ingest token (required)")
	humioURL := flag.String("humio-url", "", "humio http api url eg. https://cloud.humio.com (required)")
	port := flag.String("port", "8080", "http server port for webhooks")
	flag.Parse()

	var missingRequiredFlags []string
	missingRequiredFlags = requiredFlag(missingRequiredFlags, "crashlytics-auth-token", *token)
	missingRequiredFlags = requiredFlag(missingRequiredFlags, "humio-ingest-token", *humioIngestToken)
	missingRequiredFlags = requiredFlag(missingRequiredFlags, "humio-url", *humioURL)
	if len(missingRequiredFlags) != 0 {
		fmt.Printf("flag(s) %v required but missing\n", strings.Join(missingRequiredFlags, " "))
		os.Exit(2)
	}
	url, err := validateURL(*humioURL)
	if err != nil {
		fmt.Printf("flag humio-url not valid: should be in the form 'http://cloud.humio.com': %v\n", err)
		os.Exit(2)
	}

	client := http.Client{
		Timeout: *timeout,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/webhook", authenticate(*token, webhookHandler(time.Now, &humioPusher{
		httpClient:  &client,
		ingestToken: *humioIngestToken,
		url:         url,
	})))
	server := http.Server{
		Addr:              fmt.Sprintf(":%s", *port),
		Handler:           mux,
		IdleTimeout:       *timeout,
		ReadTimeout:       *timeout,
		WriteTimeout:      *timeout,
		ReadHeaderTimeout: *timeout,
	}
	log.Printf("level=info message=\"Listening on %s\"", server.Addr)
	err = server.ListenAndServe()
	if err != nil {
		log.Printf("level=error message=\"http server failed: %v\"", err)
		os.Exit(1)
	}
}

func validateURL(rawURL string) (url.URL, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return url.URL{}, err
	}
	u = u.ResolveReference(u)
	if u.Scheme == "" {
		return url.URL{}, errors.New("schema required")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return url.URL{}, errors.New("only schemes http(s) are supported")
	}
	return *u, nil
}

func requiredFlag(missingRequiredFlags []string, flagName, value string) []string {
	if strings.TrimSpace(value) == "" {
		missingRequiredFlags = append(missingRequiredFlags, flagName)
	}
	return missingRequiredFlags
}

type crashlyticsWebhook struct {
	Event       string                 `json:"event,omitempty"`
	PayloadType string                 `json:"payload_type,omitempty"`
	Payload     map[string]interface{} `json:"payload,omitempty"`
}

// authenticate authenticates the handler against a known token in the "token"
// query param.
//
// If authentication fails a 401 Unauthorized HTTP status is returned.
func authenticate(token string, h http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := r.URL.Query()
		queryToken := params.Get("token")
		queryToken = strings.TrimSpace(queryToken)
		if queryToken != token {
			http.Error(w, "invalid authentication token", http.StatusUnauthorized)
			return
		}
		h(w, r)
	})
}

func webhookHandler(now func() time.Time, pusher Pusher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if r.ContentLength <= 0 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		decoder := json.NewDecoder(r.Body)
		var payload crashlyticsWebhook
		err := decoder.Decode(&payload)
		if err != nil {
			log.Printf("level=error message=\"webhook: unmarshal payload failed: %v\"", err)
			w.WriteHeader(http.StatusBadRequest)
		}
		if payload.PayloadType != "issue" {
			w.WriteHeader(http.StatusOK)
			return
		}
		err = pusher.Push(Push{
			Timestamp: now().UnixNano() / 1e6,
			Type:      payload.PayloadType,
			Data:      payload.Payload,
		})
		if err != nil {
			log.Printf("level=error message=\"webhook: push '%s' type to humio failed: %v\"", payload.PayloadType, err)
			w.WriteHeader(http.StatusOK)
		}
		w.WriteHeader(http.StatusOK)
	}
}

type Pusher interface {
	Push(data Push) error
}

type Push struct {
	Type      string
	Timestamp int64
	Data      map[string]interface{}
}

type humioPusher struct {
	url         url.URL
	ingestToken string
	httpClient  Doer
}

type Doer interface {
	Do(req *http.Request) (*http.Response, error)
}

type humioPayload struct {
	Tags   map[string]string `json:"tags,omitempty"`
	Events []humioEvent      `json:"events,omitempty"`
}

type humioEvent struct {
	Timestamp  int64                  `json:"timestamp,omitempty"`
	Timezone   int64                  `json:"timezone,omitempty"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
	Rawstring  string                 `json:"rawstring,omitempty"`
}

func (p *humioPusher) Push(push Push) error {
	var body bytes.Buffer
	encoder := json.NewEncoder(&body)
	err := encoder.Encode([]humioPayload{
		{
			Events: []humioEvent{
				{
					Timestamp:  push.Timestamp,
					Attributes: push.Data,
				},
			},
		},
	})
	if err != nil {
		return err
	}
	u := p.url.ResolveReference(&url.URL{
		Path: "/api/v1/ingest/humio-structured",
	})
	req, err := http.NewRequest(http.MethodPost, u.String(), &body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.ingestToken))
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		log.Printf("level=error message=\"humio response body: %s\"", body)
		return fmt.Errorf("humio status code not ok: %s", resp.Status)
	}
	return nil
}
