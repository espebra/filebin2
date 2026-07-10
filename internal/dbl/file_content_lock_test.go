package dbl

import (
	"testing"
	"time"
)

func TestContentLockExcludesTryLock(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tearDown(dao) }()

	sha256 := "0000000000000000000000000000000000000000000000000000000000000001"

	unlock, err := dao.FileContent().LockContent(sha256)
	if err != nil {
		t.Fatal(err)
	}

	// The lock is held, so a try-lock on the same hash must not acquire it.
	tryUnlock, acquired, err := dao.FileContent().TryLockContent(sha256)
	if err != nil {
		t.Fatal(err)
	}
	if acquired {
		tryUnlock()
		t.Fatal("TryLockContent acquired a lock that was already held")
	}

	// A different hash locks independently.
	otherSha256 := "0000000000000000000000000000000000000000000000000000000000000002"
	otherUnlock, acquired, err := dao.FileContent().TryLockContent(otherSha256)
	if err != nil {
		t.Fatal(err)
	}
	if !acquired {
		t.Fatal("TryLockContent failed to acquire a lock on a different hash")
	}
	otherUnlock()

	unlock()

	// After release, the lock is available again.
	tryUnlock, acquired, err = dao.FileContent().TryLockContent(sha256)
	if err != nil {
		t.Fatal(err)
	}
	if !acquired {
		t.Fatal("TryLockContent failed to acquire a released lock")
	}
	tryUnlock()

	// Unlock is idempotent; extra calls must be safe.
	unlock()
	tryUnlock()
}

func TestContentLockBlocksUntilReleased(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tearDown(dao) }()

	sha256 := "0000000000000000000000000000000000000000000000000000000000000003"

	unlock, err := dao.FileContent().LockContent(sha256)
	if err != nil {
		t.Fatal(err)
	}

	acquiredChan := make(chan error, 1)
	go func() {
		blockedUnlock, err := dao.FileContent().LockContent(sha256)
		if err == nil {
			blockedUnlock()
		}
		acquiredChan <- err
	}()

	// The goroutine must stay blocked while the lock is held.
	select {
	case <-acquiredChan:
		t.Fatal("LockContent acquired a lock that was already held")
	case <-time.After(500 * time.Millisecond):
	}

	unlock()

	// After release, the blocked goroutine must acquire the lock promptly.
	select {
	case err := <-acquiredChan:
		if err != nil {
			t.Fatalf("LockContent failed after the lock was released: %s", err.Error())
		}
	case <-time.After(10 * time.Second):
		t.Fatal("LockContent did not acquire the lock after it was released")
	}
}
