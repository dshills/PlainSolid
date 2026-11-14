package step

import (
	"bytes"
	"context"
	"math/big"
	"os"
	"plainsolid/geom"
	"testing"
)

func TestExactControlEllipticalPrismCodec(t *testing.T) {
	raw, e := os.ReadFile("testdata/occt-exact-control-elliptical-prism.step")
	if e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	m, e := ReadExactControlEllipticalPrism(ctx, bytes.NewReader(raw), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	if m.ExactVolumePiCoefficient().Cmp(big.NewRat(30, 1)) != 0 {
		t.Fatal("volume")
	}
	q := m.ExactVolumePiCoefficient()
	q.SetInt64(0)
	if m.ExactVolumePiCoefficient().Sign() != 1 {
		t.Fatal("aliased certificate")
	}
	if _, e = ReadPythagoreanEllipticalPrism(ctx, bytes.NewReader(raw), Limits{}, 1e-9); e == nil {
		t.Fatal("old reader accepted nonbinary endpoints")
	}
	d := ExactControlEllipticalPrismDefinition{Name: "rational prism", Origin: [3]*big.Rat{big.NewRat(1, 10), big.NewRat(2, 1), big.NewRat(3, 1)}, Reference: geom.Vec3{X: 3, Y: 4}, Axis: geom.Vec3{Z: 1}, MajorRadius: 5, MinorRadius: 2, Height: 3}
	var out bytes.Buffer
	if e = WriteExactControlEllipticalPrismSTEP(ctx, &out, d, 1e-9); e != nil {
		t.Fatal(e)
	}
	n, e := ReadExactControlEllipticalPrism(ctx, bytes.NewReader(out.Bytes()), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	if n.Name != d.Name || n.ExactVolumePiCoefficient().Cmp(m.ExactVolumePiCoefficient()) != 0 {
		t.Fatal(n.Name)
	}
	if p := os.Getenv("PLAINSOLID_EXACT_CONTROL_STEP_OUTPUT"); p != "" {
		if e = os.WriteFile(p, out.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
	}
	for _, mutation := range [][2][]byte{{[]byte("3.1,6.,3."), []byte("3.2,6.,3.")}, {[]byte("5.,2."), []byte("6.,2.")}} {
		bad := bytes.Replace(raw, mutation[0], mutation[1], 1)
		if bytes.Equal(bad, raw) {
			t.Fatal("mutation absent")
		}
		if _, e = ReadExactControlEllipticalPrism(ctx, bytes.NewReader(bad), Limits{}, 1e-9); e == nil {
			t.Fatal("tamper accepted")
		}
	}
	if _, e = ReadExactControlEllipticalPrism(ctx, bytes.NewReader(raw), Limits{MaxEntities: 10}, 1e-9); e == nil {
		t.Fatal("entity limit ignored")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	out.Reset()
	if e = WriteExactControlEllipticalPrismSTEP(cancelled, &out, d, 1e-9); e == nil || out.Len() != 0 {
		t.Fatal("cancellation not atomic")
	}
	d.Origin[0] = big.NewRat(1, 3)
	out.Reset()
	if e = WriteExactControlEllipticalPrismSTEP(ctx, &out, d, 1e-9); e == nil || out.Len() != 0 {
		t.Fatal("nonterminating source not atomic", e)
	}
}
