package bridge

import (
	"context"
	"testing"
	"time"
)

func TestNewRelayConn(t *testing.T) {
	rc := NewRelayConn("wss://relay.example.com", nil)
	if rc == nil {
		t.Fatal("expected non-nil RelayConn")
	}
	if rc.url != "wss://relay.example.com" {
		t.Errorf("url = %q, want wss://relay.example.com", rc.url)
	}
}

func TestRelayConn_PublishNotConnected(t *testing.T) {
	rc := NewRelayConn("wss://relay.example.com", nil)
	err := rc.Publish(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error from Publish without connection")
	}
	if err.Error() != "not connected to relay" {
		t.Errorf("error = %q", err)
	}
}

func TestRelayConn_SubscribeNotConnected(t *testing.T) {
	rc := NewRelayConn("wss://relay.example.com", nil)
	_, err := rc.Subscribe(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error from Subscribe without connection")
	}
	if err.Error() != "not connected to relay" {
		t.Errorf("error = %q", err)
	}
}

func TestRelayConn_CloseWithoutConnect(t *testing.T) {
	rc := NewRelayConn("wss://relay.example.com", nil)
	// Should not panic
	rc.Close()
}

func TestRelayConn_CloseWithCancel(t *testing.T) {
	rc := NewRelayConn("wss://relay.example.com", nil)
	rc.ctx, rc.cancel = context.WithCancel(context.Background())
	// Should cancel context and not panic
	rc.Close()
}

func TestRelayConn_ConnectInvalidURL(t *testing.T) {
	rc := NewRelayConn("not-a-valid-url", nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := rc.Connect(ctx)
	if err == nil {
		t.Fatal("expected error connecting to invalid URL")
	}
}

func TestRelayConn_ReconnectCancelledContext(t *testing.T) {
	rc := NewRelayConn("wss://nonexistent.example.com", nil)
	ctx, cancel := context.WithCancel(context.Background())
	rc.ctx = ctx
	rc.cancel = cancel

	// Cancel immediately so Reconnect exits on first iteration
	cancel()

	err := rc.Reconnect()
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}
