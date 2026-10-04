package frame

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestAKeyComesBackFromItsText(t *testing.T) {
	k := NewKey()

	got, err := ParseKey(k.Text())
	if err != nil {
		t.Fatal(err)
	}

	if got != k {
		t.Error("the parsed key differs from the one written")
	}
}

func TestAKeyFileMayEndInANewline(t *testing.T) {
	k := NewKey()

	for _, end := range []string{"\n", "\r\n", " \t\r\n"} {
		got, err := ParseKey(k.Text() + end)
		if err != nil || got != k {
			t.Errorf("text ending in %q: err %v, same key %v", end, err, got == k)
		}
	}
}

func TestTwoNewKeysDiffer(t *testing.T) {
	if NewKey() == NewKey() {
		t.Error("two new keys are equal")
	}
}

func TestParseKeyRefusesWhatIsNotAKey(t *testing.T) {
	cases := []struct {
		name string
		text string
		want error
	}{
		{"not base64", "this is not a key!", ErrKeyText},
		{"empty", "", ErrKeySize},
		{"one byte short", base64.StdEncoding.EncodeToString(make([]byte, 31)), ErrKeySize},
		{"one byte long", base64.StdEncoding.EncodeToString(make([]byte, 33)), ErrKeySize},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := ParseKey(c.text); !errors.Is(err, c.want) {
				t.Errorf("got %v, want %v", err, c.want)
			}
		})
	}
}

func TestAKeyErrorNeverQuotesTheText(t *testing.T) {
	text := NewKey().Text()
	broken := text[:len(text)-2] + "!="

	_, err := ParseKey(broken)
	if err == nil {
		t.Fatal("a broken key parsed")
	}

	if strings.Contains(err.Error(), broken[:8]) {
		t.Errorf("the error repeats the key text: %v", err)
	}
}

func TestAKeyNeverPrintsItself(t *testing.T) {
	k := NewKey()

	for _, verb := range []string{"%v", "%+v", "%#v", "%s", "%q", "%x", "%X", "%d"} {
		if out := fmt.Sprintf(verb, k); out != "frame.Key(hidden)" {
			t.Errorf("%s prints %q", verb, out)
		}
	}
}
