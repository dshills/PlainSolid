package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math"
	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

func TestCylinderToPolygonFaceAxes(t *testing.T) {
	ctx := context.Background()
	stock, e := brep.Box(geom.Vec3{X: -2, Y: -2, Z: -2}, geom.Vec3{X: 2, Y: 2, Z: 2})
	if e != nil {
		t.Fatal(e)
	}
	coordinate := func(p geom.Vec3, axis int) float64 { return [3]float64{p.X, p.Y, p.Z}[axis] }
	for axis := 0; axis < 3; axis++ {
		for _, sign := range []float64{-1, 1} {
			faceIndex := -1
			for i, f := range stock.Faces() {
				all := true
				for _, use := range f.Boundary {
					edge := stock.Edges()[use.Edge]
					if coordinate(stock.Vertices()[edge.Start], axis) != 2*sign || coordinate(stock.Vertices()[edge.End], axis) != 2*sign {
						all = false
					}
				}
				if all {
					faceIndex = i
					break
				}
			}
			if faceIndex < 0 {
				t.Fatal("face")
			}
			normal := geom.Vec3{}
			switch axis {
			case 0:
				normal.X = sign * .125
			case 1:
				normal.Y = sign * .125
			case 2:
				normal.Z = sign * .125
			}
			solid, proof, e := CylinderToFace(ctx, geom.Vec3{}, normal, 1, stock, faceIndex, geom.DiskPolygonOptions{})
			if e != nil || proof.Height != 2 || coordinate(proof.Endpoint, axis) != 2*sign || proof.Coverage.Relation != geom.DiskPolygonStrictlyCovered {
				t.Fatal(proof, e)
			}
			mass, e := solid.MassProperties(ctx, parametric.IntegrationOptions{})
			if e != nil || math.Abs(mass.Volume-2*math.Pi) > 1e-8 || math.Abs(coordinate(mass.Centroid, axis)-sign) > 1e-8 {
				t.Fatal(mass, e)
			}
			var buffer bytes.Buffer
			if e := WriteJSON(&buffer, solid); e != nil {
				t.Fatal(e)
			}
			reloaded, e := ReadJSON(&buffer, JSONLimits{})
			if e != nil {
				t.Fatal(e)
			}
			again, e := reloaded.MassProperties(ctx, parametric.IntegrationOptions{})
			if e != nil || math.Abs(again.Volume-mass.Volume) > 1e-9 {
				t.Fatal(again, e)
			}
			if _, p, e := CylinderToFace(ctx, geom.Vec3{}, normal, 2, stock, faceIndex, geom.DiskPolygonOptions{}); e != nil || p.Coverage.Relation != geom.DiskPolygonCoveredTouching {
				t.Fatal(p, e)
			}
			if _, _, e := CylinderToFace(ctx, geom.Vec3{}, normal, math.Nextafter(2, 3), stock, faceIndex, geom.DiskPolygonOptions{}); !errors.Is(e, brep.ErrTerminationUncovered) {
				t.Fatal(e)
			}
			if _, _, e := CylinderToFace(ctx, geom.Vec3{}, normal.Scale(-1), 1, stock, faceIndex, geom.DiskPolygonOptions{}); !errors.Is(e, geom.ErrDomain) {
				t.Fatal(e)
			}
		}
	}
}

func TestCylinderToPolygonFaceHoleAndLimits(t *testing.T) {
	ctx := context.Background()
	outer := []geom.Vec3{{X: -3, Y: -3, Z: 2}, {X: 3, Y: -3, Z: 2}, {X: 3, Y: 3, Z: 2}, {X: -3, Y: 3, Z: 2}}
	hole := []geom.Vec3{{X: -.5, Y: -.5, Z: 2}, {X: -.5, Y: .5, Z: 2}, {X: .5, Y: .5, Z: 2}, {X: .5, Y: -.5, Z: 2}}
	stock, e := brep.PrismWithHoles(outer, [][]geom.Vec3{hole}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	face := -1
	vertices, edges := stock.Vertices(), stock.Edges()
	for i, f := range stock.Faces() {
		all := true
		for _, use := range f.Boundary {
			if vertices[edges[use.Edge].Start].Z != 2 || vertices[edges[use.Edge].End].Z != 2 {
				all = false
			}
		}
		if all {
			face = i
			break
		}
	}
	if face < 0 {
		t.Fatal("cap")
	}
	if _, _, e := CylinderToFace(ctx, geom.Vec3{}, geom.Vec3{Z: 1}, .25, stock, face, geom.DiskPolygonOptions{}); !errors.Is(e, brep.ErrTerminationUncovered) {
		t.Fatal("hole accepted", e)
	}
	if _, p, e := CylinderToFace(ctx, geom.Vec3{X: 1}, geom.Vec3{Z: 1}, .5, stock, face, geom.DiskPolygonOptions{}); e != nil || p.Coverage.Relation != geom.DiskPolygonCoveredTouching {
		t.Fatal(p, e)
	}
	if _, _, e := CylinderToFace(ctx, geom.Vec3{X: 1, Z: -math.SmallestNonzeroFloat64}, geom.Vec3{Z: 1}, .25, stock, face, geom.DiskPolygonOptions{}); !errors.Is(e, geom.ErrUnrepresentable) {
		t.Fatal("rounded height accepted", e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, _, e := CylinderToFace(cancelled, geom.Vec3{X: 1}, geom.Vec3{Z: 1}, .25, stock, face, geom.DiskPolygonOptions{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
