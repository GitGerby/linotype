package ocr

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
	var receivedReq ocrRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&receivedReq); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}
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
	text, err := client.Recognize(context.Background(), []byte("png"), "test-prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if text != "Hello World" {
		t.Errorf("expected 'Hello World', got '%s'", text)
	}

	if len(receivedReq.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(receivedReq.Messages))
	}
	if receivedReq.Messages[0].Role != "user" {
		t.Errorf("expected role 'user', got %q", receivedReq.Messages[0].Role)
	}
	contentList, ok := receivedReq.Messages[0].Content.([]interface{})
	if !ok {
		t.Fatalf("expected []interface{} content, got %T", receivedReq.Messages[0].Content)
	}
	if len(contentList) != 2 {
		t.Fatalf("expected 2 content items, got %d", len(contentList))
	}
	textItem, ok := contentList[0].(map[string]interface{})
	if !ok || textItem["type"] != "text" || textItem["text"] != "test-prompt" {
		t.Errorf("unexpected text content: %+v", contentList[0])
	}
	imgItem, ok := contentList[1].(map[string]interface{})
	if !ok || imgItem["type"] != "image_url" {
		t.Errorf("unexpected image content: %+v", contentList[1])
	}
	if receivedReq.Temperature != 1.0 {
		t.Errorf("expected default Temperature 1.0, got %f", receivedReq.Temperature)
	}
}

func TestClient_Recognize_EmptyPrompt(t *testing.T) {
	var receivedReq ocrRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&receivedReq); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}
		resp := ocrResponse{}
		resp.Choices = append(resp.Choices, struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		}{
			Message: struct {
				Content string `json:"content"`
			}{
				Content: "Text",
			},
		})
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test", "model")
	_, err := client.Recognize(context.Background(), []byte("png"), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(receivedReq.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(receivedReq.Messages))
	}
	if receivedReq.Messages[0].Role != "user" {
		t.Errorf("expected role 'user', got %q", receivedReq.Messages[0].Role)
	}
	contentList, ok := receivedReq.Messages[0].Content.([]interface{})
	if !ok {
		t.Fatalf("expected []interface{} content, got %T", receivedReq.Messages[0].Content)
	}
	if len(contentList) != 1 {
		t.Fatalf("expected 1 content item when prompt is empty, got %d", len(contentList))
	}
	imgItem, ok := contentList[0].(map[string]interface{})
	if !ok || imgItem["type"] != "image_url" {
		t.Errorf("unexpected image content: %+v", contentList[0])
	}
	if receivedReq.Temperature != 1.0 {
		t.Errorf("expected default Temperature 1.0, got %f", receivedReq.Temperature)
	}
}

func TestClient_Recognize_CustomTemperature(t *testing.T) {
	var receivedReq ocrRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&receivedReq); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}
		resp := ocrResponse{}
		resp.Choices = append(resp.Choices, struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		}{
			Message: struct {
				Content string `json:"content"`
			}{
				Content: "Text",
			},
		})
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test", "model")
	client.Temperature = 0.7
	_, err := client.Recognize(context.Background(), []byte("png"), "test-prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedReq.Temperature != 0.7 {
		t.Errorf("expected Temperature 0.7, got %f", receivedReq.Temperature)
	}
}

func TestClient_Recognize_ZeroValueClientDefaultTemperature(t *testing.T) {
	var receivedReq ocrRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&receivedReq); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}
		resp := ocrResponse{}
		resp.Choices = append(resp.Choices, struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		}{
			Message: struct {
				Content string `json:"content"`
			}{
				Content: "Text",
			},
		})
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := &Client{
		BaseURL:    srv.URL,
		Model:      "model",
		HTTPClient: &http.Client{},
	}
	_, err := client.Recognize(context.Background(), []byte("png"), "test-prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedReq.Temperature != 1.0 {
		t.Errorf("expected defaulted Temperature 1.0, got %f", receivedReq.Temperature)
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

func TestClient_PNGExport_Combined(t *testing.T) {
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
				Content: "OCR Text",
			},
		})
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	exportDir := filepath.Join(t.TempDir(), "exported_pngs")

	client := NewClient(srv.URL, "test", "model")
	client.Debug = true
	client.PNGExport = exportDir

	img1 := []byte("fake-png-data-1")
	img2 := []byte("fake-png-data-2")

	_, err := client.Recognize(context.Background(), img1, "prompt")
	if err != nil {
		t.Fatalf("unexpected error on img1: %v", err)
	}
	_, err = client.Recognize(context.Background(), img2, "prompt")
	if err != nil {
		t.Fatalf("unexpected error on img2: %v", err)
	}

	if client.ExportCount() != 2 {
		t.Errorf("expected ExportCount 2, got %d", client.ExportCount())
	}

	// Verify file 1
	file1Path := filepath.Join(exportDir, "sub_00001.png")
	data1, err := os.ReadFile(file1Path)
	if err != nil {
		t.Fatalf("expected sub_00001.png to exist: %v", err)
	}
	if string(data1) != string(img1) {
		t.Errorf("expected file 1 content %q, got %q", img1, data1)
	}

	// Verify file 2
	file2Path := filepath.Join(exportDir, "sub_00002.png")
	data2, err := os.ReadFile(file2Path)
	if err != nil {
		t.Fatalf("expected sub_00002.png to exist: %v", err)
	}
	if string(data2) != string(img2) {
		t.Errorf("expected file 2 content %q, got %q", img2, data2)
	}

	// Reset count
	client.ResetExportCount()
	if client.ExportCount() != 0 {
		t.Errorf("expected ExportCount 0 after reset, got %d", client.ExportCount())
	}
}

func TestClient_PNGExport_NotCombined(t *testing.T) {
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
				Content: "OCR Text",
			},
		})
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	// 1. PNGExport set, but Debug = false
	exportDir1 := filepath.Join(t.TempDir(), "exported_pngs_nodebug")
	client1 := NewClient(srv.URL, "test", "model")
	client1.Debug = false
	client1.PNGExport = exportDir1

	_, err := client1.Recognize(context.Background(), []byte("png"), "prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client1.ExportCount() != 0 {
		t.Errorf("expected ExportCount 0 when Debug=false, got %d", client1.ExportCount())
	}
	if _, err := os.Stat(exportDir1); !os.IsNotExist(err) {
		t.Errorf("expected directory %s not to be created when Debug=false", exportDir1)
	}

	// 2. Debug = true, but PNGExport is empty
	client2 := NewClient(srv.URL, "test", "model")
	client2.Debug = true
	client2.PNGExport = ""

	_, err = client2.Recognize(context.Background(), []byte("png"), "prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client2.ExportCount() != 0 {
		t.Errorf("expected ExportCount 0 when PNGExport is empty, got %d", client2.ExportCount())
	}
}


