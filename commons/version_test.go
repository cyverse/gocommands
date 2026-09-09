package commons

import "testing"

func TestGetVersionPartsAcceptsOptionalVPrefix(t *testing.T) {
	for _, version := range []string{"v1.10.0", "1.10.0"} {
		major, minor, patch := GetVersionParts(version)
		if major != 1 || minor != 10 || patch != 0 {
			t.Errorf("GetVersionParts(%q) = (%d, %d, %d), want (1, 10, 0)", version, major, minor, patch)
		}
	}
}

func TestHasNewReleaseWithMixedVersionPrefixes(t *testing.T) {
	if HasNewRelease("v1.10.0", "1.10.0") {
		t.Error("HasNewRelease() reported an identical version as newer")
	}
	if !HasNewRelease("v1.10.0", "1.11.0") {
		t.Error("HasNewRelease() did not report a newer version")
	}
}
