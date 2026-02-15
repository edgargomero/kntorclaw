package utils

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsAudioFile_ByExtension(t *testing.T) {
	tests := []struct {
		filename    string
		contentType string
		want        bool
	}{
		{"voice.mp3", "", true},
		{"recording.wav", "", true},
		{"audio.ogg", "", true},
		{"song.m4a", "", true},
		{"music.flac", "", true},
		{"track.aac", "", true},
		{"old.wma", "", true},
		{"VOICE.MP3", "", true}, // case insensitive
		{"Recording.WAV", "", true},

		// Non-audio files
		{"photo.jpg", "", false},
		{"document.pdf", "", false},
		{"video.mp4", "", false},
		{"data.json", "", false},
		{"", "", false},
	}

	for _, tt := range tests {
		got := IsAudioFile(tt.filename, tt.contentType)
		if got != tt.want {
			t.Errorf("IsAudioFile(%q, %q) = %v, want %v", tt.filename, tt.contentType, got, tt.want)
		}
	}
}

func TestIsAudioFile_ByContentType(t *testing.T) {
	tests := []struct {
		filename    string
		contentType string
		want        bool
	}{
		{"file", "audio/mpeg", true},
		{"file", "audio/wav", true},
		{"file", "audio/ogg", true},
		{"file", "application/ogg", true},
		{"file", "application/x-ogg", true},
		{"file", "Audio/MPEG", true}, // case insensitive

		{"file", "video/mp4", false},
		{"file", "image/jpeg", false},
		{"file", "text/plain", false},
	}

	for _, tt := range tests {
		got := IsAudioFile(tt.filename, tt.contentType)
		if got != tt.want {
			t.Errorf("IsAudioFile(%q, %q) = %v, want %v", tt.filename, tt.contentType, got, tt.want)
		}
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"normal.txt", "normal.txt"},
		{"../../../etc/passwd", "passwd"},
		{"path/to/file.txt", "file.txt"},
		{"path\\to\\file.txt", "file.txt"},
		{"..\\..\\secret.key", "secret.key"},
		{"file with spaces.txt", "file with spaces.txt"},
		{"audio_2024.ogg", "audio_2024.ogg"},
	}

	for _, tt := range tests {
		got := SanitizeFilename(tt.input)
		if got != tt.want {
			t.Errorf("SanitizeFilename(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSanitizeFilename_NoDirectoryTraversal(t *testing.T) {
	dangerous := []string{
		"../../../etc/passwd",
		"..\\..\\Windows\\system32\\config",
		"foo/../../../bar",
	}
	for _, input := range dangerous {
		result := SanitizeFilename(input)
		if strings.Contains(result, "..") {
			t.Errorf("SanitizeFilename(%q) = %q, still contains '..'", input, result)
		}
	}
}

func TestDownloadFile(t *testing.T) {
	payload := []byte(`{"status":"ok"}`)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Custom") != "test-value" {
			t.Errorf("missing custom header")
		}
		w.WriteHeader(http.StatusOK)
		w.Write(payload)
	}))
	defer server.Close()

	path := DownloadFile(server.URL+"/test.json", "test.json", DownloadOptions{
		ExtraHeaders: map[string]string{"X-Custom": "test-value"},
		LoggerPrefix: "test",
	})

	if path == "" {
		t.Fatal("DownloadFile returned empty path")
	}
	defer os.Remove(path)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read downloaded file: %v", err)
	}

	var result map[string]string
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to parse downloaded content: %v", err)
	}
	if result["status"] != "ok" {
		t.Errorf("downloaded content status = %q, want %q", result["status"], "ok")
	}
}

func TestDownloadFile_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	path := DownloadFile(server.URL+"/fail", "fail.txt", DownloadOptions{
		LoggerPrefix: "test",
	})
	if path != "" {
		os.Remove(path)
		t.Error("DownloadFile should return empty string on server error")
	}
}

func TestDownloadFile_InvalidURL(t *testing.T) {
	path := DownloadFile("http://invalid.test.localhost:99999/file", "file.txt", DownloadOptions{
		LoggerPrefix: "test",
	})
	if path != "" {
		os.Remove(path)
		t.Error("DownloadFile should return empty string for unreachable URL")
	}
}

func TestDownloadFileSimple(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("content"))
	}))
	defer server.Close()

	path := DownloadFileSimple(server.URL+"/file.txt", "file.txt")
	if path == "" {
		t.Fatal("DownloadFileSimple returned empty path")
	}
	defer os.Remove(path)

	// Verify the file name contains the sanitized name
	if !strings.HasSuffix(path, "_file.txt") {
		t.Errorf("downloaded file path %q doesn't end with _file.txt", filepath.Base(path))
	}
}
