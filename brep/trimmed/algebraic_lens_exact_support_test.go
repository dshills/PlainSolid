package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func exactRoofForLensMesh(t *testing.T, original Solid) Solid {
	t.Helper()
	ctx := context.Background()
	parts := make([]SewFace, 6)
	for i, face := range original.Faces() {
		s := face.Surface.(*geom.BSplineSurface)
		var e error
		if i == 1 {
			p, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			data, e := p.ExportExactBezierRecipe()
			if e != nil {
				t.Fatal(e)
			}
			// Exact repeated elevation to degree5. The first interior X
			// control is 1/8+(3/4)/5=11/40, genuinely nonbinary.
			for degree := data.DegreeU; degree < 5; degree++ {
				controls := make([][4]string, (degree+2)*3)
				for u := 0; u < degree+2; u++ {
					for v := 0; v < 3; v++ {
						for k := 0; k < 4; k++ {
							q := new(big.Rat)
							if u > 0 {
								z, _ := new(big.Rat).SetString(data.Controls[(u-1)*3+v][k])
								q.Mul(z, big.NewRat(int64(u), int64(degree+1)))
							}
							if u < degree+1 {
								z, _ := new(big.Rat).SetString(data.Controls[u*3+v][k])
								q.Add(q, z.Mul(z, big.NewRat(int64(degree+1-u), int64(degree+1))))
							}
							controls[u*3+v][k] = q.RatString()
						}
					}
				}
				data.DegreeU = degree + 1
				data.Controls = controls
			}
			p, e = geom.ImportExactBezierSurfaceRecipe(ctx, data, geom.SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			q, e := geom.NewExactBezierSurface(p)
			if e != nil {
				t.Fatal(e)
			}
			parts[i], e = ExactSplineFaceRectangle(ctx, q, geom.ParameterRectangle{}, face.Reversed, geom.SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			continue
		}
		parts[i], e = RectangularSplineFace(s, face.Reversed)
		if e != nil {
			t.Fatal(e)
		}
	}
	b, _, e := Sew(ctx, parts, SewOptions{Tolerance: 1e-8})
	if e != nil {
		t.Fatal(e)
	}
	return b
}

func TestExactLensSupportPermutation(t *testing.T) {
	ctx := context.Background()
	_, stock, _ := transverseSplineCaps(t)
	stock = exactRoofForLensMesh(t, stock)
	original := stock.faces[1].Surface.(*geom.ExactBezierSurface)
	data, e := original.ExactPatch().ExportExactBezierRecipe()
	if e != nil {
		t.Fatal(e)
	}
	nonbinary := false
	for _, h := range data.Controls {
		w, ok := new(big.Rat).SetString(h[3])
		if !ok || w.Sign() <= 0 {
			t.Fatal("weight")
		}
		for _, value := range h[:3] {
			q, ok := new(big.Rat).SetString(value)
			if !ok {
				t.Fatal(value)
			}
			q.Quo(q, w)
			_, exact := q.Float64()
			nonbinary = nonbinary || !exact
		}
	}
	x, _ := new(big.Rat).SetString(data.Controls[3][0])
	w, _ := new(big.Rat).SetString(data.Controls[3][3])
	x.Quo(x, w)
	if x.Cmp(big.NewRat(11, 40)) != 0 {
		t.Fatal("unexpected exact interior X control", x)
	}
	if !nonbinary {
		t.Fatal("fixture Euclidean controls are all binary")
	}
	partition := AlgebraicTubeFacePartition{SourceSurface: original}
	axes := [3]int{1, 2, 0}
	source, patch, e := permuteTubeLensSupport(partition, axes, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if _, ok := source.(*geom.ExactBezierSurface); !ok {
		t.Fatal("exact support replaced by float poles")
	}
	for _, uv := range [][2]*big.Rat{{big.NewRat(1, 3), big.NewRat(2, 5)}, {big.NewRat(3, 8), big.NewRat(7, 9)}} {
		a, e := original.ExactPatch().EvaluateRational(ctx, uv[0], uv[1], geom.SurfacePatchEvaluationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		b, e := patch.EvaluateRational(ctx, uv[0], uv[1], geom.SurfacePatchEvaluationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		for i, j := range axes {
			if a.Point[j].Cmp(b.Point[i]) != 0 || a.Du[j].Cmp(b.Du[i]) != 0 || a.Dv[j].Cmp(b.Dv[i]) != 0 {
				t.Fatal("nonexact homogeneous permutation")
			}
		}
	}
	// The legacy pointer denotes only a genuine float B-spline, while source
	// extraction returns the exact immutable original adapter without conversion.
	if partition.Support != nil || tubeLensPartitionSurface(partition) != original {
		t.Fatal("legacy support semantics")
	}
	legacy := AlgebraicTubeFacePartition{Support: stock.faces[0].Surface.(*geom.BSplineSurface)}
	if tubeLensPartitionSurface(legacy) != legacy.Support {
		t.Fatal("legacy fallback")
	}
}
