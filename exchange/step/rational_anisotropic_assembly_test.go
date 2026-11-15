package step

import (
	"bytes"
	"context"
	"math/big"
	"os"
	"plainsolid/geom"
	"testing"
)

func rationalAnisotropicFixture(t testing.TB) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/occt-anisotropic-oblique-assembly.step")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestRationalAnisotropicAssembly(t *testing.T) {
	ctx := context.Background()
	raw := rationalAnisotropicFixture(t)
	m, e := ReadRationalWrappedAnisotropicStationAssembly(ctx, bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Occurrences) != 2 {
		t.Fatal(m)
	}
	var found bool
	for _, u := range m.Occurrences {
		a := u.Matrix()
		if a[0][0].Cmp(big.NewRat(3, 5)) == 0 {
			found = true
			if a[1][0].Cmp(big.NewRat(4, 5)) != 0 || a[0][1].Cmp(big.NewRat(-4, 5)) != 0 {
				t.Fatal(a)
			}
			p, e := u.ApplyExact(geom.Vec3{X: 1})
			if e != nil || p[0].Cmp(big.NewRat(53, 5)) != 0 {
				t.Fatal(p, e)
			}
			if _, e = u.ApplyRepresented(geom.Vec3{X: 1}); e == nil {
				t.Fatal("rounded world point")
			}
			a[0][0].SetInt64(99)
			if u.Matrix()[0][0].Cmp(big.NewRat(3, 5)) != 0 {
				t.Fatal("mutable matrix")
			}
		}
	}
	if !found {
		t.Fatal("missing oblique frame")
	}
	if _, e = ReadWrappedAnisotropicStationAssembly(ctx, bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{}); e == nil {
		t.Fatal("signed gate widened")
	}
	var out bytes.Buffer
	if e = WriteRationalWrappedAnisotropicStationAssemblySource(ctx, &out, m.Source, Limits{}, WrappedSurfaceOptions{}); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadRationalWrappedAnisotropicStationAssembly(ctx, &out, Limits{}, WrappedSurfaceOptions{}); e != nil {
		t.Fatal(e)
	}
}
func TestRationalAnisotropicAssemblyReject(t *testing.T) {
	ctx := context.Background()
	raw := rationalAnisotropicFixture(t)
	for _, b := range [][]byte{bytes.Replace(raw, []byte("#19 = AXIS2_PLACEMENT_3D('',#20,#21,#22)"), []byte("#19 = AXIS2_PLACEMENT_3D('',#20,#109,#22)"), 1), bytes.Replace(raw, []byte("(0.6,0.8,0.)"), []byte("(0.6000000000000001,0.8,0.)"), 1), bytes.Replace(raw, []byte("(0.6,0.8,0.)"), []byte("(0.6,0.8,0.1)"), 1), bytes.Replace(raw, []byte("#12 = CARTESIAN_POINT('',(0.,0.,0.))"), []byte("#12 = CARTESIAN_POINT('',(1.,0.,0.))"), 1)} {
		if bytes.Equal(raw, b) {
			t.Fatal("mutation not applied")
		}
		m, e := ReadRationalWrappedAnisotropicStationAssembly(ctx, bytes.NewReader(b), Limits{}, WrappedSurfaceOptions{})
		if e == nil || m.Source != nil {
			t.Fatal("accepted", e)
		}
	}
	if _, e := ReadRationalWrappedAnisotropicStationAssembly(ctx, bytes.NewReader(raw), Limits{MaxEntities: 10}, WrappedSurfaceOptions{}); e == nil {
		t.Fatal("limit")
	}
	c, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := ReadRationalWrappedAnisotropicStationAssembly(c, bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{}); e == nil {
		t.Fatal("cancel")
	}
}
