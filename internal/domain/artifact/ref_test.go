package artifact

import "testing"

func TestRefValidate(t *testing.T) {
	valid := Ref{
		URI:    "cas://sha256/abc",
		Digest: "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		Size:   42,
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid ref rejected: %v", err)
	}

	cases := []Ref{
		{Digest: valid.Digest},
		{URI: valid.URI, Digest: "md5:deadbeef"},
		{URI: valid.URI, Digest: "sha256:not-hex"},
		{URI: valid.URI, Digest: valid.Digest, Size: -1},
	}
	for i, tc := range cases {
		if err := tc.Validate(); err == nil {
			t.Fatalf("case %d: expected validation error", i)
		}
	}
}
