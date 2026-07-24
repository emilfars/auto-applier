package auth

import "testing"

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hash == "correct horse battery staple" {
		t.Fatal("hash equals plaintext")
	}
	if !VerifyPassword(hash, "correct horse battery staple") {
		t.Error("valid password rejected")
	}
	if VerifyPassword(hash, "wrong password") {
		t.Error("wrong password accepted")
	}
}

func TestHashPasswordEmpty(t *testing.T) {
	if _, err := HashPassword(""); err == nil {
		t.Error("expected error for empty password")
	}
}

func TestHashPasswordUniqueSalts(t *testing.T) {
	a, _ := HashPassword("same")
	b, _ := HashPassword("same")
	if a == b {
		t.Error("identical hashes for same password — salt not random")
	}
}

func TestVerifyPasswordMalformed(t *testing.T) {
	for _, bad := range []string{
		"", "notpbkdf2", "pbkdf2_sha256$x$y", "pbkdf2_sha256$abc$c2FsdA$aGFzaA",
		"bcrypt$1$a$b",
	} {
		if VerifyPassword(bad, "pw") {
			t.Errorf("malformed hash %q accepted", bad)
		}
	}
}

func TestNewTokenUniqueAndNonEmpty(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		tok, err := NewToken()
		if err != nil {
			t.Fatalf("NewToken: %v", err)
		}
		if tok == "" {
			t.Fatal("empty token")
		}
		if seen[tok] {
			t.Fatalf("duplicate token %q", tok)
		}
		seen[tok] = true
	}
}
