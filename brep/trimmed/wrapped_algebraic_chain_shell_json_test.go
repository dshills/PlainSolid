package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
)

func wrappedChainJSONFixture(t *testing.T) (*WrappedAlgebraicChainShell, []byte) {
	t.Helper()
	ctx := context.Background()
	ab, bc := wrappedAlgebraicChainFixture(t, false, false)
	p, e := CertifyWrappedAlgebraicChainEmbedding(ctx, ab, bc, geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}})
	if e != nil {
		t.Fatal(e)
	}
	s, e := NewWrappedAlgebraicChainShell(ctx, p, edgecheck.Options{})
	if e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e := WriteWrappedAlgebraicChainShellJSON(ctx, &out, s, WrappedAlgebraicChainJSONOptions{}); e != nil {
		t.Fatal(e)
	}
	return s, out.Bytes()
}
func TestWrappedAlgebraicChainJSON(t *testing.T) {
	ctx := context.Background()
	_, raw := wrappedChainJSONFixture(t)
	s, e := ReadWrappedAlgebraicChainShellJSON(ctx, bytes.NewReader(raw), WrappedAlgebraicChainJSONOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(s.Vertices()) != 8 || len(s.Edges()) != 10 || len(s.Faces()) != 3 {
		t.Fatal("native topology")
	}
	native := Solid{vertices: s.Vertices(), edges: s.Edges(), faces: s.Faces(), tolerance: 1e-9}
	r, e := native.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if e != nil || !r.Complete {
		t.Fatal(r, e)
	}
	for i := 0; i < 3; i++ {
		shift, _ := s.Embedding().PeriodShift(i)
		if shift[0].Int64() != int64(i) {
			t.Fatal("original lift")
		}
	}
	p := s.embedding
	piece, _ := p.pairs[0].sources[0].face.algebraicProofs[0][2].Piece(0)
	root, _ := piece.Model.EndpointRoot(1)
	kind, e := root.Classify(ctx, geom.ExactCurvePlaneEventOptions{})
	if e != nil || kind.Kind != geom.ExactRootKindIrrational {
		t.Fatal("irrational seam reproof", e)
	}
	var out bytes.Buffer
	if e := WriteWrappedAlgebraicChainShellJSON(ctx, &out, s, WrappedAlgebraicChainJSONOptions{}); e != nil || !bytes.Equal(raw, out.Bytes()) {
		t.Fatal("deterministic recipe", e)
	}
	used := s.ExactOperations() + p.ExactOperations() + p.pairs[0].ExactOperations() + p.pairs[1].ExactOperations() + p.pairs[0].sources[0].ExactOperations() + p.pairs[0].sources[1].ExactOperations() + p.pairs[1].sources[1].ExactOperations()
	if q, e := ReadWrappedAlgebraicChainShellJSON(ctx, bytes.NewReader(raw), WrappedAlgebraicChainJSONOptions{MaxWork: used - 1}); e == nil || q != nil {
		t.Fatal("cumulative exhaustion")
	}
}
func TestWrappedAlgebraicChainJSONRejects(t *testing.T) {
	ctx := context.Background()
	s, raw := wrappedChainJSONFixture(t)
	mutations := map[string]func(*wrappedChainRecipe){
		"middle curve": func(n *wrappedChainRecipe) {
			var f wrappedAlgebraicFaceRecipe
			json.Unmarshal(n.Faces[1], &f)
			q, _ := new(big.Rat).SetString(f.Loops[0][0][1][1])
			q.Add(q, big.NewRat(1, 16))
			f.Loops[0][0][1][1] = q.RatString()
			n.Faces[1], _ = json.Marshal(f)
		},
		"middle lift": func(n *wrappedChainRecipe) {
			var f wrappedAlgebraicFaceRecipe
			json.Unmarshal(n.Faces[1], &f)
			for _, loop := range f.Loops {
				for _, edge := range loop {
					for _, row := range edge {
						u, _ := new(big.Rat).SetString(row[0])
						w, _ := new(big.Rat).SetString(row[2])
						row[0] = u.Add(u, w.Mul(w, big.NewRat(4, 1))).RatString()
					}
				}
			}
			n.Faces[1], _ = json.Marshal(f)
		},
		"middle orientation": func(n *wrappedChainRecipe) {
			var f wrappedAlgebraicFaceRecipe
			json.Unmarshal(n.Faces[1], &f)
			f.Reversed = !f.Reversed
			n.Faces[1], _ = json.Marshal(f)
		},
		"shift":    func(n *wrappedChainRecipe) { n.PeriodShifts[2][0] = "3" },
		"mate":     func(n *wrappedChainRecipe) { n.Mates[1].Edges[0][1] = 0 },
		"reverse":  func(n *wrappedChainRecipe) { n.Mates[0].ReverseB = false },
		"owner":    func(n *wrappedChainRecipe) { n.Witness.Edges[0].Sources[0][0] = 2 },
		"vertex":   func(n *wrappedChainRecipe) { n.Witness.Vertices[0].Point[0] = "99" },
		"use":      func(n *wrappedChainRecipe) { n.Witness.FaceUses[1][0][1] = 0 },
		"exterior": func(n *wrappedChainRecipe) { n.Witness.Boundary[0][0] = 2 },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			var n wrappedChainRecipe
			if e := json.Unmarshal(raw, &n); e != nil {
				t.Fatal(e)
			}
			mutate(&n)
			if name == "middle lift" {
				if _, e := ReadWrappedAlgebraicQuadraticFaceJSON(ctx, bytes.NewReader(n.Faces[1]), WrappedQuadraticFaceJSONOptions{}); e != nil {
					t.Fatal("changed lift is valid standalone source", e)
				}
			}
			bad, e := json.Marshal(n)
			if e != nil {
				t.Fatal(e)
			}
			if q, e := ReadWrappedAlgebraicChainShellJSON(ctx, bytes.NewReader(bad), WrappedAlgebraicChainJSONOptions{}); q != nil || e == nil {
				t.Fatal("mutation accepted")
			}
		})
	}
	for _, bad := range [][]byte{append(append([]byte(nil), raw...), []byte(" {}")...), bytes.Replace(raw, []byte(`"version":1`), []byte(`"version":1,"version":1`), 1), bytes.Replace(raw, []byte(`"version":1`), []byte(`"version":1,"complete":true`), 1)} {
		if q, e := ReadWrappedAlgebraicChainShellJSON(ctx, bytes.NewReader(bad), WrappedAlgebraicChainJSONOptions{}); q != nil || e == nil {
			t.Fatal("syntax accepted")
		}
	}
	for _, opts := range []WrappedAlgebraicChainJSONOptions{{MaxWork: 1}, {MaxBytes: int64(len(raw) - 1)}} {
		if q, e := ReadWrappedAlgebraicChainShellJSON(ctx, bytes.NewReader(raw), opts); q != nil || e == nil {
			t.Fatal("read limit")
		}
		var out bytes.Buffer
		if e := WriteWrappedAlgebraicChainShellJSON(ctx, &out, s, opts); e == nil || out.Len() != 0 {
			t.Fatal("atomic write")
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if q, e := ReadWrappedAlgebraicChainShellJSON(canceled, bytes.NewReader(raw), WrappedAlgebraicChainJSONOptions{}); q != nil || e == nil {
		t.Fatal("cancel")
	}
	// A stale source recipe cannot conceal changed actual exported geometry.
	bad := *s
	bad.edges = s.Edges()
	curve := bad.edges[0].Curve.(wrappedAlgebraicTrimCurve)
	curve.b.X += .125
	bad.edges[0].Curve = curve
	var out bytes.Buffer
	if e := WriteWrappedAlgebraicChainShellJSON(ctx, &out, &bad, WrappedAlgebraicChainJSONOptions{}); e == nil || out.Len() != 0 {
		t.Fatal("stale native model")
	}
}
