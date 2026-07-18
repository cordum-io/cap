package worker

import "errors"

func (w *ManagedWorker) beginRun() error {
	w.runMu.Lock()
	defer w.runMu.Unlock()
	if w.runStarted {
		return errors.New("worker: Run already called")
	}
	w.runStarted = true
	return nil
}
