package postgis

import (
	"testing"

	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/geom/slippy"
)

func TestReplaceTokens(t *testing.T) {
	testcases := []struct {
		layer    Layer
		tile     *slippy.Tile
		expected string
	}{
		{
			layer: Layer{
				sql:  "SELECT * FROM foo WHERE geom && !BBOX!",
				srid: tegola.WebMercator,
			},
			tile:     slippy.NewTile(2, 1, 1, 64, tegola.WebMercator),
			expected: "SELECT * FROM foo WHERE geom && ST_MakeEnvelope(-1.017529720390625e+07,1.017529720390625e+07,156543.03390624933,-156543.03390624933,3857)",
		},
		{
			layer: Layer{
				sql:  "SELECT id, scalerank=!ZOOM! FROM foo WHERE geom && !BBOX!",
				srid: tegola.WebMercator,
			},
			tile:     slippy.NewTile(2, 1, 1, 64, tegola.WebMercator),
			expected: "SELECT id, scalerank=2 FROM foo WHERE geom && ST_MakeEnvelope(-1.017529720390625e+07,1.017529720390625e+07,156543.03390624933,-156543.03390624933,3857)",
		},
	}

	for i, tc := range testcases {
		sql, err := replaceTokens(&tc.layer, tc.tile)
		if err != nil {
			t.Errorf("[%v] unexpected error, Expected nil Got %v", i, err)
			continue
		}

		if sql != tc.expected {
			t.Errorf("[%v] incorrect sql, Expected (%v) Got (%v)", i, tc.expected, sql)
		}
	}
}
