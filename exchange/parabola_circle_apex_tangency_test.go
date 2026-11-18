package exchange_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"os"
	"plainsolid/internal/slowtest"
	"slices"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange/iges"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func TestParabolaCircleApexTangencyEnvelopeExchange(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	const unit = .001
	proof := edgecheck.Options{}
	for _, c := range []struct {
		op                string
		axis, side, order int
		want              float64
	}{
		{"common", 0, 0, 0, 14065067533.0/7046430720 - 1155.0*math.Pi/134217728},
		{"common", 1, 1, 1, 14028780973.0/7046430720 - 1149.0*math.Pi/134217728},
		{"fuse", 0, 1, 0, 2844557431.0/1409286144 + 1149.0*math.Pi/134217728},
		{"fuse", 1, 0, 1, 2876007319.0/1409286144 + 1155.0*math.Pi/134217728},
	} {
		t.Run(fmt.Sprintf("%s/%d/%d", c.op, c.axis, c.side), func(t *testing.T) {
			raw := apexTangencyExchangeSource(t, c.op, c.axis, c.side, c.order)
			r, err := trimmed.ReadParabolaCircleApexTangencyEnvelopeJSON(ctx, bytes.NewReader(raw), trimmed.JSONLimits{}, trimmed.ParabolaCircleGraphOptions{})
			if err != nil {
				t.Fatal(err)
			}
			s, h := r.Boundary(), r.History()
			exact, err := s.ExtractExactBezierBoundary(ctx, proof)
			if err != nil {
				t.Fatal(err)
			}
			checkApexTransferVertex(t, exact.Vertices[h.Vertex], c.axis)
			for branch, rays := range h.Rays {
				for direction, id := range rays {
					checkApexTransferRay(t, exact, id, h.Vertex, c.axis, c.side, branch, direction)
				}
			}
			checkApexTransferBridge(t, exact, h, c.axis, c.side)
			var st, ig bytes.Buffer
			if err := step.WriteParabolaCircleApexTangencyEnvelope(ctx, &st, r, c.op, unit, step.Limits{}, proof); err != nil {
				t.Fatal(err)
			}
			if err := iges.WriteParabolaCircleApexTangencyEnvelope(ctx, &ig, r, unit, iges.Limits{}, proof); err != nil {
				t.Fatal(err)
			}
			if err := step.VerifyParabolaCircleApexTangencyEnvelopeSTEP(ctx, bytes.NewReader(st.Bytes()), r, c.op, unit, step.Limits{}, proof); err != nil {
				t.Fatal(err)
			}
			if err := iges.VerifyParabolaCircleApexTangencyEnvelopeIGES(ctx, bytes.NewReader(ig.Bytes()), r, unit, iges.Limits{}, proof); err != nil {
				t.Fatal(err)
			}
			checkTwoCirclesExchangeReadback(t, ctx, []trimmed.Solid{s}, []float64{c.want}, st.Bytes(), ig.Bytes())
		})
	}
}

