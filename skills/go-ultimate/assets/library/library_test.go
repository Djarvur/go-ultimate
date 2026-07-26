package library_test

import (
	"testing"

	"example.com/library"
)

// Tests live in an external test package (package library_test) per the skill's
// testing convention. Name tests like examples: TestF_suffixCamelCase.
// See go-ultimate/references/testing.md.
func TestHello(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		want string
	}{
		{name: "world", in: "World", want: "Hello, World!"},
		{name: "empty", in: "", want: "Hello, !"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := library.Hello(tc.in); got != tc.want {
				t.Errorf("Hello(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
