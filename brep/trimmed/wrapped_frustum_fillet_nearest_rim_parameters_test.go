package trimmed

import (
	"bytes"
	"context"
	"math/big"
	"plainsolid/brep/placement"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestFilletedFrustumNearestRimParameters(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	r := big.NewRat
	w, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, 8, 6, o)
	if e != nil {
		t.Fatal(e)
	}
	w, _, e = geom.DraftWrappedFrustum(ctx, w, 0, 8, o)
	if e != nil {
		t.Fatal(e)
	}
	s, _, e := NewWrappedFrustum(ctx, w, 1e-9, o)
	if e != nil {
		t.Fatal(e)
	}
	q, e := PrepareFilletedFrustumMaterial(ctx, s, 15./16, 15./8, []FilletedFrustumBoreCut{{Center: geom.Vec2{X: -2}, Radius: .5}, {Center: geom.Vec2{X: 2}, Radius: .5, Depth: 1}, {Center: geom.Vec2{X: 2}, Radius: .5, Depth: 4, FromUpper: true}}, o)
	if e != nil {
		t.Fatal(e)
	}
	var encoded bytes.Buffer
	if e = WriteFilletedFrustumMaterialJSON(ctx, &encoded, q); e != nil {
		t.Fatal(e)
	}
	q, e = ReadFilletedFrustumMaterialJSON(ctx, &encoded, FilletedFrustumMaterialJSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	m := [3][4]*big.Rat{{r(3, 13), r(-12, 13), r(4, 13), r(1, 7)}, {r(36, 65), r(5, 13), r(48, 65), r(-2, 9)}, {r(-4, 5), r(0, 1), r(3, 5), r(3, 11)}}
	pose, e := placement.NewRationalRigid(m, 4096)
	if e != nil {
		t.Fatal(e)
	}
	placed, e := q.PlaceRigid(ctx, pose)
	if e != nil {
		t.Fatal(e)
	}
	world := func(p [3]*big.Rat) [3]*big.Rat {
		var v [3]*big.Rat
		for i := range v {
			v[i] = new(big.Rat).Set(m[i][3])
			for j := range p {
				v[i].Add(v[i], fdMul(m[i][j], p[j]))
			}
		}
		return v
	}

	opts := FilletedFrustumRimParameterOptions{}
	for end, c := range q.fillets.Circles {
		for role := 0; role < 2; role++ {
			for _, az := range [][2]*big.Rat{{r(3, 5), r(4, 5)}, {r(-3, 5), r(-4, 5)}, {r(1, 1), r(0, 1)}, {r(0, 1), r(1, 1)}} {
				radius, height := c.Center[0], q.z[end]
				nr, nz := r(0, 1), r(-1, 1)
				if end == 1 {
					nz = r(1, 1)
				}
				if role == 1 {
					radius, height = c.Tangency[0], c.Tangency[1]
					nr, nz = c.Normal[0], c.Normal[1]
				}
				point := [3]*big.Rat{fdMul(radius, az[0]), fdMul(radius, az[1]), height}
				query := [3]*big.Rat{fdAdd(point[0], fdMul(r(1, 64), fdMul(nr, az[0]))), fdAdd(point[1], fdMul(r(1, 64), fdMul(nr, az[1]))), fdAdd(point[2], fdMul(r(1, 64), nz))}
				got, e := q.LocateNearestFilletRimRational(ctx, query, opts)
				if e != nil || !got.Located || got.Baseline.Upper.Cmp(r(1, 4096)) != 0 {
					t.Fatal(end, role, got, e)
				}
				wantCount := 1
				if az[0].Sign() == 0 || az[1].Sign() == 0 {
					wantCount = 2
				}
				if len(got.Contacts) != wantCount {
					t.Fatal("quarter edge multiplicity", got.Contacts)
				}
				for i := range point {
					if got.Baseline.Witness[i].Cmp(point[i]) != 0 {
						t.Fatal("original nearest point")
					}
				}
				for _, hit := range got.Contacts {
					edge := q.solid.edges[hit.Edge]
					tf, _ := hit.Parameter.Float64()
					eval, e := edge.Curve.Eval(tf)
					if e != nil {
						t.Fatal(e)
					}
					coords := [3]float64{eval.X, eval.Y, eval.Z}
					for i, x := range coords {
						want, _ := point[i].Float64()
						if x-want > 1e-13 || want-x > 1e-13 {
							t.Fatal("independent native evaluation")
						}
					}
					if (hit.Vertex >= 0) != (wantCount == 2) {
						t.Fatal("vertex ownership")
					}
				}
				posed, e := placed.LocateNearestFilletRimRational(ctx, world(query), opts)
				if e != nil || !posed.Original.Located || len(posed.Original.Contacts) != wantCount {
					t.Fatal("placed parameters", e)
				}
				for i, hit := range posed.Original.Contacts {
					if hit.Edge != got.Contacts[i].Edge || hit.Vertex != got.Contacts[i].Vertex || hit.Parameter.Cmp(got.Contacts[i].Parameter) != 0 {
						t.Fatal("original parameter changed")
					}
				}
				for i := range point {
					if posed.WorldWitness[i].Cmp(world(point)[i]) != 0 {
						t.Fatal("world exact witness")
					}
				}
				if end == 0 && role == 0 && wantCount == 1 {
					low := opts
					low.MaxOperations = got.ExactOperations - 1
					bad, e := q.LocateNearestFilletRimRational(ctx, query, low)
					if e == nil || bad.Located || len(bad.Contacts) != 0 || bad.Baseline.Lower != nil {
						t.Fatal("aggregate budget")
					}
				}
			}
		}
	}
	unsupported, e := q.LocateNearestFilletRimRational(ctx, [3]*big.Rat{r(8, 1), r(1, 1), r(3, 16)}, opts)
	if e != nil || unsupported.Located || len(unsupported.Contacts) != 0 {
		t.Fatal("no false rational source parameter", e)
	}
}
