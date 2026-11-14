package trimmed

import (
	"bytes"
	"context"
	"math/big"
	"plainsolid/brep/placement"
	"plainsolid/geom"
	"testing"
)

func TestFilletedFrustumNearestBoreRimParameters(t *testing.T) {
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
	for _, c := range q.cuts {
		for end, z := range c.z {
			for _, az := range [][2]*big.Rat{{r(3, 5), r(4, 5)}, {r(-3, 5), r(-4, 5)}, {r(1, 1), r(0, 1)}} {
				radial := r(1, 4)
				want := r(5, 64)
				if !c.open[end] {
					radial = r(1, 1)
					want = r(17, 64)
				}
				height := fdSub(z, r(1, 8))
				if end == 1 {
					height = fdAdd(z, r(1, 8))
				}
				query := [3]*big.Rat{fdAdd(c.center[0], fdMul(radial, az[0])), fdAdd(c.center[1], fdMul(radial, az[1])), height}
				point := [3]*big.Rat{fdAdd(c.center[0], fdMul(c.radius, az[0])), fdAdd(c.center[1], fdMul(c.radius, az[1])), z}
				got, e := q.LocateNearestBoreRimRational(ctx, query, opts)
				if e != nil || !got.Located || got.Baseline.Upper.Cmp(want) != 0 {
					t.Fatal("bore rim", got, e)
				}
				wantCount := 1
				if az[1].Sign() == 0 {
					wantCount = 2
				}
				if len(got.Contacts) != wantCount {
					t.Fatal("edge multiplicity")
				}
				other := c.floor
				if c.open[end] {
					other = q.fillets.Caps[end]
				}
				owners := map[int]bool{}
				for _, id := range got.Faces {
					owners[id] = true
				}
				if len(owners) != 2 || !owners[c.wall] || !owners[other] {
					t.Fatal("original owner pair")
				}
				for i := range point {
					if got.Baseline.Witness[i].Cmp(point[i]) != 0 {
						t.Fatal("nearest point")
					}
				}
				for _, hit := range got.Contacts {
					edge := q.solid.edges[hit.Edge]
					tf, _ := hit.Parameter.Float64()
					v, e := edge.Curve.Eval(tf)
					if e != nil {
						t.Fatal(e)
					}
					for i, x := range [3]float64{v.X, v.Y, v.Z} {
						want, _ := point[i].Float64()
						if x-want > 1e-13 || want-x > 1e-13 {
							t.Fatal("original native curve evaluation")
						}
					}
					if (hit.Vertex >= 0) != (wantCount == 2) {
						t.Fatal("original quarter vertex")
					}
				}
				posed, e := placed.LocateNearestBoreRimRational(ctx, world(query), opts)
				if e != nil || !posed.Original.Located || len(posed.Original.Contacts) != wantCount {
					t.Fatal("placed", e)
				}
				for i, hit := range posed.Original.Contacts {
					if hit.Edge != got.Contacts[i].Edge || hit.Vertex != got.Contacts[i].Vertex || hit.Parameter.Cmp(got.Contacts[i].Parameter) != 0 {
						t.Fatal("world parameter preservation")
					}
				}
				for i := range point {
					if posed.WorldWitness[i].Cmp(world(point)[i]) != 0 {
						t.Fatal("world point")
					}
				}
				low := opts
				low.MaxOperations = got.ExactOperations - 1
				bad, e := q.LocateNearestBoreRimRational(ctx, query, low)
				if e == nil || bad.Located || len(bad.Contacts) != 0 || bad.Baseline.Lower != nil {
					t.Fatal("aggregate inversion budget")
				}
			}
		}
	}
	// Irrational rim witness has no rational point/preimage contract here.
	no, e := q.LocateNearestBoreRimRational(ctx, [3]*big.Rat{r(-19, 10), r(1, 10), r(-1, 8)}, opts)
	if e != nil || no.Located || len(no.Contacts) != 0 {
		t.Fatal("no fabricated rational parameter", e)
	}
	// Interior wall has one owner and no rim-localization claim.
	no, e = q.LocateNearestBoreRimRational(ctx, [3]*big.Rat{r(-7, 5), r(0, 1), r(3, 1)}, opts)
	if e != nil || no.Located {
		t.Fatal("wall interior is not rim", e)
	}
}
