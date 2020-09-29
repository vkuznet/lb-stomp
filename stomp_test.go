package lbstomp

import (
	"testing"
)

// TestStompMgr tests functionality of stomp Mgr
func TestStompMgr(t *testing.T) {
	uri := "www.yahoo.com:12345"

	// improper configuration, lack of Login/Password
	c := Config{StompURI: uri}
	m := New(c)
	_, addr, err := m.getConnection()

	if err == nil {
		t.Errorf("did not fail with empty login")
	}

	// proper configuration
	config := Config{StompURI: uri, StompLogin: "test", StompPassword: "test"}
	mgr := New(config)
	_, addr, err = mgr.getConnection()

	if addr == "" {
		t.Errorf("unable to resolve uri %s addr %v\n", uri, addr)
	}
	if err != nil {
		t.Errorf("unable to get connections, error %v\n", err)
	}

	// send data chunk
	data := []byte(`{"test":1}`)
	err = mgr.Send(data)
	if err != nil {
		t.Fatalf("Unable to send data, error %v\n", err)
	}
}
