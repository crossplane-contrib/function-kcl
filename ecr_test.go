package main

import "testing"

func TestRegionFromECRRegistry(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"123456789012.dkr.ecr.us-west-2.amazonaws.com", "us-west-2", false},
		{"https://123456789012.dkr.ecr.eu-central-1.amazonaws.com", "eu-central-1", false},
		{"oci://123456789012.dkr.ecr.us-west-2.amazonaws.com/org/pkg", "us-west-2", false},
		{"ghcr.io/org/pkg", "", true},
		{"", "", true},
	}
	for _, c := range cases {
		got, err := regionFromECRRegistry(c.in)
		if (err != nil) != c.wantErr {
			t.Errorf("%q: err = %v, wantErr %v", c.in, err, c.wantErr)
			continue
		}
		if got != c.want {
			t.Errorf("%q: got %q, want %q", c.in, got, c.want)
		}
	}
}
