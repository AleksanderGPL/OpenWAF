package geoip

import (
	"context"
	"os"
	"testing"
)

// Opt in with GEOIP_LIVE_TEST=1 to verify the provider download and real IPv4/IPv6 data.
func TestLiveDatabase(t *testing.T) {
	if os.Getenv("GEOIP_LIVE_TEST") != "1" {
		t.Skip("set GEOIP_LIVE_TEST=1 for a real DB-IP download")
	}
	directory := t.TempDir()
	r, err := Open(context.Background(), directory)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	for _, ip := range []string{"8.8.8.8", "2001:4860:4860::8888"} {
		if got := r.CountryCode(ip); got == nil || len(*got) != 2 {
			t.Fatalf("%s: country %v", ip, got)
		} else {
			t.Logf("%s: %s", ip, *got)
		}
	}
	cached, err := Open(context.Background(), directory)
	if err != nil {
		t.Fatal(err)
	}
	defer cached.Close()
	if got := cached.CountryCode("8.8.8.8"); got == nil || *got != "US" {
		t.Fatalf("cached country: %v", got)
	}
}
