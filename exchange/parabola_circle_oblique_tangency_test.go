package exchange_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"os"
	"slices"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange/iges"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func TestParabolaCircleObliqueTangencyEnvelopeExchange(t *testing.T) {
	ctx := context.Background()
	const unit = .001
	proof := edgecheck.Options{}
	for _, c := range []struct {
		op                string
		axis, side, order int
		want              float64
	}{
		{"common", 0, 0, 0, 2000370691.0/1006632960 - 36148125.0*math.Pi/35184372088832},
		{"common", 1, 1, 1, 13936357861.0/7046430720 - 35851875.0*math.Pi/35184372088832},
		{"fuse", 0, 1, 0, 14175219227.0/7046430720 + 35851875.0*math.Pi/35184372088832},
		{"fuse", 1, 0, 1, 2034000893.0/1006632960 + 36148125.0*math.Pi/35184372088832},
	} {
		t.Run(fmt.Sprintf("%s/%d/%d", c.op, c.axis, c.side), func(t *testing.T) {
			raw := obliqueTangencyExchangeSource(t, c.op, c.axis, c.side, c.order)
			r, err := trimmed.ReadParabolaCircleObliqueTangencyEnvelopeJSON(ctx, bytes.NewReader(raw), trimmed.JSONLimits{}, trimmed.ParabolaCircleGraphOptions{})
			if err != nil {
				t.Fatal(err)
			}
			s, h := r.Boundary(), r.History()
			exact, err := s.ExtractExactBezierBoundary(ctx, proof)
			if err != nil {
				t.Fatal(err)
			}
			checkObliqueTransferVertex(t, exact.Vertices[h.Vertex])
			for branch, rays := range h.Rays {
				for direction, id := range rays {
					checkObliqueTransferRay(t, exact, id, h.Vertex, c.axis, c.side, branch, direction)
				}
			}
			checkObliqueTransferBridge(t, exact, h, c.axis, c.side)
			var st, ig bytes.Buffer
			if err := step.WriteParabolaCircleObliqueTangencyEnvelope(ctx, &st, r, c.op, unit, step.Limits{}, proof); err != nil {
				t.Fatal(err)
			}
			if err := iges.WriteParabolaCircleObliqueTangencyEnvelope(ctx, &ig, r, unit, iges.Limits{}, proof); err != nil {
				t.Fatal(err)
			}
			if err := step.VerifyParabolaCircleObliqueTangencyEnvelopeSTEP(ctx, bytes.NewReader(st.Bytes()), r, c.op, unit, step.Limits{}, proof); err != nil {
				t.Fatal(err)
			}
			if err := iges.VerifyParabolaCircleObliqueTangencyEnvelopeIGES(ctx, bytes.NewReader(ig.Bytes()), r, unit, iges.Limits{}, proof); err != nil {
				t.Fatal(err)
			}
			if c.op == "common" && c.axis == 0 {
				var blocked bytes.Buffer
				short := edgecheck.Options{MaxWork: 1}
				if err := step.WriteParabolaCircleObliqueTangencyEnvelope(ctx, &blocked, r, c.op, unit, step.Limits{}, short); !errors.Is(err, geom.ErrConvergence) || blocked.Len() != 0 {
					t.Fatal("STEP published an incompletely proved bridge", err)
				}
				if err := iges.WriteParabolaCircleObliqueTangencyEnvelope(ctx, &blocked, r, unit, iges.Limits{}, short); !errors.Is(err, geom.ErrConvergence) || blocked.Len() != 0 {
					t.Fatal("IGES published an incompletely proved bridge", err)
				}
			}
			checkTwoCirclesExchangeReadback(t, ctx, []trimmed.Solid{s}, []float64{c.want}, st.Bytes(), ig.Bytes())
		})
	}
}

func TestParabolaCircleObliqueTangencyCutExchange(t *testing.T) {
	ctx := context.Background()
	const name, unit = "Parabola-circle nonapex tangent Cut", .001
	proof := edgecheck.Options{}
	volumes := [2][]float64{
		{20734973.0 / 1006632960, 36148125.0 * math.Pi / 35184372088832},
		{156503579.0 / 7046430720, 35851875.0 * math.Pi / 35184372088832},
	}
	for axis := range 2 {
		for side, want := range volumes {
			t.Run(fmt.Sprintf("%d/%d", axis, side), func(t *testing.T) {
				raw := obliqueTangencyExchangeSource(t, "cut", axis, side, 1-side)
				r, err := trimmed.ReadParabolaCircleObliqueTangencyCutJSON(ctx, bytes.NewReader(raw), trimmed.JSONLimits{}, trimmed.ParabolaCircleGraphOptions{})
				if err != nil {
					t.Fatal(err)
				}
				parts, contact := r.Components(), r.Contact()
				wallControls := 0
				for part, s := range parts {
					exact, err := s.ExtractExactBezierBoundary(ctx, proof)
					if err != nil {
						t.Fatal(err)
					}
					checkObliqueTransferVertex(t, exact.Vertices[contact.Vertices[part]])
					for direction, id := range contact.Rays[part] {
						checkObliqueTransferRay(t, exact, id, contact.Vertices[part], axis, side, part, direction)
					}
					if axis == 0 && side == 1 && part == 0 {
						wallControls = countTwoCirclesExchangeControls(t, exact)
					}
				}
				var st, ig bytes.Buffer
				if err := step.WriteParabolaCircleObliqueTangencyCut(ctx, &st, r, name, unit, step.Limits{}, proof); err != nil {
					t.Fatal(err)
				}
				if err := iges.WriteParabolaCircleObliqueTangencyCut(ctx, &ig, r, unit, iges.Limits{}, proof); err != nil {
					t.Fatal(err)
				}
				if err := step.VerifyParabolaCircleObliqueTangencyCutSTEP(ctx, bytes.NewReader(st.Bytes()), r, name, unit, step.Limits{}, proof); err != nil {
					t.Fatal(err)
				}
				if err := iges.VerifyParabolaCircleObliqueTangencyCutIGES(ctx, bytes.NewReader(ig.Bytes()), r, unit, iges.Limits{}, proof); err != nil {
					t.Fatal(err)
				}
				if wallControls > 0 {
					var blocked bytes.Buffer
					if err := step.WriteParabolaCircleObliqueTangencyCut(ctx, &blocked, r, name, unit, step.Limits{MaxSolids: 1}, proof); !errors.Is(err, geom.ErrConvergence) || blocked.Len() != 0 {
						t.Fatal("STEP published a partial source contact pair", err)
					}
					if err := iges.WriteParabolaCircleObliqueTangencyCut(ctx, &blocked, r, unit, iges.Limits{MaxControlPoints: wallControls}, proof); !errors.Is(err, geom.ErrConvergence) || blocked.Len() != 0 {
						t.Fatal("IGES published a partial source contact pair", err)
					}
				}
				// Validate both exact local tangent occurrences, full original
				// vertex and edge incidence, independent indexing and mass.
				checkTwoCirclesExchangeReadback(t, ctx, parts[:], want, st.Bytes(), ig.Bytes())
			})
		}
	}
}

