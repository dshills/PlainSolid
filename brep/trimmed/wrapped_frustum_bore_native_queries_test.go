package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math/big"
	"slices"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestNativeWrappedFrustumOpenBoreReloadAndQueries(t *testing.T) {
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
