package auth

import "testing"

func TestHashAndCheckPassword_Success(t *testing.T) {
	password := "CorrectHorseBatteryStaple123!"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}

	if hash == "" {
		t.Fatal("hash should not be empty")
	}

	ok, err := CheckPasswordHash(password, hash)
	if err != nil {
		t.Fatalf("CheckPasswordHash returned error: %v", err)
	}

	if !ok {
		t.Fatal("expected password to match hash")
	}
}
