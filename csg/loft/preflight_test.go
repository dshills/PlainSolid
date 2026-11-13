package loft

import (
	"context"
	"errors"
	"plainsolid/brep/support"
	"plainsolid/collision"
	"plainsolid/geom"
	"testing"
)

// A failed size gate must occur before geometric construction/triangulation.
// The intentionally malformed ring would otherwise trigger a geometry error.
func TestLoftSizePreflightBeforeGeometry(t *testing.T) {
	a, b := rings()
	b[1].Z++
	for _, tc := range []struct {
		o    Options
		want error
	}{{Options{Support: support.Options{MaxFaces: 5}}, support.ErrLimit}, {Options{Support: support.Options{MaxVertices: 7}}, support.ErrLimit}, {Options{Validation: collision.LocationOptions{MaxTriangles: 11}}, collision.ErrLimit}} {
		if _, e := Planar(context.Background(), a, b, tc.o); !errors.Is(e, tc.want) {
			t.Fatalf("Planar reached geometry before size rejection: %v", e)
		}
	}
}

func TestStationSizePreflightAndDefaultPrecedence(t *testing.T) {
	a, b := rings()
	bad := [][]geom.Vec3{a, b, b}
	for _, build := range []func(context.Context, [][]geom.Vec3, StationOptions) error{
		func(c context.Context, r [][]geom.Vec3, o StationOptions) error {
			_, e := PlanarStations(c, r, o)
			return e
		},
		func(c context.Context, r [][]geom.Vec3, o StationOptions) error {
			_, e := PlanarStationBands(c, r, o)
			return e
		},
		func(c context.Context, r [][]geom.Vec3, o StationOptions) error {
			_, e := MiteredStations(c, r, o)
			return e
		},
	} {
		if e := build(context.Background(), bad, StationOptions{Options: Options{Support: support.Options{MaxFaces: 9}}}); !errors.Is(e, support.ErrLimit) {
			t.Fatal(e)
		}
		if e := build(context.Background(), bad, StationOptions{}); !errors.Is(e, ErrUnsupported) {
			t.Fatal("default geometry error changed", e)
		}
	}
	b[1].Z++
	if _, e := Planar(context.Background(), a, b, Options{}); !errors.Is(e, ErrUnsupported) {
		t.Fatal(e)
	}
}
