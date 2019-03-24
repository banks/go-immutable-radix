package art

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDumper(t *testing.T) {
	cases := []struct {
		Name  string
		Input []struct{ k, v string }
		Want  string
	}{
		{
			Name: "simples",
			Input: []struct{ k, v string }{
				{"foo", "FOO"},
				{"bar", "BAR"},
				{"foobar", "FOOBAR"},
				{"fooboo", "FOOBOO"},
				{"grunk", "GRUNK"},
				{"zap", "ZAP"},
				{"wop", "WOP"},
				{"A", "AAAA"},
				{"B", "BBBB"},
				{"C", "CCCC"},
				{"D", "DDDD"},
				{"E", "EEEE"},
				{"F", "FFFF"},
				{"G", "GGGG"},
				{"H", "HHHH"},
				{"I", "IIII"},
				{"J", "JJJJ"},
				{"K", "KKKK"},
				{"L", "LLLL"},
				{"M", "MMMM"},
				{"N", "NNNN"},
				{"O", "OOOO"},
				{"P", "PPPP"},
				{"Q", "QQQQ"},
				{"R", "RRRR"},
				{"S", "SSSS"},
				{"T", "TTTT"},
				{"U", "UUUU"},
				{"V", "VVVV"},
				{"W", "WWWW"},
				{"X", "XXXX"},
				{"Y", "YYYY"},
				{"Z", "ZZZZ"},
				{"a", "aaaa"},
				{"b", "bbbb"},
				{"c", "cccc"},
				{"d", "dddd"},
				{"e", "eeee"},
				{"f", "ffff"},
				{"g", "gggg"},
				{"h", "hhhh"},
				{"i", "iiii"},
				{"j", "jjjj"},
				{"k", "kkkk"},
				{"l", "llll"},
				{"m", "mmmm"},
				{"n", "nnnn"},
				{"o", "oooo"},
				{"p", "pppp"},
				{"q", "qqqq"},
				{"r", "rrrr"},
				{"s", "ssss"},
				{"t", "tttt"},
				{"u", "uuuu"},
				{"v", "vvvv"},
				{"w", "wwww"},
				{"x", "xxxx"},
				{"y", "yyyy"},
				{"z", "zzzz"},
			},
			Want: "sad",
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			require := require.New(t)

			art := New()

			var ok bool
			for _, kv := range tc.Input {
				art, _, ok = art.Insert([]byte(kv.k), kv.v)
				require.False(ok)

				d := dumper{
					root: art.root,
				}
				got := d.String()
				t.Logf("JUST INSERTED %s\n%s", kv.k, got)
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