func obliqueTangencyExchangeSource(t *testing.T, op string, axis, side, order int) []byte {
	t.Helper()
	raw, err := os.ReadFile(fmt.Sprintf("testdata/native_parabola_circle_oblique_%s-axis-%d-side-%d-order-%d.json", op, axis, side, order))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func checkObliqueTransferVertex(t *testing.T, p geom.Vec3) {
	t.Helper()
	want := geom.Vec3{X: .5, Y: .5, Z: 2}
	if p != want {
		t.Fatal("displaced tangent vertex", p, want)
	}
}

func checkObliqueTransferRay(t *testing.T, b trimmed.ExactBezierBoundary, id, vertex, axis, side, branch, direction int) {
	t.Helper()
	edge := b.Edges[id]
	if (edge.Start == vertex) == (edge.End == vertex) {
		t.Fatal("tangent edge does not meet the original vertex", id, vertex)
	}
	atEnd := edge.End == vertex
	second := big.NewRat(1, 4)
	if branch == 1 {
		second.SetInt64(int64(50*side - 25))
	}
	checkObliqueTransferJet(t, edge.Model, atEnd, axis, direction, 2, second)
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
				checkObliqueTransferJet(t, use.UV, atEnd, axis, direction, 0, second)
			}
		}
	}
	if len(owners) != 2 || forward != 1 {
		t.Fatal("lost both opposed original tangent coedges", id)
	}
}

func checkObliqueTransferJet(t *testing.T, span geom.ExactBezierSpan, atEnd bool, axis, direction int, z int64, second *big.Rat) {
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
	want[axis], want[d] = big.NewRat(1, 2), big.NewRat(1, 2)
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
	slope := big.NewRat(3, 4)
	if df.Sign() != 2*direction-1 || first(d).Cmp(new(big.Rat).Mul(df, slope)) != 0 || first(2).Sign() != 0 {
		t.Fatal("changed original free or dependent tangent direction")
	}
	homogeneousSecond := func(k int) *big.Rat {
		v := new(big.Rat).Sub(controls[2][k], new(big.Rat).Mul(big.NewRat(2, 1), controls[1][k]))
		return v.Add(v, controls[0][k]).Mul(v, big.NewRat(2, 1))
	}
	dSecond := func(k int) *big.Rat {
		return new(big.Rat).Sub(homogeneousSecond(k), new(big.Rat).Mul(want[k], homogeneousSecond(3)))
	}
	// Weight-derivative terms cancel in the free-coordinate second derivative.
	n := new(big.Rat).Sub(dSecond(d), new(big.Rat).Mul(slope, dSecond(axis)))
	scale := new(big.Rat).Mul(df, df)
	scale.Mul(scale, w)
	if n.Quo(n, scale).Cmp(second) != 0 {
		t.Fatal("changed the curvature ordering at the original tangent node")
	}
}

func checkObliqueTransferBridge(t *testing.T, b trimmed.ExactBezierBoundary, h trimmed.ParabolaCircleObliqueEnvelopeHistory, axis, side int) {
	t.Helper()
	edge := b.Edges[h.Bridge]
	farY, wallY := float64(3+2*side)/8, float64(side)
	normalFree := func(y float64) float64 { return .5 - .75*(y-.5) }
	endpoints := map[[2]float64]bool{{normalFree(farY), farY}: true, {normalFree(wallY), wallY}: true}
	for _, id := range [2]int{edge.Start, edge.End} {
		p := b.Vertices[id]
		xy := [2]float64{p.X, p.Y}
		q := [2]float64{xy[axis], xy[1-axis]}
		if !endpoints[q] {
			t.Fatal("bridge endpoint left the original disk extremum or wall station", p)
		}
		delete(endpoints, q)
	}
	for _, p := range edge.Model.Control {
		x := new(big.Rat).Quo(p[axis], p[3])
		y := new(big.Rat).Quo(p[1-axis], p[3])
		normal := new(big.Rat).Add(x, new(big.Rat).Mul(big.NewRat(3, 4), y))
		if normal.Cmp(big.NewRat(7, 8)) != 0 {
			t.Fatal("bridge lost its exact original normal line")
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
