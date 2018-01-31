package atlas

import (
	"context"
	"log"
	"strings"
	"sync"

	"github.com/golang/protobuf/proto"

	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/geom"
	"github.com/terranodo/tegola/geom/slippy"
	"github.com/terranodo/tegola/internal/convert"
	"github.com/terranodo/tegola/mvt"
	"github.com/terranodo/tegola/provider"
	"github.com/terranodo/tegola/provider/debug"
)

//	NewMap creates a new map with the necessary default values
func NewWGS84Map(name string) Map {
	return Map{
		Name: name,
		//	default bounds
		Bounds:     tegola.WGS84Bounds,
		Layers:     []Layer{},
		SRID:       tegola.WGS84,
		TileExtent: 4096,
		TileBuffer: 64,
	}
}

type Map struct {
	Name string
	//	Contains an attribution to be displayed when the map is shown to a user.
	// 	This string is sanatized so it can't be abused as a vector for XSS or beacon tracking.
	Attribution string
	//	The maximum extent of available map tiles in WGS:84
	//	latitude and longitude values, in the order left, bottom, right, top.
	//	Default: [-180, -85, 180, 85]
	Bounds [4]float64
	//	The first value is the longitude, the second is latitude (both in
	//	WGS:84 values), the third value is the zoom level.
	Center [3]float64
	Layers []Layer

	SRID uint64
	//	MVT output values
	TileExtent uint64
	TileBuffer uint64
}

// AddDebugLayers returns a copy of a Map with the debug layers appended to the layer list
func (m Map) AddDebugLayers() Map {
	//	make an explict copy of the layers
	layers := make([]Layer, len(m.Layers))
	copy(layers, m.Layers)
	m.Layers = layers

	//	setup a debug provider
	debugProvider, _ := debug.NewTileProvider(map[string]interface{}{})

	m.Layers = append(layers, []Layer{
		{
			Name:              debug.LayerDebugTileOutline,
			ProviderLayerName: debug.LayerDebugTileOutline,
			Provider:          debugProvider,
			GeomType:          geom.LineString{},
			MinZoom:           0,
			MaxZoom:           MaxZoom,
		},
		{
			Name:              debug.LayerDebugTileCenter,
			ProviderLayerName: debug.LayerDebugTileCenter,
			Provider:          debugProvider,
			GeomType:          geom.Point{},
			MinZoom:           0,
			MaxZoom:           MaxZoom,
		},
	}...)

	return m
}

// FilterLayersByZoom returns a copy of a Map with a subset of layers that match the given zoom
func (m Map) FilterLayersByZoom(zoom int) Map {
	var layers []Layer

	for i := range m.Layers {
		if (m.Layers[i].MinZoom <= zoom || m.Layers[i].MinZoom == 0) && (m.Layers[i].MaxZoom >= zoom || m.Layers[i].MaxZoom == 0) {
			layers = append(layers, m.Layers[i])
			continue
		}
	}

	//	overwrite the Map's layers with our subset
	m.Layers = layers

	return m
}

// FilterLayersByName returns a copy of a Map witha subset of layers that match the supplied list of layer names
func (m Map) FilterLayersByName(names ...string) Map {
	var layers []Layer

	nameStr := strings.Join(names, ",")
	for i := range m.Layers {
		// if we have a name set, use it for the lookup
		if m.Layers[i].Name != "" && strings.Contains(nameStr, m.Layers[i].Name) {
			layers = append(layers, m.Layers[i])
			continue
		} else if m.Layers[i].ProviderLayerName != "" && strings.Contains(nameStr, m.Layers[i].ProviderLayerName) { //	default to using the ProviderLayerName for the lookup
			layers = append(layers, m.Layers[i])
			continue
		}
	}

	// overwrite the Map's layers with our subset
	m.Layers = layers

	return m
}

//	TODO (arolek): support for max zoom
func (m Map) Encode(ctx context.Context, tile *slippy.Tile) ([]byte, error) {
	// tile container
	var mvtTile mvt.Tile
	// wait group for concurrent layer fetching
	var wg sync.WaitGroup

	// layer stack
	mvtLayers := make([]*mvt.Layer, len(m.Layers))

	// set our waitgroup count
	wg.Add(len(m.Layers))

	// iterate our layers
	for i, layer := range m.Layers {
		var mvtLayer mvt.Layer

		// go routine for fetching the layer concurrently
		go func(i int, l Layer) {
			// on completion let the wait group know
			defer wg.Done()

			//	fetch layer from data provider
			err := l.Provider.TileFeatures(ctx, l.ProviderLayerName, tile, func(f *provider.Feature) error {
				// TODO: remove this geom conversion step once the mvt package has adopted the new geom package
				geo, err := convert.ToTegola(f.Geometry)
				if err != nil {
					return err
				}

				mvtLayer.AddFeatures(mvt.Feature{
					ID:       &f.ID,
					Tags:     f.Tags,
					Geometry: geo,
				})

				// TODO (arolek): add default tags

				return nil
			})
			if err != nil {
				switch err {
				case context.Canceled:
					// TODO (arolek): add debug logs
				default:
					z, x, y := tile.ZXY()
					// TODO (arolek): should we return an error to the response or just log the error?
					// we can't just write to the response as the waitgroup is going to write to the response as well
					log.Printf("Error Getting MVTLayer for tile Z: %v, X: %v, Y: %v: %v", z, x, y, err)
				}
				return
			}

			// check if we have a layer name
			if l.Name != "" {
				mvtLayer.Name = l.Name
			}

			// add the layer to the slice position
			mvtLayers[i] = &mvtLayer
		}(i, layer)
	}

	// wait for the waitgroup to finish
	wg.Wait()

	// stop processing if the context has an error. this check is necessary
	// otherwise the server continues processing even if the request was canceled
	// as the waitgroup was not notified of the cancel
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	//	add layers to our tile
	mvtTile.AddLayers(mvtLayers...)

	z, x, y := tile.ZXY()

	// TODO (arolek): change out the tile type for VTile. tegola.Tile will be deprecated
	tegolaTile := tegola.NewTile(int(z), int(x), int(y))

	// generate our tile
	vtile, err := mvtTile.VTile(ctx, tegolaTile)
	if err != nil {
		return nil, err
	}

	// encode the tile
	return proto.Marshal(vtile)
}
