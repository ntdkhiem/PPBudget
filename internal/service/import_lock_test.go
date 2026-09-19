package service

import (
	"sync"
	"testing"
)

func TestImportLockIsPerUser(t *testing.T) {
	const userA, userB = "user-a", "user-b"
	t.Cleanup(func() {
		releaseImportLock(userA)
		releaseImportLock(userB)
	})

	if !tryAcquireImportLock(userA) {
		t.Fatal("first acquire for user A should succeed")
	}
	if tryAcquireImportLock(userA) {
		t.Error("second acquire for user A should fail while the first is held")
	}

	// One user's import must not block another's.
	if !tryAcquireImportLock(userB) {
		t.Error("user B should be able to import while user A is importing")
	}

	releaseImportLock(userA)
	if !tryAcquireImportLock(userA) {
		t.Error("user A should be able to acquire again after release")
	}
}

func TestImportLockRunningReflectsState(t *testing.T) {
	const user = "user-running"
	t.Cleanup(func() { releaseImportLock(user) })

	if ImportRunning(user) {
		t.Fatal("no import should be running before acquire")
	}
	tryAcquireImportLock(user)
	if !ImportRunning(user) {
		t.Error("ImportRunning should report true while held")
	}
	releaseImportLock(user)
	if ImportRunning(user) {
		t.Error("ImportRunning should report false after release")
	}
}

// TestImportLockExactlyOneWinner is the property that matters: when the
// in-process cron, the GitHub Actions ping, and a manual sync all arrive at
// once, exactly one proceeds. Run with -race.
func TestImportLockExactlyOneWinner(t *testing.T) {
	const user = "user-concurrent"
	t.Cleanup(func() { releaseImportLock(user) })

	const goroutines = 50
	var wg sync.WaitGroup
	var mu sync.Mutex
	winners := 0

	start := make(chan struct{})
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start // maximize contention
			if tryAcquireImportLock(user) {
				mu.Lock()
				winners++
				mu.Unlock()
			}
		}()
	}
	close(start)
	wg.Wait()

	if winners != 1 {
		t.Errorf("%d goroutines acquired the lock, want exactly 1", winners)
	}
}
