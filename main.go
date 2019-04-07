package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"strings"
	"time"
)

func main() {
	timeout := flag.Duration("timeout", 10*time.Second, "server request timeouts")
	token := flag.String("crashlytics-auth-token", "", "crashlytics webhook authentication token (requried)")
	flag.Parse()
	if strings.TrimSpace(*token) == "" {
		log.Fatalf("flag crashlytics-auth-token is required")
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/webhook", authenticate(*token, webhookHandler(&humioPusher{})))
	server := http.Server{
		Addr:              ":8080",
		Handler:           mux,
		IdleTimeout:       *timeout,
		ReadTimeout:       *timeout,
		WriteTimeout:      *timeout,
		ReadHeaderTimeout: *timeout,
	}
	log.Printf("Listening on %s\n", server.Addr)
	err := server.ListenAndServe()
	if err != nil {
		log.Fatalf("http server failed: %v", err)
	}
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

func webhookHandler(pusher Pusher) http.HandlerFunc {
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
			log.Printf(`level=error message="webhook: unmarshal payload failed: %v"`, err)
			w.WriteHeader(http.StatusBadRequest)
		}
		if payload.PayloadType != "issue" {
			w.WriteHeader(http.StatusOK)
			return
		}
		err = pusher.Push(Push{
			Type: payload.PayloadType,
			Data: payload.Payload,
		})
		if err != nil {
			log.Printf(`level=error message="webhook: push '%s' type to humio failed: %v"`, payload.PayloadType, err)
			w.WriteHeader(http.StatusOK)
		}
		w.WriteHeader(http.StatusOK)
	}
}

type Pusher interface {
	Push(data Push) error
}

type Push struct {
	Type string
	Data map[string]interface{}
}

type humioPusher struct{}

func (*humioPusher) Push(push Push) error {
	return nil
}
