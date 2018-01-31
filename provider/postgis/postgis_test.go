package postgis_test

import (
	"os"
	"strconv"
	"testing"

	"context"

	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/geom/slippy"
	"github.com/terranodo/tegola/provider"
	"github.com/terranodo/tegola/provider/postgis"
)

func TestNewTileProvider(t *testing.T) {
	if os.Getenv("RUN_POSTGIS_TESTS") != "yes" {
		return
	}

	port, err := strconv.ParseInt(os.Getenv("PGPORT"), 10, 64)
	if err != nil {
		t.Fatalf("err parsing PGPORT: %v", err)
	}

	testcases := []struct {
		config map[string]interface{}
	}{
		{
			config: map[string]interface{}{
				postgis.ConfigKeyHost:     os.Getenv("PGHOST"),
				postgis.ConfigKeyPort:     port,
				postgis.ConfigKeyDB:       os.Getenv("PGDATABASE"),
				postgis.ConfigKeyUser:     os.Getenv("PGUSER"),
				postgis.ConfigKeyPassword: os.Getenv("PGPASSWORD"),
				postgis.ConfigKeyLayers: []map[string]interface{}{
					{
						postgis.ConfigKeyLayerName: "land",
						postgis.ConfigKeyTablename: "ne_10m_land_scale_rank",
					},
				},
			},
		},
	}

	for i, tc := range testcases {
		_, err := postgis.NewTileProvider(tc.config)
		if err != nil {
			t.Errorf("Failed test %v. Unable to create a new provider. err: %v", i, err)
			return
		}
	}
}

func TestMVTLayer(t *testing.T) {
	if os.Getenv("RUN_POSTGIS_TESTS") != "yes" {
		return
	}

	port, err := strconv.ParseInt(os.Getenv("PGPORT"), 10, 64)
	if err != nil {
		t.Fatalf("err parsing PGPORT: %v", err)
	}

	testcases := []struct {
		config               map[string]interface{}
		tile                 *slippy.Tile
		expectedFeatureCount int
	}{
		{
			config: map[string]interface{}{
				postgis.ConfigKeyHost:     os.Getenv("PGHOST"),
				postgis.ConfigKeyPort:     port,
				postgis.ConfigKeyDB:       os.Getenv("PGDATABASE"),
				postgis.ConfigKeyUser:     os.Getenv("PGUSER"),
				postgis.ConfigKeyPassword: os.Getenv("PGPASSWORD"),
				postgis.ConfigKeyLayers: []map[string]interface{}{
					{
						postgis.ConfigKeyLayerName: "land",
						postgis.ConfigKeyTablename: "ne_10m_land_scale_rank",
					},
				},
			},
			tile:                 slippy.NewTile(1, 1, 1, 64, tegola.WebMercator),
			expectedFeatureCount: 4032,
		},
		//	scalerank test
		{
			config: map[string]interface{}{
				postgis.ConfigKeyHost:     os.Getenv("PGHOST"),
				postgis.ConfigKeyPort:     port,
				postgis.ConfigKeyDB:       os.Getenv("PGDATABASE"),
				postgis.ConfigKeyUser:     os.Getenv("PGUSER"),
				postgis.ConfigKeyPassword: os.Getenv("PGPASSWORD"),
				postgis.ConfigKeyLayers: []map[string]interface{}{
					{
						postgis.ConfigKeyLayerName: "land",
						postgis.ConfigKeySQL:       "SELECT gid, ST_AsBinary(geom) AS geom FROM ne_10m_land_scale_rank WHERE scalerank=!ZOOM! AND geom && !BBOX!",
					},
				},
			},
			tile:                 slippy.NewTile(1, 1, 1, 64, tegola.WebMercator),
			expectedFeatureCount: 98,
		},
		//	decode numeric(x,x) types
		{
			config: map[string]interface{}{
				postgis.ConfigKeyHost:     os.Getenv("PGHOST"),
				postgis.ConfigKeyPort:     port,
				postgis.ConfigKeyDB:       os.Getenv("PGDATABASE"),
				postgis.ConfigKeyUser:     os.Getenv("PGUSER"),
				postgis.ConfigKeyPassword: os.Getenv("PGPASSWORD"),
				postgis.ConfigKeyLayers: []map[string]interface{}{
					{
						postgis.ConfigKeyLayerName:   "buildings",
						postgis.ConfigKeyGeomIDField: "osm_id",
						postgis.ConfigKeyGeomField:   "geometry",
						postgis.ConfigKeySQL:         "SELECT ST_AsBinary(geometry) AS geometry, osm_id, name, nullif(as_numeric(height),-1) AS height, type FROM osm_buildings_test WHERE geometry && !BBOX!",
					},
				},
			},
			tile:                 slippy.NewTile(16, 11241, 26168, 64, tegola.WebMercator),
			expectedFeatureCount: 101,
		},
	}

	for i, tc := range testcases {
		p, err := postgis.NewTileProvider(tc.config)
		if err != nil {
			t.Errorf("[%v] unexpected error; unable to create a new provider, Expected: nil Got %v", i, err)
			continue
		}

		//	iterate our configured layers
		for _, tcLayer := range tc.config[postgis.ConfigKeyLayers].([]map[string]interface{}) {
			layerName := tcLayer[postgis.ConfigKeyLayerName].(string)

			var featureCount int
			err := p.TileFeatures(context.Background(), layerName, tc.tile, func(f *provider.Feature) error {
				featureCount++

				return nil
			})
			if err != nil {
				t.Errorf("[%v] unexpected error; failed to create mvt layer, Expected nil Got %v", i, err)
				continue
			}

			if featureCount != tc.expectedFeatureCount {
				t.Errorf("[%v] feature count, Expected %v Got %v", i, tc.expectedFeatureCount, featureCount)
			}
		}
	}
}
