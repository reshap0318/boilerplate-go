package helpers

import "testing"

// The reset-password flow stores HashToken(token) and later looks it up with
// WHERE token = HashToken(token), so the hash MUST be deterministic.
func TestHashTokenIsDeterministic(t *testing.T) {
	if HashToken("abc") != HashToken("abc") {
		t.Fatal("HashToken must return the same digest for the same input")
	}
	if HashToken("abc") == HashToken("abd") {
		t.Fatal("HashToken must differ for different input")
	}
}

// HashString is bcrypt: salted, so it can never be used as a lookup key.
func TestHashStringIsSalted(t *testing.T) {
	a, _ := HashString("abc")
	b, _ := HashString("abc")
	if a == b {
		t.Fatal("expected bcrypt hashes to differ")
	}
	if VerifyString("abc", a) != nil {
		t.Fatal("VerifyString should accept the original input")
	}
}