func TestParabolaCircleApexTangencyCutExchange(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	const name, unit = "Parabola-circle tangent Cut", .001
	proof := edgecheck.Options{}
	volumes := [2][]float64{
		{57435031.0 / 1409286144, 1155.0 * math.Pi / 134217728},
		{64080467.0 / 7046430720, 1149.0 * math.Pi / 134217728},
	}
	for axis := range 2 {
		for side, want := range volumes {
			t.Run(fmt.Sprintf("%d/%d", axis, side), func(t *testing.T) {
				raw := apexTangencyExchangeSource(t, "cut", axis, side, 1-side)
				r, err := trimmed.ReadParabolaCircleApexTangencyCutJSON(ctx, bytes.NewReader(raw), trimmed.JSONLimits{}, trimmed.ParabolaCircleGraphOptions{})
				if err != nil {
					t.Fatal(err)
				}
				parts, apex := r.Components(), r.Contact()
				wallControls := 0
				for part, s := range parts {
					exact, err := s.ExtractExactBezierBoundary(ctx, proof)
					if err != nil {
						t.Fatal(err)
					}
					checkApexTransferVertex(t, exact.Vertices[apex.Vertices[part]], axis)
					for direction, id := range apex.Rays[part] {
						checkApexTransferRay(t, exact, id, apex.Vertices[part], axis, side, part, direction)
					}
					if axis == 0 && side == 1 && part == 0 {
						wallControls = countTwoCirclesExchangeControls(t, exact)
					}
				}
				var st, ig bytes.Buffer
				if err := step.WriteParabolaCircleApexTangencyCut(ctx, &st, r, name, unit, step.Limits{}, proof); err != nil {
					t.Fatal(err)
				}
				if err := iges.WriteParabolaCircleApexTangencyCut(ctx, &ig, r, unit, iges.Limits{}, proof); err != nil {
					t.Fatal(err)
				}
				if err := step.VerifyParabolaCircleApexTangencyCutSTEP(ctx, bytes.NewReader(st.Bytes()), r, name, unit, step.Limits{}, proof); err != nil {
					t.Fatal(err)
				}
				if err := iges.VerifyParabolaCircleApexTangencyCutIGES(ctx, bytes.NewReader(ig.Bytes()), r, unit, iges.Limits{}, proof); err != nil {
					t.Fatal(err)
				}
				if wallControls > 0 {
					var blocked bytes.Buffer
					if err := step.WriteParabolaCircleApexTangencyCut(ctx, &blocked, r, name, unit, step.Limits{MaxSolids: 1}, proof); !errors.Is(err, geom.ErrConvergence) || blocked.Len() != 0 {
						t.Fatal("STEP published a partial source contact pair", err)
					}
					if err := iges.WriteParabolaCircleApexTangencyCut(ctx, &blocked, r, unit, iges.Limits{MaxControlPoints: wallControls}, proof); !errors.Is(err, geom.ErrConvergence) || blocked.Len() != 0 {
						t.Fatal("IGES published a partial source contact pair", err)
					}
				}
				// Validate both exact local apex occurrences, full original
				// vertex and edge incidence, independent indexing and mass.
				checkTwoCirclesExchangeReadback(t, ctx, parts[:], want, st.Bytes(), ig.Bytes())
			})
		}
	}
}

