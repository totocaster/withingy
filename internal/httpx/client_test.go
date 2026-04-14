package httpx

import "testing"

func TestNewTransportDisablesHTTP2(t *testing.T) {
	transport := NewTransport()
	if transport.ForceAttemptHTTP2 {
		t.Fatalf("expected ForceAttemptHTTP2 to be false")
	}
}
