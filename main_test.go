package main

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestParseFormatCodes(t *testing.T) {
	jsonData := `{
		"formats": [
			{"format_id": "720p60"},
			{"format_id": "480p"},
			{"format_id": "audio_only"},
			{"format_id": "1080p60"},
			{"format_id": "720p60"},
			{"format_id": "160p"}
		]
	}`

	got, err := parseFormatCodes(jsonData)
	if err != nil {
		t.Fatalf("parseFormatCodes() error = %v", err)
	}

	want := []string{"1080p60", "720p60", "480p", "160p"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseFormatCodes() = %v, want %v", got, want)
	}
}

func TestParseFormatCodesInvalidJSON(t *testing.T) {
	_, err := parseFormatCodes("{invalid")
	if err == nil {
		t.Fatal("expected JSON parse error, got nil")
	}
}

func TestClassifyYTDLPFailure(t *testing.T) {
	tests := []struct {
		name   string
		runErr error
		output string
		want   string
	}{
		{
			name:   "offline",
			runErr: errors.New("exit status 1"),
			output: "ERROR: somechannel is offline",
			want:   "stream is offline",
		},
		{
			name:   "geo",
			runErr: errors.New("exit status 1"),
			output: "ERROR: This stream is not available in your country",
			want:   "stream is geo-restricted from your location",
		},
		{
			name:   "auth",
			runErr: errors.New("exit status 1"),
			output: "ERROR: Login required; cookies are needed",
			want:   "stream requires authentication (cookies/login)",
		},
		{
			name:   "missing ytdlp",
			runErr: errors.New("exec: \"yt-dlp\": executable file not found in $PATH"),
			output: "",
			want:   "yt-dlp is not installed or not in PATH",
		},
		{
			name:   "unknown",
			runErr: errors.New("exit status 1"),
			output: "ERROR: unexpected failure",
			want:   "yt-dlp returned an unknown error",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyYTDLPFailure(tc.runErr, tc.output)
			if got != tc.want {
				t.Fatalf("classifyYTDLPFailure() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFormatYTDLPErrorNonVerbose(t *testing.T) {
	err := formatYTDLPError("foo", errors.New("exit status 1"), "ERROR: foo is offline", false)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	text := err.Error()
	if !strings.Contains(text, "stream is offline") {
		t.Fatalf("expected offline summary, got %q", text)
	}
	if !strings.Contains(text, "rerun with -v") {
		t.Fatalf("expected verbose hint, got %q", text)
	}
}
