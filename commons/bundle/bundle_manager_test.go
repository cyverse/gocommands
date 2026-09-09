package bundle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cyverse/gocommands/commons/terminal"
)

func TestIsBundleFilename(t *testing.T) {
	manager := NewBundleManager(1, 1, 1, t.TempDir(), "/staging")

	testCases := []struct {
		name string
		want bool
	}{
		{name: "bundle_abc.tar", want: true},
		{name: "bundle_abc.tar.gz", want: false},
		{name: "other.tar", want: false},
		{name: ".tar", want: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := manager.IsBundleFilename(testCase.name); got != testCase.want {
				t.Errorf("IsBundleFilename(%q) = %t, want %t", testCase.name, got, testCase.want)
			}
		})
	}
}

func TestClearLocalBundlesRemovesOnlyBundles(t *testing.T) {
	terminal.InitTerminalOutput()

	tempDir := t.TempDir()
	manager := NewBundleManager(1, 1, 1, tempDir, "/staging")

	for _, filename := range []string{"bundle_abc.tar", "keep.txt", "bundle_abc.tar.gz"} {
		if err := os.WriteFile(filepath.Join(tempDir, filename), nil, 0600); err != nil {
			t.Fatalf("WriteFile(%q): %v", filename, err)
		}
	}

	if err := manager.ClearLocalBundles(); err != nil {
		t.Fatalf("ClearLocalBundles() error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(tempDir, "bundle_abc.tar")); !os.IsNotExist(err) {
		t.Errorf("bundle file still exists, stat error = %v", err)
	}
	for _, filename := range []string{"keep.txt", "bundle_abc.tar.gz"} {
		if _, err := os.Stat(filepath.Join(tempDir, filename)); err != nil {
			t.Errorf("non-bundle file %q was removed: %v", filename, err)
		}
	}
}
