package operations

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestInstallStagedSiteRestoresPreviousDataWhenInstallFails(t *testing.T) {
	parent := t.TempDir()
	destination := filepath.Join(parent, "site")
	stage := filepath.Join(parent, "stage")
	if err := os.Mkdir(destination, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(destination, "keep"), []byte("previous"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(stage, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stage, "new"), []byte("replacement"), 0o600); err != nil {
		t.Fatal(err)
	}

	originalRename := renamePath
	t.Cleanup(func() { renamePath = originalRename })
	renamePath = func(old, new string) error {
		if old == stage && new == destination {
			return errors.New("injected install failure")
		}
		return os.Rename(old, new)
	}
	if err := installStagedSite(stage, destination); err == nil {
		t.Fatal("install unexpectedly succeeded")
	}
	contents, err := os.ReadFile(filepath.Join(destination, "keep"))
	if err != nil || string(contents) != "previous" {
		t.Fatalf("previous destination was not restored: %q err=%v", contents, err)
	}
}
