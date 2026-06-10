package config

import (
	"os"
	"testing"
	"time"

	"zenflow/pkg/filter"
)

func TestWatchLocalFile_EmptyFileClearsBlocklist(t *testing.T) {
	// 1. Create a valid file
	tmpFile, err := os.CreateTemp("", "hosts_test_*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	validData := "0.0.0.0 bad-domain.com\n"
	tmpFile.Write([]byte(validData))
	tmpFile.Close()

	// Initialize blocker with valid data
	domains, err := FetchFromFile(tmpFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	b := filter.NewBlocker(domains)

	if blocked, _ := b.IsBlocked("bad-domain.com"); !blocked {
		t.Fatal("Expected bad-domain.com to be blocked initially")
	}

	// 2. Start watcher
	WatchLocalFile(tmpFile.Name(), b, 100*time.Millisecond)

	// 3. Wait a bit, then overwrite file with garbage (e.g. user made a typo while editing)
	time.Sleep(200 * time.Millisecond)
	err = os.WriteFile(tmpFile.Name(), []byte("just some garbage data"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// 4. Wait for watcher to poll
	time.Sleep(300 * time.Millisecond)

	// 5. Check if the blocklist was cleared out
	// 5. Check if the blocklist was cleared out
	if blocked, _ := b.IsBlocked("bad-domain.com"); blocked {
		t.Errorf("Expected blocklist to be cleared, but it wasn't")
	}
}
