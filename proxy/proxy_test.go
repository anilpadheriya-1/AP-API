package proxy

import (
	"testing"
)

// A basic test to make sure syntax is correct
func TestProxyStruct(t *testing.T) {
    p := ErrorResponse{
        Error: "Test",
        Message: "Test",
    }
    if p.Error != "Test" {
        t.Errorf("Expected Test")
    }
}
