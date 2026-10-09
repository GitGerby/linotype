package ocr

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBuildPrompt(t *testing.T) {
	lang := "Japanese"
	defaultP := BuildPrompt(lang, "")
	if defaultP == "" {
		t.Fatalf("expected non-empty prompt")
	}
	customP := BuildPrompt(lang, "Translate from {language}.")
	if customP != "Translate from Japanese." {
		t.Errorf("custom prompt failed: %s", customP)
	}
}

func TestClient_Recognize_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ocrResponse{}
		resp.Choices = append(resp.Choices, struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		}{
			Message: struct {
				Content string `json:"content"`
			}{
				Content: "  Hello World  \n",
			},
		})
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test", "model")
	text, err := client.Recognize(context.Background(), []byte("png"), "prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if text != "Hello World" {
		t.Errorf("expected 'Hello World', got '%s'", text)
	}
}

func TestClient_Recognize_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("bad request"))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test", "model")
	_, err := client.Recognize(context.Background(), []byte("png"), "prompt")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestClient_Recognize_Empty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"choices":[]}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test", "model")
	_, err := client.Recognize(context.Background(), []byte("png"), "prompt")
	if err == nil {
		t.Fatal("expected error for empty choices")
	}
}

