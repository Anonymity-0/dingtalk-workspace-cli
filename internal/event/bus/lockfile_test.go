// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package bus

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestAcquire_WritesOurPID(t *testing.T) {
	path := filepath.Join(t.TempDir(), LockFileName)
	l, err := Acquire(path)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	defer l.Close()

	got := ReadHolderPID(path)
	if got != os.Getpid() {
		t.Fatalf("ReadHolderPID = %d, want %d", got, os.Getpid())
	}
}

func TestAcquire_SecondCallerGetsBusy(t *testing.T) {
	path := filepath.Join(t.TempDir(), LockFileName)
	l1, err := Acquire(path)
	if err != nil {
		t.Fatalf("first Acquire: %v", err)
	}
	defer l1.Close()

	_, err = Acquire(path)
	if !errors.Is(err, ErrBusy) {
		t.Fatalf("second Acquire = %v, want ErrBusy", err)
	}
}

func TestAcquire_StaleOrphanIsReclaimed(t *testing.T) {
	path := filepath.Join(t.TempDir(), LockFileName)
	// Pre-populate file with a definitely-dead PID (max int32 is unlikely to be alive).
	if err := os.WriteFile(path, []byte("2147483646\n"), 0o600); err != nil {
		t.Fatalf("pre-populate: %v", err)
	}
	l, err := Acquire(path)
	if err != nil {
		t.Fatalf("Acquire on stale orphan: %v", err)
	}
	defer l.Close()

	if got := ReadHolderPID(path); got != os.Getpid() {
		t.Fatalf("after orphan reclaim, ReadHolderPID = %d, want %d", got, os.Getpid())
	}
}

func TestAcquire_EmptyExistingFileWorks(t *testing.T) {
	path := filepath.Join(t.TempDir(), LockFileName)
	if err := os.WriteFile(path, []byte{}, 0o600); err != nil {
		t.Fatalf("pre-create: %v", err)
	}
	l, err := Acquire(path)
	if err != nil {
		t.Fatalf("Acquire on empty file: %v", err)
	}
	defer l.Close()
	if got := ReadHolderPID(path); got != os.Getpid() {
		t.Fatalf("ReadHolderPID = %d, want %d", got, os.Getpid())
	}
}

func TestClose_BlanksPID(t *testing.T) {
	path := filepath.Join(t.TempDir(), LockFileName)
	l, err := Acquire(path)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if err := l.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	// Body should be empty (or at least not our PID anymore).
	if got := ReadHolderPID(path); got != 0 {
		t.Fatalf("after Close, ReadHolderPID = %d, want 0", got)
	}
}

func TestReadHolderPID_MissingFileReturnsZero(t *testing.T) {
	if got := ReadHolderPID(filepath.Join(t.TempDir(), "does-not-exist")); got != 0 {
		t.Fatalf("ReadHolderPID(missing) = %d, want 0", got)
	}
}

func TestReadHolderPID_MalformedReturnsZero(t *testing.T) {
	path := filepath.Join(t.TempDir(), "junk.lock")
	if err := os.WriteFile(path, []byte("not-a-pid"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if got := ReadHolderPID(path); got != 0 {
		t.Fatalf("ReadHolderPID(malformed) = %d, want 0", got)
	}
}

func TestCrossPlatformCoverageValidateHolderOwnershipHeldLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), LockFileName)
	held, err := busTryAcquire(path)
	if err != nil {
		t.Fatalf("hold lock: %v", err)
	}
	defer held.Close()
	if err := truncateAndWritePID(held.File(), os.Getpid()); err != nil {
		t.Fatalf("write holder PID: %v", err)
	}

	owned, err := ValidateHolderOwnership(path, os.Getpid())
	if err != nil {
		t.Fatalf("ValidateHolderOwnership: %v", err)
	}
	if !owned {
		t.Fatal("held lock was not attributed to its recorded PID")
	}
}

func TestCrossPlatformCoverageValidateHolderOwnershipClearsReusedPID(t *testing.T) {
	path := filepath.Join(t.TempDir(), LockFileName)
	// The current PID is alive but deliberately does not hold this lock. This
	// models a stale daemon PID that the OS has reassigned to another process.
	if err := os.WriteFile(path, []byte(fmt.Sprintf("%d\n", os.Getpid())), 0o600); err != nil {
		t.Fatalf("write stale PID: %v", err)
	}

	owned, err := ValidateHolderOwnership(path, os.Getpid())
	if err != nil {
		t.Fatalf("ValidateHolderOwnership: %v", err)
	}
	if owned {
		t.Fatal("live reused PID without the bus lock was accepted as owner")
	}
	if got := ReadHolderPID(path); got != 0 {
		t.Fatalf("stale PID after validation = %d, want cleared", got)
	}
}

func TestCrossPlatformCoverageValidateHolderOwnershipRejectsMismatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), LockFileName)
	if err := os.WriteFile(path, []byte("123\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	owned, err := ValidateHolderOwnership(path, 456)
	if err != nil || owned {
		t.Fatalf("ValidateHolderOwnership mismatch = %v, %v", owned, err)
	}
}

func TestAcquire_AfterReleaseReclaimable(t *testing.T) {
	path := filepath.Join(t.TempDir(), LockFileName)
	l1, err := Acquire(path)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	if err := l1.Close(); err != nil {
		t.Fatalf("close first: %v", err)
	}
	l2, err := Acquire(path)
	if err != nil {
		t.Fatalf("reclaim after Close: %v", err)
	}
	defer l2.Close()
}

// Sanity check that PID round-trips through truncateAndWritePID.
func TestTruncateAndWritePID_Roundtrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x")
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := truncateAndWritePID(f, 9999); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	want := fmt.Sprintf("%d\n", 9999)
	if string(b) != want {
		t.Fatalf("body = %q, want %q", b, want)
	}
}
