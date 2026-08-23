package account

import "testing"

func TestPasswordHasherRoundTrip(t *testing.T) {
	t.Parallel()
	hasher := passwordHasher{}
	encoded, err := hasher.Hash("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if !hasher.Verify("correct horse battery staple", encoded) {
		t.Fatal("valid password did not verify")
	}
	if hasher.Verify("wrong password", encoded) {
		t.Fatal("invalid password verified")
	}
}
