package mirakurun

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientMetadataAndStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/services":
			io.WriteString(w, `[{"id":3273601024,"serviceId":1024,"networkId":32736,"name":"Test TV"}]`)
		case "/api/programs":
			io.WriteString(w, `[{"id":1,"serviceId":1024,"networkId":32736,"startAt":1000,"duration":60000,"name":"News"}]`)
		case "/api/tuners":
			io.WriteString(w, `[{"index":0,"name":"T0","isAvailable":true,"isFree":true}]`)
		case "/api/services/3273601024/stream":
			if r.URL.Query().Get("decode") != "1" || r.URL.Query().Get("priority") != "10" {
				t.Errorf("unexpected stream query: %s", r.URL.RawQuery)
			}
			w.Header().Set("Content-Type", "video/MP2T")
			io.WriteString(w, "transport-stream")
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := NewClient(server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	services, err := client.Services(context.Background())
	if err != nil || len(services) != 1 || services[0].Name != "Test TV" {
		t.Fatalf("Services() = %#v, %v", services, err)
	}
	programs, err := client.Programs(context.Background())
	if err != nil || len(programs) != 1 || programs[0].Name != "News" {
		t.Fatalf("Programs() = %#v, %v", programs, err)
	}
	tuners, err := client.Tuners(context.Background())
	if err != nil || len(tuners) != 1 || !tuners[0].IsFree {
		t.Fatalf("Tuners() = %#v, %v", tuners, err)
	}
	stream, err := client.OpenServiceStream(context.Background(), 3273601024, 10)
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	data, _ := io.ReadAll(stream)
	if string(data) != "transport-stream" {
		t.Fatalf("stream = %q", data)
	}
}

func TestClientIncludesResponseError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "backend unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	client, _ := NewClient(server.URL, server.Client())
	if _, err := client.Services(context.Background()); err == nil {
		t.Fatal("Services() succeeded for a failing backend")
	}
}
