package step

import (
	"bytes"
	"context"
	"math/big"
	"os"
	"plainsolid/geom"
	"testing"
)

func rationalAnisotropicBoreFixture(t testing.TB) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/occt-anisotropic-bore-oblique-assembly.step")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestRationalAnisotropicBoreAssembly(t *testing.T) {
	ctx := context.Background()
	raw := rationalAnisotropicBoreFixture(t)
	m, e := ReadRationalWrappedAnisotropicStationBoreAssembly(ctx, bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Occurrences) != 2 {
		t.Fatal(m)
	}
	proof, e := m.Body.Solid.ValidateWrappedAnisotropicStationBore(ctx, geom.SurfacePatchOptions{})
	if e != nil || !proof.Complete || m.Body.Solid.Topology().EulerCharacteristic != 0 || m.Body.Solid.Topology().InnerWires != 2 {
		t.Fatal("annular material", e)
	}
	if len(m.Body.Outer.Walls) != 2 || len(m.Body.Inner.Walls) != 2 {
		t.Fatal("original bands")
	}
	for _, side := range []WrappedAnisotropicStationLoftBody{m.Body.Outer, m.Body.Inner} {
		if side.Walls[0].SideEdges[2] != side.Walls[1].SideEdges[0] {
			t.Fatal("shared original rim")
		}
	}
	if _, e = ReadRationalWrappedAnisotropicStationAssembly(ctx, bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{}); e == nil {
		t.Fatal("filled gate widened")
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
	if _, e = ReadWrappedAnisotropicStationBoreAssembly(ctx, bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{}); e == nil {
		t.Fatal("signed gate widened")
	}
	var out bytes.Buffer
	if e = WriteRationalWrappedAnisotropicStationBoreAssemblySource(ctx, &out, m.Source, Limits{}, WrappedSurfaceOptions{}); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadRationalWrappedAnisotropicStationBoreAssembly(ctx, &out, Limits{}, WrappedSurfaceOptions{}); e != nil {
		t.Fatal(e)
	}
}
func TestRationalAnisotropicBoreAssemblyReject(t *testing.T) {
	ctx := context.Background()
	raw := rationalAnisotropicBoreFixture(t)
	for _, b := range [][]byte{bytes.Replace(raw, []byte("(0.6,0.8,0.)"), []byte("(0.6000000000000001,0.8,0.)"), 1), bytes.Replace(raw, []byte("(0.6,0.8,0.)"), []byte("(0.6,0.8,0.1)"), 1), bytes.Replace(raw, []byte("#12 = CARTESIAN_POINT('',(0.,0.,0.))"), []byte("#12 = CARTESIAN_POINT('',(1.,0.,0.))"), 1)} {
		if bytes.Equal(raw, b) {
			t.Fatal("mutation not applied")
		}
		m, e := ReadRationalWrappedAnisotropicStationBoreAssembly(ctx, bytes.NewReader(b), Limits{}, WrappedSurfaceOptions{})
		if e == nil || m.Source != nil {
			t.Fatal("accepted", e)
		}
	}
	f, e := Parse(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if !sideDiskEditRecord(f, "CLOSED_SHELL", func(r *Record) { r.Args[1].Items = r.Args[1].Items[1:] }) {
		t.Fatal("shell")
	}
	forged, e := mappedPlanarSourceBytes(f)
	if e != nil {
		t.Fatal(e)
	}
	if m, e := ReadRationalWrappedAnisotropicStationBoreAssembly(ctx, bytes.NewReader(forged), Limits{}, WrappedSurfaceOptions{}); e == nil || m.Source != nil {
		t.Fatal("missing source face", e)
	}

	if _, e := ReadRationalWrappedAnisotropicStationBoreAssembly(ctx, bytes.NewReader(raw), Limits{MaxEntities: 10}, WrappedSurfaceOptions{}); e == nil {
		t.Fatal("limit")
	}
	c, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := ReadRationalWrappedAnisotropicStationBoreAssembly(c, bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{}); e == nil {
		t.Fatal("cancel")
	}
}
