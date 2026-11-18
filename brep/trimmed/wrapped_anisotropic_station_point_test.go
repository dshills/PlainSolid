package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"reflect"
	"slices"
	"testing"
)

func TestWrappedAnisotropicPointOwners(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	r := big.NewRat
	body := anisotropicMassFixture(t, false, geom.Vec3{})
	faces := body.Faces()
	slices.Reverse(faces)
	for i := range faces {
		loop := faces[i].Outer
		faces[i].Outer = append(append([]Use(nil), loop[3:]...), loop[:3]...)
	}
	body, e := NewSolidContext(ctx, body.Vertices(), body.Edges(), faces, 1e-8)
	if e != nil {
		t.Fatal(e)
	}
	var raw bytes.Buffer
	if e = WriteJSON(&raw, body); e != nil {
		t.Fatal(e)
	}
	body, e = ReadJSONContext(ctx, &raw, JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	proof, e := body.ValidateWrappedAnisotropicStationLoft(ctx, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	point := func(u, v, z, sx, sy *big.Rat) [3]*big.Rat {
		return [3]*big.Rat{new(big.Rat).Add(r(1, 4), new(big.Rat).Mul(sx, u)), new(big.Rat).Add(r(-1, 2), new(big.Rat).Mul(sy, v)), new(big.Rat).Set(z)}
	}
	locate := func(p [3]*big.Rat) WrappedAnisotropicStationPoint {
		t.Helper()
		got, e := body.LocateWrappedAnisotropicStationRational(ctx, p, WrappedRationalStationPointOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if got.ExactOperations <= 0 || got.ExactOperations > 4096 {
			t.Fatal("query ledger", got.ExactOperations)
		}
		return got
	}
	eps := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 100))
	for quadrant := 0; quadrant < 4; quadrant++ {
		u, v := r(5, 7), r(5, 7)
		for j := 0; j < quadrant; j++ {
			u, v = new(big.Rat).Neg(v), u
		}
		q := point(u, v, r(1, 3), r(7, 3), r(4, 3))
		got := locate(q)
		if got.Location != parametric.Boundary || !reflect.DeepEqual(got.Faces, []int{proof.History.WallFaces[0]}) || len(got.Edges) != 0 {
			t.Fatal("quadrant boundary", quadrant, got)
		}
		for _, sign := range []int64{-1, 1} {
			scale := new(big.Rat).Add(r(1, 1), new(big.Rat).Mul(r(sign, 1), eps))
			displaced := point(new(big.Rat).Mul(scale, u), new(big.Rat).Mul(scale, v), r(1, 3), r(7, 3), r(4, 3))
			a, _ := q[0].Float64()
			b, _ := displaced[0].Float64()
			if a != b {
				t.Fatal("probe is not subfloat")
			}
			got := locate(displaced)
			want := parametric.Inside
			if sign > 0 {
				want = parametric.Outside
			}
			if got.Location != want || len(got.Faces) != 0 || len(got.Edges) != 0 {
				t.Fatal("exact radial side", quadrant, sign, got)
			}
		}
		crease := locate(point(u, v, r(1, 1), r(1, 1), r(2, 1)))
		owners := append([]int(nil), proof.History.WallFaces...)
		slices.Sort(owners)
		if crease.Location != parametric.Boundary || !reflect.DeepEqual(crease.Faces, owners) || !reflect.DeepEqual(crease.RimEdges, []int{proof.History.StationRims[1][quadrant]}) {
			t.Fatal("crease owners", quadrant, crease)
		}
	}
	// Cardinal rim points have two incident rim edges and exactly one vertex;
	// positive X additionally owns both original wall seam ends.
	directions := [][2]int64{{1, 0}, {0, 1}, {-1, 0}, {0, -1}}
	for i, d := range directions {
		q := point(r(d[0], 1), r(d[1], 1), r(1, 1), r(1, 1), r(2, 1))
		got := locate(q)
		want := []int{proof.History.StationRims[1][i], proof.History.StationRims[1][(i+3)%4]}
		slices.Sort(want)
		if !reflect.DeepEqual(got.RimEdges, want) || len(got.Vertices) != 1 {
			t.Fatal("cardinal incidence", i, got)
		}
		vertex := body.Vertices()[got.Vertices[0]]
		for axis, x := range []float64{vertex.X, vertex.Y, vertex.Z} {
			if new(big.Rat).SetFloat64(x).Cmp(q[axis]) != 0 {
				t.Fatal("wrong original vertex")
			}
		}
		expected := 2
		if i == 0 {
			expected = 4
		}
		if len(got.Edges) != expected {
			t.Fatal("periodic seam incidence", i, got)
		}
	}
	seam := locate(point(r(1, 1), r(0, 1), r(1, 3), r(7, 3), r(4, 3)))
	if len(seam.Edges) != 1 || len(seam.RimEdges) != 0 || len(seam.Vertices) != 0 {
		t.Fatal("seam interior", seam)
	}
	for _, z := range []int64{0, 1, 3} {
		got := locate(point(r(0, 1), r(0, 1), r(z, 1), r(1, 1), r(1, 1)))
		if z == 1 {
			if got.Location != parametric.Inside || len(got.Faces) != 0 {
				t.Fatal("invented internal cap", got)
			}
		} else {
			idx := 0
			if z == 3 {
				idx = 1
			}
			if got.Location != parametric.Boundary || !reflect.DeepEqual(got.Faces, []int{proof.History.CapFaces[idx]}) || len(got.Edges) != 0 {
				t.Fatal("terminal cap", got)
			}
		}
	}
	capRim := locate(point(r(5, 7), r(5, 7), r(0, 1), r(3, 1), r(1, 1)))
	want := []int{proof.History.WallFaces[0], proof.History.CapFaces[0]}
	slices.Sort(want)
	if !reflect.DeepEqual(capRim.Faces, want) || len(capRim.RimEdges) != 1 {
		t.Fatal("cap-rim ownership", capRim)
	}
	capRim.Faces[0] = -100
	fresh := locate(point(r(5, 7), r(5, 7), r(0, 1), r(3, 1), r(1, 1)))
	if !reflect.DeepEqual(fresh.Faces, want) {
		t.Fatal("mutable source ownership")
	}
	outside := locate(point(r(2, 1), r(2, 1), r(0, 1), r(3, 1), r(1, 1)))
	if outside.Location != parametric.Outside {
		t.Fatal("infinite cap plane accepted")
	}
}

func TestWrappedAnisotropicPointLimits(t *testing.T) {
	ctx := context.Background()
	body := anisotropicMassFixture(t, false, geom.Vec3{})
	r := big.NewRat
	q := [3]*big.Rat{r(23, 12), r(19, 42), r(1, 3)}
	baseline, e := body.LocateWrappedAnisotropicStationRational(ctx, q, WrappedRationalStationPointOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if p, e := body.LocateWrappedAnisotropicStationRational(ctx, q, WrappedRationalStationPointOptions{MaxPointOperations: baseline.ExactOperations}); e != nil || p.ExactOperations != baseline.ExactOperations {
		t.Fatal("exact cumulative allowance", p, e)
	}
	if p, e := body.LocateWrappedAnisotropicStationRational(ctx, q, WrappedRationalStationPointOptions{MaxPointOperations: baseline.ExactOperations - 1}); e == nil || p.ExactOperations != 0 {
		t.Fatal("reset query budget", p, e)
	}

	for _, o := range []WrappedRationalStationPointOptions{{MaxPointOperations: 1}, {MaxPointOperations: 10}, {Source: geom.SurfacePatchOptions{MaxOperations: 1}}} {
		p, e := body.LocateWrappedAnisotropicStationRational(ctx, q, o)
		if e == nil || p.ExactOperations != 0 || len(p.Faces) != 0 {
			t.Fatal("partial quota result", p, e)
		}
	}
	q[0] = new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 100))
	if p, e := body.LocateWrappedAnisotropicStationRational(ctx, q, WrappedRationalStationPointOptions{MaxCoefficientBits: 64}); e == nil || p.ExactOperations != 0 {
		t.Fatal(p, e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if p, e := body.LocateWrappedAnisotropicStationRational(cancelled, q, WrappedRationalStationPointOptions{}); !errors.Is(e, context.Canceled) || len(p.Faces) != 0 {
		t.Fatal(p, e)
	}
	q[0] = nil
	if _, e := body.LocateWrappedAnisotropicStationRational(ctx, q, WrappedRationalStationPointOptions{}); !errors.Is(e, geom.ErrInvalid) {
		t.Fatal(e)
	}
}
