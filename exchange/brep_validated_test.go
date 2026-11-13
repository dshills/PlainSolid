package exchange

import (
	"bytes"
	"context"
	"errors"
	"math/big"
	"plainsolid/brep"
	"plainsolid/collision"
	"plainsolid/geom"
	"testing"
)

func validatedBox(t *testing.T, min, max geom.Vec3) brep.Solid {
	t.Helper()
	s, e := brep.Box(min, max)
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func validatedJoin(t *testing.T, solids ...brep.Solid) brep.Solid {
	t.Helper()
	var vs []geom.Vec3
	var es []brep.Edge
	var fs []brep.Face
	for _, s := range solids {
		vo, eo := len(vs), len(es)
		vs = append(vs, s.Vertices()...)
		for _, e := range s.Edges() {
			es = append(es, brep.Edge{Start: e.Start + vo, End: e.End + vo})
		}
		for _, f := range s.Faces() {
			for i := range f.Boundary {
				f.Boundary[i].Edge += eo
			}
			fs = append(fs, f)
		}
	}
	s, e := brep.NewSolid(vs, es, fs)
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func nativeBytes(t *testing.T, s brep.Solid) []byte {
	t.Helper()
	var b bytes.Buffer
	if e := WriteBRepJSON(&b, s); e != nil {
		t.Fatal(e)
	}
	return b.Bytes()
}
func TestValidatedNativeObliqueCavity(t *testing.T) {
	outer := validatedBox(t, geom.Vec3{}, geom.Vec3{X: 4, Y: 4, Z: 4})
	void := validatedBox(t, geom.Vec3{X: 1, Y: 1, Z: 1}, geom.Vec3{X: 3, Y: 3, Z: 3}).Reversed()
	s := validatedJoin(t, outer, void)
	a := geom.Identity()
	a[0][1] = .5
	a[1][2] = .25
	s, e := s.Transformed(a)
	if e != nil {
		t.Fatal(e)
	}
	read, report, e := ReadBRepJSONValidated(context.Background(), bytes.NewReader(nativeBytes(t, s)), BRepLimits{}, collision.AssemblyOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if report.SignedVolume.Cmp(big.NewRat(56, 1)) != 0 || len(report.Shells) != 2 || report.Shells[1].Depth != 1 || report.Shells[1].Parent != 0 {
		t.Fatal(report)
	}
	if len(read.Vertices()) != len(s.Vertices()) {
		t.Fatal("entity identity")
	}
	for _, sh := range report.Shells {
		for _, f := range sh.Faces {
			if f.Solid != 0 || f.Face < 0 || f.Face >= len(s.Faces()) {
				t.Fatal("ancestry")
			}
		}
	}
}
func TestValidatedNativeRejectOverlap(t *testing.T) {
	a := validatedBox(t, geom.Vec3{}, geom.Vec3{X: 2, Y: 2, Z: 2})
	b := validatedBox(t, geom.Vec3{X: 1, Y: 1, Z: 1}, geom.Vec3{X: 3, Y: 3, Z: 3})
	data := nativeBytes(t, validatedJoin(t, a, b))
	if _, e := ReadBRepJSON(bytes.NewReader(data), BRepLimits{}); e != nil {
		t.Fatal("legacy parsing changed", e)
	}
	s, r, e := ReadBRepJSONValidated(context.Background(), bytes.NewReader(data), BRepLimits{}, collision.AssemblyOptions{})
	if e == nil || len(s.Faces()) != 0 || len(r.Shells) != 0 {
		t.Fatal("overlap accepted")
	}
}
func TestValidatedNativeLimitsCancellation(t *testing.T) {
	s := validatedBox(t, geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
	data := nativeBytes(t, s)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, e := ReadBRepJSONValidated(ctx, bytes.NewReader(data), BRepLimits{}, collision.AssemblyOptions{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if _, _, e := ReadBRepJSONValidated(context.Background(), bytes.NewReader(data), BRepLimits{MaxBytes: 1}, collision.AssemblyOptions{}); e == nil {
		t.Fatal("parser byte limit")
	}
	if _, _, e := ReadBRepJSONValidated(context.Background(), bytes.NewReader(data), BRepLimits{}, collision.AssemblyOptions{Validation: collision.LocationOptions{MaxTriangles: 1}}); !errors.Is(e, collision.ErrLimit) {
		t.Fatal(e)
	}
}
