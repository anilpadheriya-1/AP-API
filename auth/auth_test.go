package auth

import (
	"testing"
)

// A basic test to make sure syntax is correct
func TestAuthResult(t *testing.T) {
    if AuthOK != 0 {
		t.Errorf("Expected AuthOK to be 0")
	}
}
