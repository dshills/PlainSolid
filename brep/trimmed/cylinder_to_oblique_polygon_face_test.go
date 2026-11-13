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

func TestCylinderToFiniteObliqueFace(t *testing.T) {
	ctx := context.Background()
	stock := func(slope, base float64) (brep.Solid, int) {
		points := []geom.Vec3{{X: -2, Y: -2, Z: base - 2*slope}, {X: 2, Y: -2, Z: base + 2*slope}, {X: 2, Y: 2, Z: base + 2*slope}, {X: -2, Y: 2, Z: base - 2*slope}}
		s, e := brep.Prism(points, geom.Vec3{Z: 1})
		if e != nil {
			t.Fatal(e)
		}
		face := -1
		vs, es := s.Vertices(), s.Edges()
		for i, f := range s.Faces() {
			all := true
			for _, u := range f.Boundary {
				p := vs[es[u.Edge].Start]
				q := vs[es[u.Edge].End]
				if p.Z != base+slope*p.X || q.Z != base+slope*q.X {
					all = false
				}
			}
			if all {
				face = i
				break
			}
		}
		return s, face
	}
	target, face := stock(1, 3)
	solid, proof, e := CylinderToObliqueFace(ctx, geom.Vec3{}, geom.Vec3{Z: 2}, .5, target, face, 1e-9, geom.DiskPolygonOptions{})
	if e != nil || proof.RimHeightError > 1e-9 || proof.Coverage.Relation != geom.DiskPolygonStrictlyCovered {
		t.Fatal(proof, e)
	}
	mass, e := solid.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil || math.Abs(mass.Volume-3*math.Pi*.25) > 1e-8 || math.Abs(mass.Centroid.X-.25/12) > 1e-8 || math.Abs(mass.Centroid.Z-(1.5+.25/24)) > 1e-8 {
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
	if _, _, e := CylinderToObliqueFace(ctx, geom.Vec3{}, geom.Vec3{Z: 1}, 2.1, target, face, 1e-9, geom.DiskPolygonOptions{}); !errors.Is(e, brep.ErrTerminationUncovered) {
		t.Fatal("finite coverage", e)
	}
	nonrepresentable, face := stock(.75, 3)
	if _, _, e := CylinderToObliqueFace(ctx, geom.Vec3{}, geom.Vec3{Z: 1}, .5, nonrepresentable, face, 1e-9, geom.DiskPolygonOptions{}); !errors.Is(e, geom.ErrUnrepresentable) {
		t.Fatal("changed normalized plane accepted", e)
	}
	rim, face := stock(1, .25)
	if _, _, e := CylinderToObliqueFace(ctx, geom.Vec3{}, geom.Vec3{Z: 1}, .5, rim, face, 1e-9, geom.DiskPolygonOptions{}); !errors.Is(e, geom.ErrDomain) {
		t.Fatal("nonforward generator", e)
	}
}
