package encryption

import "testing"

func TestIsCorrectFilename(t *testing.T) {
	testCases := []struct {
		filename string
		want     bool
	}{
		{filename: "plain-file.txt", want: true},
		{filename: "backup~.txt", want: true},
		{filename: "한글 파일.txt", want: true},
		{filename: "line\nbreak", want: false},
		{filename: "delete\x7f", want: false},
		{filename: string([]byte{0xff}), want: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.filename, func(t *testing.T) {
			if got := IsCorrectFilename([]byte(testCase.filename)); got != testCase.want {
				t.Errorf("IsCorrectFilename(%q) = %t, want %t", testCase.filename, got, testCase.want)
			}
		})
	}
}

func TestWinSCPFilenameRoundTripWithUnicode(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	want := "보고서~최종본.txt"

	encrypted, err := EncryptFilenameWinSCP(want, key)
	if err != nil {
		t.Fatalf("EncryptFilenameWinSCP() error = %v", err)
	}
	got, err := DecryptFilenameWinSCP(encrypted, key)
	if err != nil {
		t.Fatalf("DecryptFilenameWinSCP() error = %v", err)
	}
	if got != want {
		t.Errorf("decrypted filename = %q, want %q", got, want)
	}
}
