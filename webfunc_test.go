package oom

import (
	"fmt"
	"testing"
)

func TestLogin(t *testing.T) {
	fmt.Println("TestLogin")
	err := login()
	if err != nil {
		t.Errorf("Expected nil, got err", err)
	}
}
