package art

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDumper(t *testing.T) {
	cases := []struct {
		Name  string
		Input map[string]string
		Want  string
	}{
		{
			Name: "simples",
			Input: map[string]string{
				"foo":    "FOO",
				"bar":    "BAR",
				"foobar": "FOOBAR",
				// "fooboo": "FOOBOO",
			},
			Want: "sad",
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			require := require.New(t)

			art := New()

			var ok bool
			for k, v := range tc.Input {
				art, _, ok = art.Insert([]byte(k), v)
				require.False(ok)
			}

			d := dumper{
				root: art.root,
			}

			got := d.String()

			t.Log("\n" + got)
			require.Equal(tc.Want, got)
		})
	}
}
