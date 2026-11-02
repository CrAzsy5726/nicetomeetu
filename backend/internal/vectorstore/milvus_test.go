package vectorstore

import (
	"errors"
	"testing"
)

func TestIsNoResultErr(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"extra output fields", errors.New("extra output fields [content] found and result does not dynamic field"), true},
		{"field data type not matched", errors.New("FieldData type not matched"), true},
		{"connection error", errors.New("connection refused"), false},
		{"nil", nil, false},
	}
	for _, c := range cases {
		if got := isNoResultErr(c.err); got != c.want {
			t.Errorf("%s: isNoResultErr(%v) = %v, want %v", c.name, c.err, got, c.want)
		}
	}
}
