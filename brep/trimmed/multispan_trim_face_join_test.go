//go:build research

package trimmed

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func multiSpanJoinFixture(t *testing.T) (QuadraticTrimFaceJoinInput, QuadraticTrimFaceJoinInput) {
	t.Helper()
	patches := [2]*geom.SurfacePatch{partitionedSeamPatch(t, false), partitionedSeamPatchY(t, true, true)}
	var inputs [2]QuadraticTrimFaceJoinInput
	corners := []geom.Vec3{{Y: .25}, {X: 1, Y: .25}, {X: 1, Y: .5}, {Y: .5}}
	for fi := range inputs {
		inputs[fi].Patch = patches[fi]
		for j, p := range corners {
			lo, hi := 0., 1.
			var weights []float64
			if j == 0 {
				if fi == 0 {
					lo, hi = -2, 6
					weights = []float64{1, .5}
				} else {
					lo, hi = 10, 26
					weights = []float64{.5, 1}
				}
			}
			c, e := geom.NewBSpline(1, []geom.Vec3{p, corners[(j+1)%4]}, []float64{lo, lo, hi, hi}, weights)
			if e != nil {
				t.Fatal(e)
			}
			inputs[fi].Outer = append(inputs[fi].Outer, SplineUV{Curve: c, Range: UVInterval{Min: lo, Max: hi}})
		}
	}
	return inputs[0], inputs[1]
}
func TestMultiSpanTrimFaceJoinExactRationalCuts(t *testing.T) {
	slowtest.Skip(t)
	a, b := multiSpanJoinFixture(t)
	opts := MultiSpanTrimFaceJoinOptions{Face: MultiSpanCurvedTrimMeshOptions{Mesh: CurvedQuadraticTrimMeshOptions{Deflection: .15}}}
	got, e := StitchMultiSpanTrimFacesCertified(context.Background(), a, b, opts)
	if e != nil {
		t.Fatal(e)
	}
	if len(got.Seam.Pieces) != 4 {
		t.Fatal("partition")
	}
	present := map[string]bool{}
	for i, q := range got.SharedParameters {
		present[q.RatString()] = true
		p := got.Mesh.Vertices[got.SharedVertices[i]]
		x := new(big.Rat).Quo(q, new(big.Rat).Sub(big.NewRat(2, 1), q))
		f := new(big.Rat).Mul(x, x)
		if x.Cmp(big.NewRat(1, 2)) > 0 {
			d := new(big.Rat).Sub(x, big.NewRat(1, 2))
			f.Add(big.NewRat(1, 4), new(big.Rat).Mul(d, d))
		}
		z := new(big.Rat).Add(f, new(big.Rat).Quo(x, big.NewRat(4, 1)))
		xf, _ := x.Float64()
		zf, _ := z.Float64()
		if p != (geom.Vec3{X: xf, Y: .25, Z: zf}) {
			t.Fatal("independent curve witness", q, p)
		}
	}
	for _, q := range []string{"0", "2/5", "2/3", "6/7", "1"} {
		if !present[q] {
			t.Fatal("missing exact cut", q)
		}
	}
	edges := map[[2]int]int{}
	for _, tri := range got.Mesh.Triangles {
		for j, a := range tri {
			b := tri[(j+1)%3]
			if a > b {
				a, b = b, a
			}
			edges[[2]int{a, b}]++
		}
	}
	if len(got.Mesh.Vertices)-len(edges)+len(got.Mesh.Triangles) != 1 {
		t.Fatal("not a disk")
	}
	for i := 0; i+1 < len(got.SharedVertices); i++ {
		a, b := got.SharedVertices[i], got.SharedVertices[i+1]
		if a > b {
			a, b = b, a
		}
		if edges[[2]int{a, b}] != 2 {
			t.Fatal("seam incidence")
		}
	}
	for fi, face := range got.Faces {
		if face.SpanCellChecks == 0 || face.SpanPieceChecks == 0 || len(face.SquaredErrors) != len(face.Mesh.Triangles) {
			t.Fatal("missing recertification")
		}
		if face.FillSquaredDistance.Cmp(new(big.Rat).Mul(ratInset(.15), ratInset(.15))) > 0 {
			t.Fatal("distance")
		}
		doubleArea := new(big.Rat)
		for _, tri := range face.Mesh.Triangles {
			a, b, c := face.ExactUV[tri[0]], face.ExactUV[tri[1]], face.ExactUV[tri[2]]
			ux, uy := new(big.Rat).Sub(b[0], a[0]), new(big.Rat).Sub(b[1], a[1])
			vx, vy := new(big.Rat).Sub(c[0], a[0]), new(big.Rat).Sub(c[1], a[1])
			area := new(big.Rat).Sub(new(big.Rat).Mul(ux, vy), new(big.Rat).Mul(uy, vx))
			if area.Sign() <= 0 {
				t.Fatal("degenerate or reversed UV split")
			}
			doubleArea.Add(doubleArea, area)
		}
		if doubleArea.Cmp(big.NewRat(1, 2)) != 0 {
			t.Fatal("exact original rectangle coverage", doubleArea)
		}
		for _, id := range face.OriginalFace {
			if id != fi {
				t.Fatal("ancestry")
			}
		}
	}
	t.Logf("triangles=%d shared=%d cells=%d/%d", len(got.Mesh.Triangles), len(got.SharedVertices), got.Faces[0].SpanCellChecks, got.Faces[1].SpanCellChecks)
	opts.MaxSharedVertices = 4
	if r, e := StitchMultiSpanTrimFacesCertified(context.Background(), a, b, opts); !errors.Is(e, geom.ErrConvergence) || len(r.Mesh.Triangles) != 0 {
		t.Fatal("partial vertex budget", e)
	}
	opts.MaxSharedVertices = 0
	opts.Face.MaxSpanCells = got.Faces[1].SpanCellChecks - 1
	// Face 1's cumulative count exceeds this cap during the final recertification.
	if r, e := StitchMultiSpanTrimFacesCertified(context.Background(), a, b, opts); !errors.Is(e, geom.ErrConvergence) || len(r.Mesh.Triangles) != 0 {
		t.Fatal("partial cell budget", e)
	}
}

func TestMultiSpanTrimFaceJoinRejectsUnsupportedAndCancellation(t *testing.T) {
	a, b := multiSpanJoinFixture(t)
	o := MultiSpanTrimFaceJoinOptions{Face: MultiSpanCurvedTrimMeshOptions{Mesh: CurvedQuadraticTrimMeshOptions{Deflection: .15}}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if r, e := StitchMultiSpanTrimFacesCertified(ctx, a, b, o); !errors.Is(e, context.Canceled) || len(r.Mesh.Triangles) != 0 {
		t.Fatal(e)
	}
	o.MaxSharedVertices = -1
	if _, e := StitchMultiSpanTrimFacesCertified(context.Background(), a, b, o); !errors.Is(e, geom.ErrInvalid) {
		t.Fatal(e)
	}
	o.MaxSharedVertices = 0
	b.Outer[0].Reversed = true
	if r, e := StitchMultiSpanTrimFacesCertified(context.Background(), a, b, o); e == nil || len(r.Mesh.Triangles) != 0 {
		t.Fatal("same incidence accepted")
	}
}
