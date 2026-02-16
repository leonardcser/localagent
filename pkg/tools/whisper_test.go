package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestTranscribeAudio(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}

		err := r.ParseMultipartForm(10 << 20)
		if err != nil {
			t.Fatalf("failed to parse multipart form: %v", err)
		}

		if r.FormValue("response_format") != "json" {
			t.Errorf("expected response_format=json, got %s", r.FormValue("response_format"))
		}

		_, header, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("expected file field: %v", err)
		}
		if header.Filename != "test.mp3" {
			t.Errorf("expected filename test.mp3, got %s", header.Filename)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"text": "hello world"}`))
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	audioFile := filepath.Join(tmpDir, "test.mp3")
	os.WriteFile(audioFile, []byte("fake audio data"), 0644)

	text, err := TranscribeAudio(context.Background(), audioFile, server.URL, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if text != "hello world" {
		t.Errorf("expected 'hello world', got %q", text)
	}
}

func TestTranscribeAudioWithAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("expected Authorization header 'Bearer test-key', got %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"text": "authenticated"}`))
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	audioFile := filepath.Join(tmpDir, "test.wav")
	os.WriteFile(audioFile, []byte("fake audio data"), 0644)

	text, err := TranscribeAudio(context.Background(), audioFile, server.URL, "test-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if text != "authenticated" {
		t.Errorf("expected 'authenticated', got %q", text)
	}
}

func TestTranscribeAudioBadStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	audioFile := filepath.Join(tmpDir, "test.mp3")
	os.WriteFile(audioFile, []byte("fake audio data"), 0644)

	_, err := TranscribeAudio(context.Background(), audioFile, server.URL, "")
	if err == nil {
		t.Fatal("expected error for bad status code")
	}
}

func TestTranscribeAudioInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not json"))
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	audioFile := filepath.Join(tmpDir, "test.mp3")
	os.WriteFile(audioFile, []byte("fake audio data"), 0644)

	_, err := TranscribeAudio(context.Background(), audioFile, server.URL, "")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestTranscribeAudioFileNotFound(t *testing.T) {
	_, err := TranscribeAudio(context.Background(), "/nonexistent/file.mp3", "http://localhost", "")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}