func apexTangencyExchangeSource(t *testing.T, op string, axis, side, order int) []byte {
	t.Helper()
	raw, err := os.ReadFile(fmt.Sprintf("testdata/native_parabola_circle_apex_%s-axis-%d-side-%d-order-%d.json", op, axis, side, order))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func checkApexTransferVertex(t *testing.T, p geom.Vec3, axis int) {
	t.Helper()
	want := geom.Vec3{X: .5, Y: 3. / 8, Z: 2}
	if axis == 1 {
		want.X, want.Y = want.Y, want.X
	}
	if p != want {
		t.Fatal("displaced apex vertex", p, want)
	}
}

func checkApexTransferRay(t *testing.T, b trimmed.ExactBezierBoundary, id, vertex, axis, side, branch, direction int) {
	t.Helper()
	edge := b.Edges[id]
	if (edge.Start == vertex) == (edge.End == vertex) {
		t.Fatal("apex edge does not meet the original vertex", id, vertex)
	}
	atEnd := edge.End == vertex
	second := big.NewRat(1, 4)
	if branch == 1 {
		second.SetInt64(int64(16*side - 8))
	}
	checkApexTransferJet(t, edge.Model, atEnd, axis, direction, 2, second)
	owners := make(map[*geom.BSplineSurface]bool)
	forward := 0
	for _, face := range b.Faces {
		for _, loop := range face.Loops {
			for _, use := range loop {
				if use.Edge != id {
					continue
				}
				if face.Spline == nil || owners[face.Spline] {
					t.Fatal("lost a distinct original tangent roof owner", id)
				}
				owners[face.Spline] = true
				if face.Reversed == use.Reversed {
					forward++
				}
				checkApexTransferJet(t, use.UV, atEnd, axis, direction, 0, second)
			}
		}
	}
	if len(owners) != 2 || forward != 1 {
		t.Fatal("lost both opposed original apex coedges", id)
	}
}

func checkApexTransferJet(t *testing.T, span geom.ExactBezierSpan, atEnd bool, axis, direction int, z int64, second *big.Rat) {
	t.Helper()
	if len(span.Control) != 3 || span.Start != 0 || span.End != 1 {
		t.Fatal("lost normalized exact quadratic edge clock")
	}
	controls := slices.Clone(span.Control)
	if atEnd {
		slices.Reverse(controls)
	}
	d := 1 - axis
	w := controls[0][3]
	want := [4]*big.Rat{nil, nil, big.NewRat(z, 1), big.NewRat(1, 1)}
	want[axis], want[d] = big.NewRat(1, 2), big.NewRat(3, 8)
	for k := range want {
		if new(big.Rat).Quo(controls[0][k], w).Cmp(want[k]) != 0 {
			t.Fatal("original ray endpoint changed")
		}
	}
	first := func(k int) *big.Rat {
		v := new(big.Rat).Sub(controls[1][k], new(big.Rat).Mul(want[k], controls[1][3]))
		return v.Mul(v, big.NewRat(2, 1)).Quo(v, w)
	}
	df := first(axis)
	if df.Sign() != 2*direction-1 || first(d).Sign() != 0 || first(2).Sign() != 0 {
		t.Fatal("changed original free or dependent tangent direction")
	}
	homogeneousSecond := func(k int) *big.Rat {
		v := new(big.Rat).Sub(controls[2][k], new(big.Rat).Mul(big.NewRat(2, 1), controls[1][k]))
		return v.Add(v, controls[0][k]).Mul(v, big.NewRat(2, 1))
	}
	n := new(big.Rat).Sub(homogeneousSecond(d), new(big.Rat).Mul(want[d], homogeneousSecond(3)))
	scale := new(big.Rat).Mul(df, df)
	scale.Mul(scale, w)
	if n.Quo(n, scale).Cmp(second) != 0 {
		t.Fatal("changed the curvature ordering at the original tangent node")
	}
}

func checkApexTransferBridge(t *testing.T, b trimmed.ExactBezierBoundary, h trimmed.ParabolaCircleApexEnvelopeHistory, axis, side int) {
	t.Helper()
	edge := b.Edges[h.Bridge]
	y := [2]float64{1. / 8, 0}
	if side == 1 {
		y = [2]float64{5. / 8, 1}
	}
	for _, id := range [2]int{edge.Start, edge.End} {
		p := b.Vertices[id]
		xy := [2]float64{p.X, p.Y}
		if xy[axis] != .5 || !slices.Contains(y[:], xy[1-axis]) {
			t.Fatal("bridge endpoint left the original disk extremum or wall station", p)
		}
	}
	for _, p := range edge.Model.Control {
		if new(big.Rat).Quo(p[axis], p[3]).Cmp(big.NewRat(1, 2)) != 0 {
			t.Fatal("bridge lost its original free station")
		}
	}
	first, second := h.RoofFaces[side], h.RoofFaces[3]
	if b.Faces[first].Spline == nil || b.Faces[first].Spline != b.Faces[second].Spline {
		t.Fatal("bridge lost its common original enclosing roof")
	}
	uses := 0
	for i, face := range b.Faces {
		for _, loop := range face.Loops {
			for _, use := range loop {
				if use.Edge != h.Bridge {
					continue
				}
				if i != first && i != second {
					t.Fatal("bridge migrated off the enclosing roof")
				}
				uses++
			}
		}
	}
	if uses != 2 {
		t.Fatal("lost one of the original enclosing roof bridge uses")
	}
}
