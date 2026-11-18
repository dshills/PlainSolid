package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math/big"
	"plainsolid/internal/slowtest"
	"slices"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestNativeWrappedFrustumOpenBoreReloadAndQueries(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	opts := geom.SurfacePatchOptions{}
	base, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, 2, 4, opts)
	if e != nil {
		t.Fatal(e)
	}
	stock, _, e := geom.DraftWrappedFrustum(ctx, base, 0, 3, opts)
	if e != nil {
		t.Fatal(e)
	}
	tool, _, e := geom.InsetWrappedCircularFrustumAtCap(ctx, stock, .5, true, opts)
	if e != nil {
		t.Fatal(e)
	}
	m := geom.Identity()
	m[0][3], m[1][3] = .25, .25
	tool, _, e = tool.Transformed(ctx, m, geom.WrappedSurfaceOptions{WrapU: true})
	if e != nil {
		t.Fatal(e)
	}
	body, _, e := NewWrappedEccentricCircularFrustumBore(ctx, stock, tool, 1e-8, opts)
	if e != nil {
		t.Fatal(e)
	}
	faces := body.Faces()
	slices.Reverse(faces)
	body, e = NewSolidContext(ctx, body.Vertices(), body.Edges(), faces, 1e-8)
	if e != nil {
		t.Fatal(e)
	}
	var recipe bytes.Buffer
	if e = WriteJSON(&recipe, body); e != nil {
		t.Fatal(e)
	}
	back, e := ReadJSONContext(ctx, &recipe, JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	p, e := back.ValidateCanonicalWrappedFrustumBore(ctx, opts)
	if e != nil || !p.Complete || p.Kind != "blind-bore" || p.OuterFaces != [3]int{4, 3, 2} || p.InnerFaces != [3]int{1, 0, 2} {
		t.Fatal("exact owner identity after native reload", p, e)
	}
	if where, e := back.LocateCanonicalWrappedFrustumBore(ctx, geom.Vec3{X: .25, Y: .25, Z: 4}, opts); e != nil || where != parametric.Outside {
		t.Fatal("native opening classified as a disk", where, e)
	}
	h, e := back.IntersectCanonicalWrappedFrustumOpenBoreLine(ctx, geom.Vec3{X: .25, Y: .25, Z: -1}, geom.Vec3{Z: 1},
		0, 6, geom.ToroidLineOptions{ParameterTolerance: 1e-8})
	if e != nil || len(h.Hits) != 2 || !slices.Equal(h.Hits[0].Faces, []int{3}) || !slices.Equal(h.Hits[1].Faces, []int{0}) {
		t.Fatal("reordered native line face IDs", h, e)
	}
	floor, e := back.SectionCanonicalWrappedFrustumBore(ctx, .5, opts)
	if e != nil || !floor.MaterialPresent || floor.HasHole || floor.InnerFace != 1 || floor.CapFace != 0 || floor.Inner.Curve == nil {
		t.Fatal("retained floor closes the inner rim at its exact height", floor, e)
	}
	mouth, e := back.SectionCanonicalWrappedFrustumBore(ctx, 4, opts)
	if e != nil || !mouth.HasHole || mouth.CapFace != 2 || mouth.OuterFace != 4 || mouth.InnerFace != 1 {
		t.Fatal("native annular cap ownership", mouth, e)
	}
	station, e := back.SectionCanonicalWrappedFrustumBore(ctx, 2, opts)
	if e != nil || !station.HasHole || station.CapFace != -1 || station.Inner.ParameterV.Cmp(big.NewRat(3, 7)) != 0 ||
		station.Inner.Radius.Cmp(big.NewRat(23, 8)) != 0 {
		t.Fatal("exact nonbinary original chart station", station, e)
	}
	height := big.NewRat(7, 3)
	rat, e := back.SectionCanonicalWrappedFrustumBoreRational(ctx, height, opts)
	if e != nil || !rat.HasHole || rat.Outer.ParameterV.Cmp(big.NewRat(7, 12)) != 0 ||
		rat.Inner.ParameterV.Cmp(big.NewRat(11, 21)) != 0 || rat.Outer.Radius.Cmp(big.NewRat(15, 4)) != 0 ||
		rat.Inner.Radius.Cmp(big.NewRat(25, 8)) != 0 || rat.AreaPi.Cmp(big.NewRat(275, 64)) != 0 {
		t.Fatal("exact nonbinary height and annular material area", rat, e)
	}
	height.SetInt64(0)
	if rat.Height.Cmp(big.NewRat(7, 3)) != 0 || rat.Outer.Height.Cmp(rat.Height) != 0 || floor.AreaPi.Cmp(big.NewRat(361, 64)) != 0 {
		t.Fatal("section did not retain its exact height or filled floor", rat.Height, floor.AreaPi)
	}
	epsilon := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 120))
	for _, row := range []struct {
		x    *big.Rat
		want parametric.Location
	}{{new(big.Rat).Sub(big.NewRat(27, 8), epsilon), parametric.Outside},
		{big.NewRat(27, 8), parametric.Boundary},
		{new(big.Rat).Add(big.NewRat(27, 8), epsilon), parametric.Inside}} {
		q := [3]*big.Rat{row.x, big.NewRat(1, 4), big.NewRat(7, 3)}
		where, e := back.LocateCanonicalWrappedFrustumBoreRational(ctx, q, opts)
		if e != nil || where != row.want {
			t.Fatal("exact rational probes at one original wall", q, where, row.want, e)
		}
	}
	if beyond, e := back.SectionCanonicalWrappedFrustumBore(ctx, 5, opts); e != nil || beyond.MaterialPresent || beyond.OuterFace != -1 {
		t.Fatal("outside section must be empty", beyond, e)
	}
	display, e := back.TessellateCanonicalWrappedFrustumOpenBoreCertified(ctx, CertifiedWrappedFrustumMeshOptions{
		Patch: CertifiedPatchMeshOptions{Deflection: .15},
	})
	if e != nil || !display.Collision.Complete || len(display.Collision.Contacts) != 0 || display.OriginalFace[0] != 4 {
		t.Fatal("certified reload display ancestry", e)
	}
	if _, e := back.ValidateCanonicalWrappedBore(ctx, opts); !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("cylinder validator accepted a tapered bore", e)
	}
	damaged := back
	damaged.faces = back.Faces()
	damaged.faces[2].Holes = nil
	if p, e := damaged.ValidateCanonicalWrappedFrustumBore(ctx, opts); e == nil || p.Complete {
		t.Fatal("a changed opening was accepted", p, e)
	}
}

