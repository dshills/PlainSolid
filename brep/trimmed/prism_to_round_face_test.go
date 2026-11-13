package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math"
	"plainsolid/brep"
	"plainsolid/geom"
	"testing"
)

func TestPrismToActualRoundFace(t *testing.T) {
	ctx := context.Background()
	for _, sign := range []float64{1, -1} {
		frame, _ := geom.FrameFromNormal(geom.Vec3{Z: 2 * sign}, geom.Vec3{Z: sign})
		target, e := ExtrudeDiskWithHoles(ctx, frame, ExtrusionCircle{Radius: 5}, []ExtrusionCircle{{Radius: .5}}, nil, 1)
		if e != nil {
			t.Fatal(e)
		}
		face := -1
		for i, f := range target.faces {
			p, ok := f.Surface.(geom.PlaneSurface)
			if ok && p.Frame().Origin().Z == 2*sign {
				face = i
				break
			}
		}
		if face < 0 {
			t.Fatal("target")
		}
		var buffer bytes.Buffer
		if e = WriteJSON(&buffer, target); e != nil {
			t.Fatal(e)
		}
		target, e = ReadJSON(&buffer, JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		outer := []geom.Vec3{{X: -2, Y: -2}, {X: 2, Y: -2}, {X: 2, Y: 2}, {X: -2, Y: 2}}
		hole := []geom.Vec3{{X: -.5, Y: -.5}, {X: -.5, Y: .5}, {X: .5, Y: .5}, {X: .5, Y: -.5}}
		result, proof, e := PrismToRoundFace(ctx, outer, [][]geom.Vec3{hole}, geom.Vec3{Z: 3 * sign}, target, face, geom.DiskPolygonOptions{})
		if e != nil || proof.Height != 2 || proof.Offset.Z != 2*sign || !proof.Coverage.Touching {
			t.Fatal(proof, e)
		}
		mass, e := result.MassProperties()
		if e != nil || math.Abs(mass.Volume-30) > 1e-10 || math.Abs(mass.Centroid.Z-sign) > 1e-10 {
			t.Fatal(mass, e)
		}
		if _, _, e := PrismToRoundFace(ctx, outer, nil, geom.Vec3{Z: sign}, target, face, geom.DiskPolygonOptions{}); !errors.Is(e, brep.ErrTerminationUncovered) {
			t.Fatal("bore not excluded", e)
		}
		offset := []geom.Vec3{{X: 1, Y: -1}, {X: 2, Y: -1}, {X: 2, Y: 1}, {X: 1, Y: 1}}
		if _, p, e := PrismToRoundFace(ctx, offset, nil, geom.Vec3{Z: sign}, target, face, geom.DiskPolygonOptions{}); e != nil || !p.Coverage.Covered {
			t.Fatal(p, e)
		}
		if _, _, e := PrismToRoundFace(ctx, outer, [][]geom.Vec3{hole}, geom.Vec3{Z: -sign}, target, face, geom.DiskPolygonOptions{}); !errors.Is(e, geom.ErrDomain) {
			t.Fatal(e)
		}
	}
}

func TestPrismToRoundFaceAxisPermutations(t *testing.T) {
	ctx := context.Background()
	for _, axis := range []geom.Vec3{{X: 1}, {X: -1}, {Y: 1}, {Y: -1}, {Z: 1}, {Z: -1}} {
		frame, _ := geom.FrameFromNormal(axis.Scale(2), axis)
		base, _ := geom.FrameFromNormal(geom.Vec3{}, axis)
		target, e := ExtrudeDiskWithHoles(ctx, frame, ExtrusionCircle{Radius: 3}, nil, nil, 1)
		if e != nil {
			t.Fatal(e)
		}
		face := -1
		for i, f := range target.faces {
			p, ok := f.Surface.(geom.PlaneSurface)
			if ok && p.Frame().Origin() == axis.Scale(2) {
				face = i
				break
			}
		}
		var source []geom.Vec3
		for _, uv := range []geom.Vec3{{X: -1, Y: -1}, {X: 1, Y: -1}, {X: 1, Y: 1}, {X: -1, Y: 1}} {
			source = append(source, base.ToWorld(uv))
		}
		result, proof, e := PrismToRoundFace(ctx, source, nil, axis, target, face, geom.DiskPolygonOptions{})
		if e != nil || proof.Offset != axis.Scale(2) {
			t.Fatal(axis, proof, e)
		}
		mass, e := result.MassProperties()
		if e != nil || math.Abs(mass.Volume-8) > 1e-9 || mass.Centroid.Sub(axis).Length() > 1e-9 {
			t.Fatal(mass, e)
		}
		if _, _, e := PrismToRoundFace(ctx, source, nil, axis, target, face, geom.DiskPolygonOptions{MaxOperations: 1}); e == nil {
			t.Fatal("budget")
		}
	}
}
