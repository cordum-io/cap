package main

import "testing"

func TestNewEchoWorkerExplicitlyOptsIntoUnsignedLocalDevelopment(t *testing.T) {
	w := newEchoWorker(nil)
	if !w.AllowUnsigned {
		t.Fatal("simple echo worker must explicitly opt into unsigned local-development transport")
	}
	if w.SenderID != workerID || w.Subject != workerSubject || w.Handler == nil {
		t.Fatalf("worker configuration = sender %q subject %q handler_nil=%t", w.SenderID, w.Subject, w.Handler == nil)
	}
}
