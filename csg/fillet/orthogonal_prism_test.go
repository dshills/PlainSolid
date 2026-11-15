package fillet

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"os/exec"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func reentrantFilletProfile() []geom.Vec2 {
	return []geom.Vec2{{}, {X: 4}, {X: 4, Y: 1}, {X: 1, Y: 1}, {X: 1, Y: 4}, {Y: 4}}
}

func TestExactOrthogonalPrismConvexAndReentrantFillets(t *testing.T) {
	ctx := context.Background()
	const r = .25
	q, err := PrismOrthogonalCorners(ctx, reentrantFilletProfile(), -1, 1, []CornerRadius{{Corner: 1, Radius: r}, {Corner: 3, Radius: r}}, OrthogonalPrismOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(q.Solid.Faces()) != 10 || len(q.Solid.Edges()) != 24 || len(q.Solid.Vertices()) != 16 || q.NominalArea != 7 ||
		math.Abs(q.NominalPerimeter-(15+math.Pi/4)) > 1e-13 {
		t.Fatalf("wrong mixed fillet source ledger: %+v", q)
	}
	for _, tc := range []struct {
		i                  int
		center, in, out, p geom.Vec2
		sign               float64
	}{
		{1, geom.Vec2{X: 3.75, Y: .25}, geom.Vec2{X: 3.75}, geom.Vec2{X: 4, Y: .25}, geom.Vec2{X: 3.95, Y: .1}, 1},
		{3, geom.Vec2{X: 1.25, Y: 1.25}, geom.Vec2{X: 1.25, Y: 1}, geom.Vec2{X: 1, Y: 1.25}, geom.Vec2{X: 1.05, Y: 1.1}, -1},
	} {
		c := q.Corners[tc.i]
		if c == nil || c.Center != tc.center || c.Incoming != tc.in || c.Outgoing != tc.out || c.Sweep != tc.sign*math.Pi/2 {
			t.Fatalf("incorrect original tangent support for corner %d: %+v", tc.i, c)
		}
		edge := q.Solid.Edges()[c.BottomArc]
		p, err := edge.Curve.Eval(.5)
		if err != nil || p.Sub(geom.Vec3{X: tc.p.X, Y: tc.p.Y, Z: -1}).Length() > 1e-13 ||
			math.Abs(math.Hypot(p.X-c.Center.X, p.Y-c.Center.Y)-r) > 1e-13 {
			t.Fatalf("original directed circular arc changed: %v %v", p, err)
		}
	}
	p, err := q.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
	if err != nil || math.Abs(p.Volume-14) > 1e-7 || math.Abs(p.Area-(44+math.Pi/2)) > 1e-7 {
		t.Fatalf("signed reentrant material failed independent mass formula: %+v %v", p, err)
	}
	var raw bytes.Buffer
	if err := trimmed.WriteJSON(&raw, q.Solid); err != nil {
		t.Fatal(err)
	}
	got, err := trimmed.ReadJSONContext(ctx, bytes.NewReader(raw.Bytes()), trimmed.JSONLimits{})
	if err != nil {
		t.Fatal(err)
	}
	m, err := got.MassProperties(ctx, parametric.IntegrationOptions{})
	if err != nil || math.Abs(m.Volume-p.Volume) > 1e-10 || math.Abs(m.Area-p.Area) > 1e-10 {
		t.Fatalf("native reload did not preserve CW and CCW arc trims: %+v %v", m, err)
	}
}

func TestExactOrthogonalPrismReentrantFilletRejectsInvalidInputs(t *testing.T) {
	ctx := context.Background()
	for _, test := range []struct {
		r     float64
		limit int
		want  error
	}{
		{3, 0, ErrRadius},
		{.1, 0, geom.ErrUnrepresentable},
		{.25, 1, ErrLimit},
	} {
		v, err := PrismOrthogonalCorners(ctx, reentrantFilletProfile(), -1, 1, []CornerRadius{{Corner: 3, Radius: test.r}}, OrthogonalPrismOptions{MaxWork: test.limit})
		if !errors.Is(err, test.want) || len(v.Solid.Faces()) != 0 {
			t.Fatalf("invalid exact fillet published material: %+v %v", v, err)
		}
	}
	p := reentrantFilletProfile()
	p[3].X, p[3].Y = 5, 1
	if v, err := PrismOrthogonalCorners(ctx, p, -1, 1, []CornerRadius{{Corner: 3, Radius: .25}}, OrthogonalPrismOptions{}); err == nil || len(v.Solid.Faces()) != 0 {
		t.Fatalf("crossing stock published a fillet: %+v %v", v, err)
	}
}

func TestExactOrthogonalPrismFilletsLiveOCCT(t *testing.T) {
	program := os.Getenv("PLAINSOLID_OCCT_ORTHOGONAL_PRISM_FILLETS")
	if program == "" {
		t.Skip("optional pinned OCCT fillet reference")
	}
	data, err := exec.Command(program).CombinedOutput()
	var ref struct {
		Faces        int
		Volume, Area float64
	}
	if err != nil || json.Unmarshal(data, &ref) != nil {
		t.Fatalf("independent OCCT fillet: %v %s", err, data)
	}
	ctx := context.Background()
	s, err := PrismOrthogonalCorners(ctx, reentrantFilletProfile(), -1, 1, []CornerRadius{{Corner: 1, Radius: .25}, {Corner: 3, Radius: .25}}, OrthogonalPrismOptions{})
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
	if err != nil || ref.Faces != len(s.Solid.Faces()) || math.Abs(ref.Volume-p.Volume) > 1e-7 || math.Abs(ref.Area-p.Area) > 1e-7 {
		t.Fatalf("original selected OCCT edges disagree: ref %+v, Go %+v, %v", ref, p, err)
	}
}
