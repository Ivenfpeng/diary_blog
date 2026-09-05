package main

import (
	"context"
	"net/http"
	"testing"
	"time"
)

type shutdownSpy struct {
	deadline time.Time
}

func (s *shutdownSpy) Shutdown(ctx context.Context) error {
	s.deadline, _ = ctx.Deadline()
	return nil
}

func TestShutdownUsesTenSecondTimeout(t *testing.T) {
	before := time.Now()
	server := &shutdownSpy{}
	if err := shutdownServer(server); err != nil {
		t.Fatalf("shutdown server: %v", err)
	}
	if server.deadline.Before(before.Add(9*time.Second)) || server.deadline.After(before.Add(11*time.Second)) {
		t.Fatalf("shutdown deadline = %v, want approximately ten seconds after %v", server.deadline, before)
	}
}

var _ interface{ Shutdown(context.Context) error } = (*http.Server)(nil)
