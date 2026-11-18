package trimmed

import (
	"bytes"
	"context"
	"math/big"
	"plainsolid/internal/slowtest"
	"reflect"
	"slices"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/placement"
	"plainsolid/geom"
)

func TestRationalStationProperOccurrence(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	r := big.NewRat
	body, _, e := NewWrappedRationalStationLoft(ctx, []*geom.WrappedSurfacePatch{rationalStationBand(t, 0, 1, 3, 1, .75), rationalStationBand(t, 1, 3, 1, 2, .75)}, 1e-8, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	faces := body.Faces()
	slices.Reverse(faces)
	body, e = NewSolidContext(ctx, body.Vertices(), body.Edges(), faces, 1e-8)
	if e != nil {
		t.Fatal(e)
	}
	var buf bytes.Buffer
	if e = WriteJSON(&buf, body); e != nil {
		t.Fatal(e)
	}
	body, e = ReadJSONContext(ctx, &buf, JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	proof, e := body.ValidateWrappedRationalStationLoft(ctx, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	matrix := [3][4]*big.Rat{{r(3, 5), r(0, 1), r(4, 5), r(1, 3)}, {r(0, 1), r(1, 1), r(0, 1), r(-2, 7)}, {r(-4, 5), r(0, 1), r(3, 5), r(5, 11)}}
	pose, e := placement.NewRationalRigid(matrix, 4096)
	if e != nil {
		t.Fatal(e)
	}
	q, e := body.PlaceWrappedRationalStation(ctx, pose, WrappedRationalStationOccurrenceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	// Independent component formula, including exact nonbinary translations.
	rotate := func(p [3]*big.Rat) [3]*big.Rat {
		return [3]*big.Rat{new(big.Rat).Add(new(big.Rat).Mul(r(3, 5), p[0]), new(big.Rat).Mul(r(4, 5), p[2])), new(big.Rat).Set(p[1]), new(big.Rat).Add(new(big.Rat).Mul(r(-4, 5), p[0]), new(big.Rat).Mul(r(3, 5), p[2]))}
	}
	world := func(p [3]*big.Rat) [3]*big.Rat {
		v := rotate(p)
		for i, shift := range []*big.Rat{r(1, 3), r(-2, 7), r(5, 11)} {
			v[i].Add(v[i], shift)
		}
		return v
	}
	point := func(x, y, z *big.Rat) [3]*big.Rat {
		return [3]*big.Rat{new(big.Rat).Add(r(1, 4), x), new(big.Rat).Add(r(-1, 2), y), z}
	}
	matrix[0][0].SetInt64(99)
	owned, e := q.Pose()
	if e != nil {
		t.Fatal(e)
	}
	owned[0][3].SetInt64(99)
	localPoints := [][3]*big.Rat{
		point(r(10, 7), r(10, 7), r(1, 2)), // genuine weighted noncircular wall
		point(r(1, 1), r(0, 1), r(1, 1)),   // two original crease normals
		point(r(0, 1), r(0, 1), r(0, 1)),   // lower terminal cap
		point(r(2, 1), r(0, 1), r(3, 1)),   // upper rim
	}
	for _, p := range localPoints {
		wp := world(p)
		if _, exact := wp[1].Float64(); exact {
			t.Fatal("fixture lost nonbinary world point")
		}
		got, e := q.LocateRational(ctx, wp, WrappedRationalStationPointOptions{})
		if e != nil {
			t.Fatal(e)
		}
		want, e := body.LocateWrappedRationalStationLoftRational(ctx, p, WrappedRationalStationPointOptions{})
		if e != nil || !reflect.DeepEqual(got, want) || got.Location != parametric.Boundary {
			t.Fatal("original owner invariance", got, want, e)
		}
		normals, e := q.BoundaryNormalsRational(ctx, wp, WrappedRationalStationNormalOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if len(normals.Normals) != len(want.Faces) {
			t.Fatal(normals)
		}
		for k := range p {
			if normals.Point[k].Cmp(wp[k]) != 0 || normals.Original.Point[k].Cmp(p[k]) != 0 {
				t.Fatal("world/source evidence")
			}
		}
		for i, n := range normals.Normals {
			original := normals.Original.Normals[i]
			expected := rotate(original.Direction)
			squared := new(big.Rat)
			for k := range expected {
				if n.Direction[k].Cmp(expected[k]) != 0 {
					t.Fatal("rotation", n, expected)
				}
				squared.Add(squared, new(big.Rat).Mul(n.Direction[k], n.Direction[k]))
			}
			if squared.Cmp(n.SquaredLength) != 0 || n.SquaredLength.Cmp(original.SquaredLength) != 0 || n.Face != original.Face || n.Face != want.Faces[i] {
				t.Fatal("normal metric/ancestry", n)
			}
		}
		saved := new(big.Rat).Set(normals.Normals[0].SquaredLength)
		normals.Original.Normals[0].SquaredLength.SetInt64(-1)
		if normals.Normals[0].SquaredLength.Cmp(saved) != 0 {
			t.Fatal("aliased normal evidence")
		}
		saved.Set(normals.Point[0])
		wp[0].SetInt64(-99)
		if normals.Point[0].Cmp(saved) != 0 {
			t.Fatal("aliased query")
		}
	}
	// Probe either side in exact world coordinates at distances below float ULP.
	p := localPoints[0]
	wp := world(p)
	n, e := q.BoundaryNormalsRational(ctx, wp, WrappedRationalStationNormalOptions{})
	if e != nil {
		t.Fatal(e)
	}
	epsilon := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 100))
	for _, sign := range []int64{-1, 1} {
		var probe [3]*big.Rat
		for i := range probe {
			d := new(big.Rat).Mul(n.Normals[0].Direction[i], epsilon)
			d.Mul(d, r(sign, 1))
			probe[i] = new(big.Rat).Add(wp[i], d)
		}
		got, e := q.LocateRational(ctx, probe, WrappedRationalStationPointOptions{})
		want := parametric.Inside
		if sign > 0 {
			want = parametric.Outside
		}
		if e != nil || got.Location != want {
			t.Fatal("world side", sign, got, e)
		}
	}
	sourceFaces := q.Source().Faces()
	sourceFaces[0].Outer = nil
	if got, e := q.LocateRational(ctx, world(point(r(0, 1), r(0, 1), r(1, 1))), WrappedRationalStationPointOptions{}); e != nil || got.Location != parametric.Inside {
		t.Fatal("source snapshot", got, e)
	}
	if len(proof.History.WallFaces) != 2 {
		t.Fatal("fixture")
	}
}

func TestRationalStationOccurrenceGuards(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	body, _, e := NewWrappedRationalStationLoft(ctx, []*geom.WrappedSurfacePatch{rationalStationBand(t, 0, 1, 3, 1, .6)}, 1e-8, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	identity := [3][4]*big.Rat{{r(1, 1), r(0, 1), r(0, 1), r(0, 1)}, {r(0, 1), r(1, 1), r(0, 1), r(0, 1)}, {r(0, 1), r(0, 1), r(1, 1), r(0, 1)}}
	pose, e := placement.NewRationalRigid(identity, 64)
	if e != nil {
		t.Fatal(e)
	}
	q, e := body.PlaceWrappedRationalStation(ctx, pose, WrappedRationalStationOccurrenceOptions{MaxTransformBits: 64})
	if e != nil {
		t.Fatal(e)
	}
	p := [3]*big.Rat{r(13, 4), r(-1, 2), r(0, 1)}
	// Canonical normal proof succeeds, but its exact .6-weight gradient needs
	// >64 bits. Failed world transport must not expose a partial local result.
	if _, e := body.WrappedRationalStationBoundaryNormalsRational(ctx, p, WrappedRationalStationNormalOptions{}); e != nil {
		t.Fatal(e)
	}
	if got, e := q.BoundaryNormalsRational(ctx, p, WrappedRationalStationNormalOptions{}); e == nil || len(got.Normals) != 0 || len(got.Original.Normals) != 0 || got.Point[0] != nil {
		t.Fatal("non-atomic transport", got, e)
	}
	huge := p
	huge[0] = new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 100))
	if got, e := q.LocateRational(ctx, huge, WrappedRationalStationPointOptions{}); e == nil || len(got.Faces) != 0 {
		t.Fatal("transform bit quota", got, e)
	}
	if _, e := (WrappedRationalStationOccurrence{}).LocateRational(ctx, p, WrappedRationalStationPointOptions{}); e == nil {
		t.Fatal("zero occurrence")
	}
	if _, e := body.PlaceWrappedRationalStation(ctx, placement.RationalRigid{}, WrappedRationalStationOccurrenceOptions{}); e == nil {
		t.Fatal("zero pose")
	}
	if _, e := (Solid{}).PlaceWrappedRationalStation(ctx, pose, WrappedRationalStationOccurrenceOptions{}); e == nil {
		t.Fatal("unproved source")
	}
	if _, e := body.PlaceWrappedRationalStation(ctx, pose, WrappedRationalStationOccurrenceOptions{Source: geom.SurfacePatchOptions{MaxOperations: 1}}); e == nil {
		t.Fatal("source quota")
	}
	identity[0][0] = r(-1, 1)
	if _, e := placement.NewRationalRigid(identity, 64); e == nil {
		t.Fatal("reflection")
	}
	identity[0][0] = r(2, 1)
	if _, e := placement.NewRationalRigid(identity, 64); e == nil {
		t.Fatal("nonrigid")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := q.BoundaryNormalsRational(cancelled, p, WrappedRationalStationNormalOptions{}); e == nil {
		t.Fatal("cancel")
	}
}
