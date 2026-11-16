package trimmed

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func projectiveFootprintRoof(t *testing.T, left, right, offset, slope float64) Face {
	t.Helper()
	f := projectiveContourImprintFace(t, false, false, false)
	var poles []geom.Vec3
	for _, x := range [2]float64{left, right} {
		for v := range 2 {
			y := float64(v)
			poles = append(poles, geom.Vec3{X: x, Y: y, Z: 1 + x*y + offset + slope*x})
		}
	}
	k := []float64{0, 0, 1, 1}
	raw, err := geom.NewBSplineSurface(1, 1, 2, 2, poles, k, k, nil)
	if err != nil {
		t.Fatal(err)
	}
	f.Surface = raw
	return f
}

func TestBuildProjectiveSplineRoofOverlapCommonClipsOriginalOwnership(t *testing.T) {
	ctx := context.Background()
	a := projectiveFootprintRoof(t, 0, 1, 0, 0)
	for _, axes := range [][2]int{{0, 1}, {1, 0}} {
		for _, tc := range []struct {
			b                          Face
			vol                        float64
			sections, walls, generated int
		}{
			{projectiveFootprintRoof(t, .5, 1.5, .5, 0), 11.0 / 16, 0, 4, 1},
			{projectiveFootprintRoof(t, .5, 1.5, -.75, 1), 21.0 / 32, 1, 6, 0},
		} {
			r, err := BuildProjectiveSplineRoofOverlapCommon(ctx, a, tc.b, axes, 0, ProjectiveRoofSolidOptions{})
			if err != nil || !r.Complete || r.Empty || len(r.Walls) != tc.walls || len(r.SectionEdges) != tc.sections {
				t.Fatal("rectangular partial overlap and its original lifted material must close", axes, tc.sections, err)
			}
			if r.Solid.faces[r.Roofs[0]].Surface != a.Surface ||
				tc.sections > 0 && r.Solid.faces[r.Roofs[1]].Surface != tc.b.Surface {
				t.Fatal("overlap clipping replaced an original support", axes, tc.sections)
			}
			if q, err := BuildProjectiveSplineRoofBoolean(ctx, a, tc.b, axes, 0, parametric.Intersection, ProjectiveRoofSolidOptions{}); err != nil ||
				!q.Complete || len(q.Walls) != tc.walls || q.Roofs != r.Roofs {
				t.Fatal("the material dispatcher must use the exact original intersection footprint", axes, tc.sections, err)
			}
			generated, clipped := 0, 0
			original := [2]Face{a, tc.b}
			for _, wall := range r.Walls {
				ref := wall.Source
				if ref.OriginalUse == -1 {
					if ref.OriginalEdge != -1 {
						t.Fatal("a generated wall cannot borrow an existing coedge ID", wall)
					}
					generated++
				} else {
					if ref.OriginalEdge != original[wall.Owner].Outer[ref.OriginalUse].Edge {
						t.Fatal("original owner edge ID was not restored after clipping", wall)
					}
					if ref.OriginalRange[0].Sign() > 0 || ref.OriginalRange[1].Cmp(big.NewRat(1, 1)) < 0 {
						clipped++
					}
				}
			}
			if generated != tc.generated || clipped != 2+2*tc.sections {
				t.Fatal("clipped wall ownership must have the exact source and subdivision count", axes, tc.sections, generated, clipped)
			}
			p, err := r.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
			if err != nil || math.Abs(p.Volume-tc.vol) > 1e-10 {
				t.Fatal("the overlap solid does not match the independent rectangle integral", axes, tc.sections, p, err)
			}
		}
	}
	for _, left := range []float64{1, 2} {
		b := projectiveFootprintRoof(t, left, left+1, .5, 0)
		r, err := BuildProjectiveSplineRoofOverlapCommon(ctx, a, b, [2]int{0, 1}, 0, ProjectiveRoofSolidOptions{})
		if err != nil || !r.Complete || !r.Empty || len(r.Solid.faces) != 0 {
			t.Fatal("disjoint and edge-only footprints have no regularized volume", r, err)
		}
	}
	wide, other := projectiveFootprintRoof(t, 0, 3, 0, 0), projectiveFootprintRoof(t, 1, 4, .5, 0)
	r, err := BuildProjectiveSplineRoofOverlapCommon(ctx, wide, other, [2]int{0, 1}, 0, ProjectiveRoofSolidOptions{})
	if !errors.Is(err, geom.ErrUnrepresentable) || r.Complete {
		t.Fatal("a nonbinary original clipping station cannot be rounded into native topology", r, err)
	}
}
