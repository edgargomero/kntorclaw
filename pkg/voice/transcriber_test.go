package voice

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewGroqTranscriber(t *testing.T) {
	tr := NewGroqTranscriber("test-key")
	if tr == nil {
		t.Fatal("NewGroqTranscriber returned nil")
	}
	if tr.apiKey != "test-key" {
		t.Errorf("apiKey = %q, want %q", tr.apiKey, "test-key")
	}
	if tr.apiBase != "https://api.groq.com/openai/v1" {
		t.Errorf("apiBase = %q, want default Groq URL", tr.apiBase)
	}
	if tr.httpClient == nil {
		t.Error("httpClient should not be nil")
	}
}

func TestIsAvailable(t *testing.T) {
	tests := []struct {
		key  string
		want bool
	}{
		{"valid-key", true},
		{"", false},
	}

	for _, tt := range tests {
		tr := NewGroqTranscriber(tt.key)
		if got := tr.IsAvailable(); got != tt.want {
			t.Errorf("IsAvailable() with key=%q = %v, want %v", tt.key, got, tt.want)
		}
	}
}

func TestTranscribe_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/audio/transcriptions") {
			t.Errorf("path = %q, want suffix /audio/transcriptions", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-api-key" {
			t.Errorf("Authorization = %q, want 'Bearer test-api-key'", r.Header.Get("Authorization"))
		}
		if !strings.Contains(r.Header.Get("Content-Type"), "multipart/form-data") {
			t.Errorf("Content-Type = %q, should contain multipart/form-data", r.Header.Get("Content-Type"))
		}

		// Verify multipart form fields
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			t.Fatalf("ParseMultipartForm error: %v", err)
		}
		if r.FormValue("model") != "whisper-large-v3" {
			t.Errorf("model field = %q, want 'whisper-large-v3'", r.FormValue("model"))
		}
		if r.FormValue("response_format") != "json" {
			t.Errorf("response_format = %q, want 'json'", r.FormValue("response_format"))
		}

		// Verify file upload
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("FormFile error: %v", err)
		}
		defer file.Close()
		if header.Filename != "test_audio.wav" {
			t.Errorf("filename = %q, want 'test_audio.wav'", header.Filename)
		}
		content, _ := io.ReadAll(file)
		if string(content) != "fake audio data" {
			t.Errorf("file content = %q, want 'fake audio data'", string(content))
		}

		// Send response
		resp := TranscriptionResponse{
			Text:     "Hello, this is a test transcription.",
			Language: "en",
			Duration: 3.14,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Create a temp audio file
	tmpDir := t.TempDir()
	audioPath := filepath.Join(tmpDir, "test_audio.wav")
	if err := os.WriteFile(audioPath, []byte("fake audio data"), 0644); err != nil {
		t.Fatalf("failed to create test audio file: %v", err)
	}

	tr := NewGroqTranscriber("test-api-key")
	tr.apiBase = server.URL

	result, err := tr.Transcribe(context.Background(), audioPath)
	if err != nil {
		t.Fatalf("Transcribe() error: %v", err)
	}

	if result.Text != "Hello, this is a test transcription." {
		t.Errorf("Text = %q, want %q", result.Text, "Hello, this is a test transcription.")
	}
	if result.Language != "en" {
		t.Errorf("Language = %q, want %q", result.Language, "en")
	}
	if result.Duration != 3.14 {
		t.Errorf("Duration = %f, want 3.14", result.Duration)
	}
}

func TestTranscribe_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "invalid api key"}`))
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	audioPath := filepath.Join(tmpDir, "test.wav")
	os.WriteFile(audioPath, []byte("data"), 0644)

	tr := NewGroqTranscriber("bad-key")
	tr.apiBase = server.URL

	_, err := tr.Transcribe(context.Background(), audioPath)
	if err == nil {
		t.Fatal("Transcribe() should return error on API error")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error should contain status code 401, got: %v", err)
	}
}

func TestTranscribe_FileNotFound(t *testing.T) {
	tr := NewGroqTranscriber("key")
	_, err := tr.Transcribe(context.Background(), "/nonexistent/audio.wav")
	if err == nil {
		t.Fatal("Transcribe() should return error for missing file")
	}
	if !strings.Contains(err.Error(), "open") {
		t.Errorf("error should mention file open failure, got: %v", err)
	}
}

func TestTranscribe_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not valid json"))
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	audioPath := filepath.Join(tmpDir, "test.wav")
	os.WriteFile(audioPath, []byte("data"), 0644)

	tr := NewGroqTranscriber("key")
	tr.apiBase = server.URL

	_, err := tr.Transcribe(context.Background(), audioPath)
	if err == nil {
		t.Fatal("Transcribe() should return error for invalid JSON response")
	}
	if !strings.Contains(err.Error(), "unmarshal") {
		t.Errorf("error should mention unmarshal, got: %v", err)
	}
}

func TestTranscribe_CancelledContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Slow response - should be cancelled
		select {}
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	audioPath := filepath.Join(tmpDir, "test.wav")
	os.WriteFile(audioPath, []byte("data"), 0644)

	tr := NewGroqTranscriber("key")
	tr.apiBase = server.URL

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := tr.Transcribe(ctx, audioPath)
	if err == nil {
		t.Fatal("Transcribe() should return error when context is cancelled")
	}
}
