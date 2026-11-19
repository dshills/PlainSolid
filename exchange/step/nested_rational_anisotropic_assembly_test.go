package step

import (
	"bytes"
	"context"
	"math/big"
	"os"
	"plainsolid/geom"
	"testing"
)

func nestedRationalAnisotropicFixture(t testing.TB) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/occt-anisotropic-nested-oblique-assembly.step")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func nestedRationalAnisotropicBoreFixture(t testing.TB) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/occt-anisotropic-bore-nested-oblique-assembly.step")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestNestedRationalAnisotropicAssemblies(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	for _, bore := range []bool{false, true} {
		var instances []RationalHierarchyInstance
		var group int64
		var relations []ExactNAUOOccurrence
		if bore {
			raw := nestedRationalAnisotropicBoreFixture(t)
			m, e := ReadNestedRationalWrappedAnisotropicStationBoreAssembly(ctx, bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
			if e != nil {
				t.Fatal(e)
			}
			instances, group, relations = m.Instances, m.GroupID, m.Occurrences
			if _, e = ReadRationalWrappedAnisotropicStationBoreAssembly(ctx, bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{}); e == nil {
				t.Fatal("flat widened")
			}
			if _, e = ReadNestedWrappedAnisotropicStationBoreAssembly(ctx, bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{}); e == nil {
				t.Fatal("signed widened")
			}
			p, e := m.Body.Solid.ValidateWrappedAnisotropicStationBore(ctx, geom.SurfacePatchOptions{})
			if e != nil || !p.Complete {
				t.Fatal(e)
			}
		} else {
			raw := nestedRationalAnisotropicFixture(t)
			m, e := ReadNestedRationalWrappedAnisotropicStationAssembly(ctx, bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
			if e != nil {
				t.Fatal(e)
			}
			instances, group, relations = m.Instances, m.GroupID, m.Occurrences
			if _, e = ReadRationalWrappedAnisotropicStationAssembly(ctx, bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{}); e == nil {
				t.Fatal("flat widened")
			}
			if _, e = ReadNestedWrappedAnisotropicStationAssembly(ctx, bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{}); e == nil {
				t.Fatal("signed widened")
			}
		}
		if group == 0 || len(relations) != 3 || len(instances) != 2 {
			t.Fatal("grammar")
		}
		uses := map[int64]ExactNAUOOccurrence{}
		for _, u := range relations {
			uses[u.NAUOID] = u
		}
		for i, u := range instances {
			if len(u.Path) != 2 || uses[u.Path[0]].ChildID != group || uses[u.Path[1]].ParentID != group || uses[u.Path[1]].ChildID != u.ProductID {
				t.Fatal("path")
			}
			a := u.Matrix()
			x, y, ox, oy := big.NewRat(3, 5), big.NewRat(4, 5), big.NewRat(1, 1), big.NewRat(2, 1)
			if i == 1 {
				x, y, ox, oy = big.NewRat(-7, 25), big.NewRat(24, 25), big.NewRat(9, 1), big.NewRat(0, 1)
			}
			if a[0][0].Cmp(x) != 0 || a[1][0].Cmp(y) != 0 || a[0][3].Cmp(ox) != 0 || a[1][3].Cmp(oy) != 0 {
				t.Fatal(a)
			}
			a[0][0].SetInt64(99)
			if u.Matrix()[0][0].Cmp(x) != 0 {
				t.Fatal("borrowed exact matrix")
			}
		}
	}
}
func TestNestedRationalAnisotropicReject(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	for _, bore := range []bool{false, true} {
		raw := nestedRationalAnisotropicFixture(t)
		if bore {
			raw = nestedRationalAnisotropicBoreFixture(t)
		}
		run := func(b []byte, l Limits) bool {
			if bore {
				m, e := ReadNestedRationalWrappedAnisotropicStationBoreAssembly(ctx, bytes.NewReader(b), l, WrappedSurfaceOptions{})
				return e != nil && m.Source == nil
			}
			m, e := ReadNestedRationalWrappedAnisotropicStationAssembly(ctx, bytes.NewReader(b), l, WrappedSurfaceOptions{})
			return e != nil && m.Source == nil
		}
		bad := bytes.Replace(raw, []byte("(0.6,0.8,0.)"), []byte("(0.6000000000000001,0.8,0.)"), 1)
		if bytes.Equal(bad, raw) || !run(bad, Limits{}) {
			t.Fatal("rounded frame")
		}
		f, e := Parse(bytes.NewReader(raw), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		if !sideDiskEditRecord(f, "NEXT_ASSEMBLY_USAGE_OCCURRENCE", func(r *Record) { r.Args[4] = r.Args[3] }) {
			t.Fatal("use")
		}
		bad, e = mappedPlanarSourceBytes(f)
		if e != nil || !run(bad, Limits{}) {
			t.Fatal("cycle", e)
		}
		if !run(raw, Limits{MaxEntities: 10}) {
			t.Fatal("limit")
		}
	}
	// Individually bounded terminating translations can exceed the composed bound.
	huge := new(big.Rat).SetInt(new(big.Int).Mul(big.NewInt(9), new(big.Int).Exp(big.NewInt(10), big.NewInt(62), nil)))
	id := mappedRationalBasis{X: [3]*big.Rat{big.NewRat(1, 1), big.NewRat(0, 1), big.NewRat(0, 1)}, Y: [3]*big.Rat{big.NewRat(0, 1), big.NewRat(1, 1), big.NewRat(0, 1)}, Z: [3]*big.Rat{big.NewRat(0, 1), big.NewRat(0, 1), big.NewRat(1, 1)}, Origin: [3]*big.Rat{huge, big.NewRat(0, 1), big.NewRat(0, 1)}}
	if rationalHierarchyFrameBound(id) != nil {
		t.Fatal("fixture")
	}
	uses := []ExactNAUOOccurrence{{NAUOID: 1, ParentID: 2, ChildID: 3, pose: id}, {NAUOID: 2, ParentID: 1, ChildID: 2, pose: id}, {NAUOID: 3, ParentID: 1, ChildID: 2, pose: id}}
	if _, _, e := nestedRationalAnisotropicUses(3, uses); e == nil {
		t.Fatal("composition budget")
	}
}
