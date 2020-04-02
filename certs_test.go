package pack

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestNewCertConfig(t *testing.T) {
	type test struct {
		input []string
		want  CertConfig
	}

	args := []string{
		"build:./build.crt",
		"run:/absolute/run.crt",
		"./both.crt",
	}

	tests := []test{
		{
			input: args,
			want: CertConfig{
				Build: []string{"./build.crt", "./both.crt"},
				Run:   []string{"/absolute/run.crt", "./both.crt"},
			},
		},
	}
	for _, tc := range tests {
		cfg := NewCertConfig(tc.input)

		if s := cmp.Diff(cfg, tc.want); s != "" {
			t.Fatalf("Unexpected Certconfig:\n%s\n", s)
		}
	}

}
