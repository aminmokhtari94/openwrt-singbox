package health

import (
	"net/http"
	"testing"
)

func TestParseDelayResponseSuccess(t *testing.T) {
	ms, err := parseDelayResponse(http.StatusOK, []byte(`{"delay":187}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ms != 187 {
		t.Fatalf("expected 187ms, got %d", ms)
	}
}

func TestParseDelayResponseZeroDelayIsFailure(t *testing.T) {
	if _, err := parseDelayResponse(http.StatusOK, []byte(`{"delay":0}`)); err == nil {
		t.Fatal("expected zero delay to be treated as a failure")
	}
}

func TestParseDelayResponseInvalidBody(t *testing.T) {
	if _, err := parseDelayResponse(http.StatusOK, []byte(`not-json`)); err == nil {
		t.Fatal("expected invalid body to error")
	}
}

func TestParseDelayResponseUsesServerMessage(t *testing.T) {
	_, err := parseDelayResponse(http.StatusGatewayTimeout, []byte(`{"message":"An error occurred in the delay test"}`))
	if err == nil {
		t.Fatal("expected non-200 to error")
	}
	if err.Error() != "An error occurred in the delay test" {
		t.Fatalf("expected server message surfaced, got %q", err.Error())
	}
}

func TestParseDelayResponseFallsBackToStatus(t *testing.T) {
	_, err := parseDelayResponse(http.StatusNotFound, []byte(``))
	if err == nil {
		t.Fatal("expected error for empty non-200 body")
	}
}
