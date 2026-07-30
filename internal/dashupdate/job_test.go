package dashupdate

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicFileStateReportsCommittedAfterDirectorySyncFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.env")
	syncErr := errors.New("sync failed")
	committed, err := writeAtomicFileWithSync(path, []byte("state\n"), 0o600, func(string) error {
		return syncErr
	})
	if !committed || !errors.Is(err, syncErr) {
		t.Fatalf("writeAtomicFileWithSync() = (%v, %v), want committed sync failure", committed, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read committed file: %v", err)
	}
	if string(data) != "state\n" {
		t.Fatalf("committed file = %q", data)
	}
}

func TestRemoveDurableReportsRemovedAfterDirectorySyncFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.env")
	if err := os.WriteFile(path, []byte("state\n"), 0o600); err != nil {
		t.Fatalf("write state: %v", err)
	}
	syncErr := errors.New("sync failed")
	removed, err := removeDurableWithSync(path, func(string) error {
		return syncErr
	})
	if !removed || !errors.Is(err, syncErr) {
		t.Fatalf("removeDurableWithSync() = (%v, %v), want removed sync failure", removed, err)
	}
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("removed path error = %v, want not exist", err)
	}
}

func TestCompletedTransactionDoesNotRequireRecovery(t *testing.T) {
	paths := runnerPathsForHome(t.TempDir())
	if err := writeTransaction(paths.transactionPath, updateTransaction{
		JobID: "completed-job",
		Phase: PhaseDone,
	}); err != nil {
		t.Fatalf("writeTransaction() error = %v", err)
	}
	pending, err := recoveryStatePending(paths)
	if err != nil {
		t.Fatalf("recoveryStatePending() error = %v", err)
	}
	if pending {
		t.Fatal("completed transaction requires recovery")
	}
}