func TestNativeWrappedFrustumBoreLineThroughAndCavity(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	opts := geom.SurfacePatchOptions{}
	base, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, 2, 4, opts)
	if e != nil {
		t.Fatal(e)
	}
	stock, _, e := geom.DraftWrappedFrustum(ctx, base, 0, 3, opts)
	if e != nil {
		t.Fatal(e)
	}
	lineOptions := geom.ToroidLineOptions{ParameterTolerance: 1e-9}
	checkRoot := func(t *testing.T, r geom.ToroidLineRoot, n, d int64, exact bool) {
		t.Helper()
		q := big.NewRat(n, d)
		if r.Min == nil || r.Max == nil || r.Min.Cmp(q) > 0 || r.Max.Cmp(q) < 0 ||
			new(big.Rat).Sub(r.Max, r.Min).Cmp(ratInset(lineOptions.ParameterTolerance)) > 0 ||
			(exact && r.Min.Cmp(r.Max) != 0) {
			t.Fatal("invalid certified line enclosure", r, q)
		}
	}
	for _, tc := range []struct {
		name           string
		bottom, height float64
	}{
		{"through", 0, 4},
		{"cavity", .5, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tool, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{X: .25, Y: .25, Z: tc.bottom}, 1, tc.height, opts)
			if e != nil {
				t.Fatal(e)
			}
			body, saved, e := NewWrappedEccentricCircularFrustumBore(ctx, stock, tool, 1e-8, opts)
			if e != nil {
				t.Fatal(e)
			}
			faces := body.Faces()
			slices.Reverse(faces)
			body, e = NewSolidContext(ctx, body.Vertices(), body.Edges(), faces, 1e-8)
			if e != nil {
				t.Fatal(e)
			}
			id := func(i int) int { return len(faces) - 1 - i }
			axis, d := geom.Vec3{X: .25, Y: .25, Z: -1}, geom.Vec3{Z: 1}
			q, e := body.IntersectCanonicalWrappedFrustumBoreLine(ctx, axis, d, 0, 6, lineOptions)
			if e != nil || len(q.Overlaps) != 0 {
				t.Fatal("native axial query", q, e)
			}
			if _, e = body.IntersectCanonicalWrappedFrustumOpenBoreLine(ctx, axis, d, 0, 6, lineOptions); !errors.Is(e, geom.ErrConvergence) {
				t.Fatal("blind-only API must preserve its original contract", e)
			}
			if tc.name == "cavity" {
				if len(q.Hits) != 4 {
					t.Fatal("the cavity keeps both inner disks", q)
				}
				wantIDs := [4]int{saved.OuterFaces[1], saved.InnerFaces[1], saved.InnerFaces[2], saved.OuterFaces[2]}
				wantT := [4][2]int64{{1, 1}, {3, 2}, {9, 2}, {5, 1}}
				for i, h := range q.Hits {
					checkRoot(t, h.Parameter, wantT[i][0], wantT[i][1], true)
					if !slices.Equal(h.Faces, []int{id(wantIDs[i])}) {
						t.Fatal("reindexed retained cap", i, h.Faces)
					}
				}
				return
			}
			if len(q.Hits) != 0 {
				t.Fatal("the open axial line must miss both end planes", q)
			}
			for i, z := range []float64{0, 4} {
				q, e = body.IntersectCanonicalWrappedFrustumBoreLine(ctx, geom.Vec3{X: -7, Y: .25, Z: z}, geom.Vec3{X: 1}, 0, 14, lineOptions)
				if e != nil || len(q.Hits) != 4 || len(q.Overlaps) != 2 {
					t.Fatal("annular query on either end", z, q, e)
				}
				checkRoot(t, q.Hits[1].Parameter, 25, 4, false)
				checkRoot(t, q.Hits[2].Parameter, 33, 4, false)
				capID := id(saved.OuterFaces[i+1])
				for j, h := range q.Hits {
					wallID := id(saved.OuterFaces[0])
					if j == 1 || j == 2 {
						wallID = id(saved.InnerFaces[0])
					}
					owners := []int{capID, wallID}
					slices.Sort(owners)
					if !slices.Equal(h.Faces, owners) {
						t.Fatal("annular rim must retain both native owners", z, h, owners)
					}
				}
				for _, span := range q.Overlaps {
					if span.Face != capID {
						t.Fatal("annular overlap must retain its native cap", z, span)
					}
				}
			}
		})
	}
}

