//go:build research

package trimmed

import (
	"context"
	"errors"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestImprintProjectiveSplineShellFacePairsRetainsTwoSeparateSections(t *testing.T) {
	ctx := context.Background()
	a, b := projectiveImprintShellFixture(t, true, false), projectiveImprintShellFixture(t, false, true)
	axes := [2]int{0, 1}
	pairs := []ProjectiveShellFacePair{{Faces: [2]int{5, 5}, Axes: axes}, {Faces: [2]int{4, 4}, Axes: axes}}
	r, err := a.ImprintProjectiveSplineShellFacePairs(ctx, b, pairs, ProjectiveShellImprintOptions{})
	if err != nil || !r.Complete || len(r.Sections) != 2 {
		t.Fatal("both original face pairs must be imprinted in one closed rebuild", r, err)
	}
	for i, section := range r.Sections {
		p, err := section.Model.ExactPoint(.5)
		if err != nil || p != (geom.Vec3{X: .5, Y: .5, Z: 1.25 - float64(i)}) {
			t.Fatal("section order or independent original model position changed", i, p, err)
		}
	}
	for i, v := range r.Owners {
		if len(v.Solid.Faces()) != 8 || len(v.Solid.Edges()) != 18 || len(v.Solid.Vertices()) != 12 ||
			v.InsertedBoundaries != 4 || len(v.SectionEdges) != 2 || v.SectionEdges[0] == v.SectionEdges[1] ||
			len(v.SourceFaces[4]) != 2 || len(v.SourceFaces[5]) != 2 || v.SectionEdge != -1 {
			t.Fatal("multi-pair imprint changed expected independent closed topology", i, v)
		}
		for j, id := range v.SectionEdges {
			rangeCheck := v.Solid.edges[id]
			p, err := rangeCheck.Curve.Eval(.5)
			if err != nil || p != (geom.Vec3{X: .5, Y: .5, Z: 1.25 - float64(j)}) {
				t.Fatal("final seam IDs must match the input pair order", i, j, p, err)
			}
		}
		mp, err := v.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
		if err != nil || math.Abs(mp.Volume-1) > 1e-10 {
			t.Fatal("multi-face imprint changed material volume", i, mp, err)
		}
	}
	duplicate := []ProjectiveShellFacePair{pairs[0], {Faces: [2]int{5, 4}, Axes: axes}}
	result, err := a.ImprintProjectiveSplineShellFacePairs(ctx, b, duplicate, ProjectiveShellImprintOptions{})
	if !errors.Is(err, geom.ErrInvalid) || result.Complete || len(result.Sections) != 0 {
		t.Fatal("two section pairs cannot silently replace the same original face", result, err)
	}
	bad := []ProjectiveShellFacePair{pairs[0], {Faces: [2]int{4, 4}, Axes: [2]int{1, 1}}}
	result, err = a.ImprintProjectiveSplineShellFacePairs(ctx, b, bad, ProjectiveShellImprintOptions{})
	if err == nil || result.Complete || len(result.Sections) != 0 || len(result.Owners[0].Solid.faces) != 0 {
		t.Fatal("an unresolved later pair must not publish an earlier section or shell", result, err)
	}
}
