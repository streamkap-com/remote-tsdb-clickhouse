package clickhouse

import "testing"

func TestTLSConfigVerification(t *testing.T) {
	if !tlsConfig(false).InsecureSkipVerify {
		t.Error("verify off must skip certificate verification (legacy default)")
	}
	if tlsConfig(true).InsecureSkipVerify {
		t.Error("verify on must not skip certificate verification")
	}
}