func TestNativeWrappedFrustumBoreRationalLineNearTangent(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	patchOptions := geom.SurfacePatchOptions{}
	stock, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, 2, 4, patchOptions)
	if e != nil {
		t.Fatal(e)
	}
	for _, cavity := range []bool{false, true} {
		name, z, h := "through", 0.0, 4.0
		if cavity {
			name, z, h = "cavity", .5, 3
		}
		t.Run(name, func(t *testing.T) {
			tool, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{X: .25, Y: .25, Z: z}, 1, h, patchOptions)
			if e != nil {
				t.Fatal(e)
			}
			body, _, e := NewWrappedEccentricCircularFrustumBore(ctx, stock, tool, 1e-8, patchOptions)
			if e != nil {
				t.Fatal(e)
			}
			var saved bytes.Buffer
			if e = WriteJSON(&saved, body); e != nil {
				t.Fatal(e)
			}
			body, e = ReadJSONContext(ctx, &saved, JSONLimits{})
			if e != nil {
				t.Fatal(e)
			}
			faces := body.Faces()
			slices.Reverse(faces)
			body, e = NewSolidContext(ctx, body.Vertices(), body.Edges(), faces, 1e-8)
			if e != nil {
				t.Fatal(e)
			}
			proof, e := body.ValidateCanonicalWrappedFrustumBore(ctx, patchOptions)
			if e != nil {
				t.Fatal(e)
			}
			o := geom.ToroidLineOptions{ParameterTolerance: 1e-23}
			zero := new(big.Rat)
			if cavity {
				p := [3]*big.Rat{big.NewRat(1, 3), big.NewRat(1, 5), big.NewRat(-1, 3)}
				d := [3]*big.Rat{zero, zero, big.NewRat(2, 3)}
				r, e := body.IntersectCanonicalWrappedFrustumBoreLineRational(ctx, p, d, big.NewRat(1, 7), big.NewRat(7, 1), o)
				if e != nil || len(r.Hits) != 4 || len(r.Overlaps) != 0 {
					t.Fatal("four original disk contacts expected", r, e)
				}
				want := [4]*big.Rat{big.NewRat(1, 2), big.NewRat(5, 4), big.NewRat(23, 4), big.NewRat(13, 2)}
				owners := [4]int{proof.OuterFaces[1], proof.InnerFaces[1], proof.InnerFaces[2], proof.OuterFaces[2]}
				for i, hit := range r.Hits {
					if hit.Parameter.Min.Cmp(want[i]) != 0 || hit.Parameter.Max.Cmp(want[i]) != 0 || !slices.Equal(hit.Faces, []int{owners[i]}) {
						t.Fatal("nonbinary retained cap contact", i, hit, want[i])
					}
				}
				return
			}
			middle := big.NewRat(55, 36)
			lo, hi := new(big.Rat).Sub(middle, big.NewRat(1, 7)), new(big.Rat).Add(middle, big.NewRat(1, 9))
			owners := []int{proof.OuterFaces[2], proof.InnerFaces[0]}
			slices.Sort(owners)
			d := [3]*big.Rat{big.NewRat(3, 5), zero, zero}
			for sign := -1; sign <= 1; sign++ {
				dy := new(big.Rat).SetFrac(big.NewInt(int64(sign)), new(big.Int).Lsh(big.NewInt(1), 120))
				p := [3]*big.Rat{big.NewRat(-2, 3), new(big.Rat).Add(big.NewRat(5, 4), dy), big.NewRat(4, 1)}
				r, e := body.IntersectCanonicalWrappedFrustumBoreLineRational(ctx, p, d, lo, hi, o)
				spans := 1
				if sign < 0 {
					spans++
				}
				if e != nil || len(r.Hits) != 1-sign || len(r.Overlaps) != spans {
					t.Fatal("sub-ULP annular tangent classification", sign, r, e)
				}
				for _, hit := range r.Hits {
					if !slices.Equal(hit.Faces, owners) {
						t.Fatal("original reindexed annular rim owners lost", hit.Faces, owners)
					}
				}
				if sign == 0 && (r.Hits[0].Parameter.Multiplicity != 2 || r.Hits[0].Parameter.Min.Cmp(middle) != 0 || r.Hits[0].Parameter.Max.Cmp(middle) != 0) {
					t.Fatal("the exact rational tangent was rounded", r)
				}
				if sign < 0 && (r.Hits[0].Parameter.Max.Cmp(middle) >= 0 || r.Hits[1].Parameter.Min.Cmp(middle) <= 0) {
					t.Fatal("the two inner rims must separate across the tangent", r)
				}
				for _, overlap := range r.Overlaps {
					if overlap.Face != proof.OuterFaces[2] {
						t.Fatal("incorrect original annular overlap owner", overlap)
					}
				}
				limited := o
				limited.Surface.MaxCoefficientBits = 64
				if sign != 0 {
					if r, e := body.IntersectCanonicalWrappedFrustumBoreLineRational(ctx, p, d, lo, hi, limited); !errors.Is(e, geom.ErrConvergence) || len(r.Hits)+len(r.Overlaps) != 0 {
						t.Fatal("excess input precision must be rejected atomically", r, e)
					}
				}
			}
			// A shallow oblique line hits the outer lower rim, then leaves
			// the outer wall only ~2^-118 later. These are separate events
			// even with a requested root width larger than their separation.
			eps := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 120))
			p := [3]*big.Rat{big.NewRat(2, 1), zero, zero}
			d = [3]*big.Rat{new(big.Rat).Neg(eps), big.NewRat(1, 1), big.NewRat(1, 1)}
			r, e := body.IntersectCanonicalWrappedFrustumBoreLineRational(ctx, p, d, zero, big.NewRat(1, 7), o)
			if e != nil || len(r.Hits) != 2 || len(r.Overlaps) != 0 {
				t.Fatal("nearby cap and wall contacts must separate", r, e)
			}
			owners = []int{proof.OuterFaces[0], proof.OuterFaces[1]}
			slices.Sort(owners)
			want := new(big.Rat).Quo(new(big.Rat).Mul(big.NewRat(4, 1), eps), new(big.Rat).Add(big.NewRat(1, 1), new(big.Rat).Mul(eps, eps)))
			if r.Hits[0].Parameter.Min.Sign() != 0 || r.Hits[0].Parameter.Max.Sign() != 0 || !slices.Equal(r.Hits[0].Faces, owners) ||
				r.Hits[1].Parameter.Min.Sign() <= 0 || r.Hits[1].Parameter.Min.Cmp(want) > 0 || r.Hits[1].Parameter.Max.Cmp(want) < 0 ||
				!slices.Equal(r.Hits[1].Faces, []int{proof.OuterFaces[0]}) {
				t.Fatal("rim or near-wall event is missing its original ownership", r, want)
			}

		})
	}
}
