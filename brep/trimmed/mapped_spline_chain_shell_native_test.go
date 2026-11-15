package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"strings"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestMappedSplineChainNativeShell(t *testing.T) {
	ctx := context.Background()
	boundary, closing, planar, seams := mappedChainClosedFixture(t)
	reversed := [2]bool{false, true}

	// One original closing owner has a different degree and exact, nonbinary
	// interior controls. Its complete model equality must survive reload.
	ends, _ := closing[0][0].(ExactQuadraticUV).Endpoints()
	cubic := make([][3]*big.Rat, 4)
	for i := range cubic {
		for k := range ends[0] {
			a := new(big.Rat).Mul(ends[0][k], big.NewRat(int64(3-i), 3))
			b := new(big.Rat).Mul(ends[1][k], big.NewRat(int64(i), 3))
			cubic[i][k] = new(big.Rat).Add(a, b)
		}
		cubic[i][2] = big.NewRat(1, 1)
	}
	var err error
	closing[0][0], err = NewExactBezierUV(cubic)
	if err != nil {
		t.Fatal(err)
	}
	original, err := CertifyMappedSplineChainShell(ctx, boundary, closing, reversed, planar, seams, MappedChainShellProofOptions{})
	if err != nil {
		t.Fatal(err)
	}
	want := MappedChainShellTopology{Faces: 7, Edges: 15, Vertices: 10, EulerCharacteristic: 2}
	if original.Topology() != want {
		t.Fatal("original analytic incidence", original.Topology())
	}
	// Ownership is copied before the caller relinquishes these recipe slices.
	closing[0][0] = nil
	planar[0].Outer[0] = nil
	seams[0][0].Trim = 99

	var encoded bytes.Buffer
	opts := MappedChainShellNativeOptions{}
	if err := original.WriteJSON(ctx, &encoded, opts); err != nil {
		t.Fatal(err)
	}
	t.Logf("native source recipe bytes=%d topology=%+v", encoded.Len(), original.Topology())
	loaded, err := ReadMappedSplineChainShellJSON(ctx, bytes.NewReader(encoded.Bytes()), opts)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Topology() != want {
		t.Fatal("reloaded source incidence", loaded.Topology())
	}
	h := loaded.closing[0][0].(ExactBezierUV).HomogeneousControls()
	for j, row := range h {
		for k, coefficient := range row {
			if coefficient.Cmp(cubic[j][k]) != 0 {
				t.Fatal("original exact cubic coefficient was changed", j, k)
			}
		}
	}
	var replay bytes.Buffer
	if err := loaded.WriteJSON(ctx, &replay, opts); err != nil || !bytes.Equal(encoded.Bytes(), replay.Bytes()) {
		t.Fatal("source round-trip differs", err)
	}
	if _, err := ReadJSON(bytes.NewReader(encoded.Bytes()), JSONLimits{}); err == nil {
		t.Fatal("source shell recipe accepted into generic Solid")
	}

	face := MixedExactMultiSpanMeshOptions{Face: MixedExactCurvedMeshOptions{
		BoundaryModelBudget: .005,
		Interior:            CurvedQuadraticTrimMeshOptions{Deflection: .04},
	}}
	display, err := loaded.TessellateClosedCertified(ctx, nil, MappedChainClosedMeshOptions{
		Boundary: MappedChainPolylineOptions{Deflection: .005},
		Curved:   [2]MixedExactMultiSpanMeshOptions{face, face},
	})
	if err != nil || !display.DisplayCollision.Complete || len(display.DisplayCollision.Contacts) != 0 || len(display.ChainSamples) != 33 {
		t.Fatal("could not independently display the reloaded source recipe", err)
	}

	warpedFloor := func() json.RawMessage {
		poles := make([]geom.Vec3, 0, 9)
		for u := range 3 {
			for v := range 3 {
				z := -1.0
				if u == 1 && v == 1 {
					z = -.875
				}
				poles = append(poles, geom.Vec3{X: float64(u) / 2, Y: float64(v) * .375, Z: z})
			}
		}
		patch, err := geom.NewBSplineSurface(2, 2, 3, 3, poles, []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 0, .75, .75, .75}, nil)
		if err != nil {
			t.Fatal(err)
		}
		wire, err := parametric.EncodeSurface(patch)
		if err != nil {
			t.Fatal(err)
		}
		return wire
	}
	cases := []struct {
		name   string
		mutate func(*nativeMappedChainShell)
		err    string
	}{
		{"missing seam", func(n *nativeMappedChainShell) { n.Seams = n.Seams[1:] }, "invalid"},
		{"extra owner", func(n *nativeMappedChainShell) { n.Seams[0] = append(n.Seams[0], n.Seams[0][1]) }, "invalid"},
		{"reused trim", func(n *nativeMappedChainShell) { n.Seams[1][0] = n.Seams[0][0] }, "invalid"},
		{"orientation", func(n *nativeMappedChainShell) { n.Planar[0].Reversed = true }, "closing seam 0 agreement"},
		{"interior trim change", func(n *nativeMappedChainShell) { n.Planar[0].Outer[2].Controls[1][1] = "7/8" }, "closing seam 0 agreement"},
		{"nonaffine floor with identical boundary", func(n *nativeMappedChainShell) { n.Planar[4].Surface = warpedFloor() }, "source face 6 affine support"},
		{"broken original loop", func(n *nativeMappedChainShell) {
			loop := n.Curved[0].Outer
			loop[0], loop[1] = loop[1], loop[0]
			n.Seams[0][0].Trim, n.Seams[1][0].Trim = n.Seams[1][0].Trim, n.Seams[0][0].Trim
		}, "source face 0 outer loop"},
		{"wrong shape of exact controls", func(n *nativeMappedChainShell) {
			control := n.Planar[0].Outer[2].Controls[0]
			n.Planar[0].Outer[2].Controls[0] = append(control, "1")
		}, "source face 2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var record nativeMappedChainShell
			if err := json.Unmarshal(encoded.Bytes(), &record); err != nil {
				t.Fatal(err)
			}
			tc.mutate(&record)
			raw, err := json.Marshal(record)
			if err != nil {
				t.Fatal(err)
			}
			rejected, err := ReadMappedSplineChainShellJSON(ctx, bytes.NewReader(raw), opts)
			if err == nil || !strings.Contains(err.Error(), tc.err) || rejected.Topology().Faces != 0 {
				t.Fatal("expected atomic source reproof failure", err)
			}
		})
	}
	for _, input := range [][]byte{
		append([]byte(`{"DisplayMesh":null,`), encoded.Bytes()[1:]...),
		append(append([]byte(nil), encoded.Bytes()...), []byte(" {}")...),
	} {
		if out, err := ReadMappedSplineChainShellJSON(ctx, bytes.NewReader(input), opts); err == nil || out.Topology().Faces != 0 {
			t.Fatal("non-source payload accepted")
		}
	}
	for _, limited := range []MappedChainShellNativeOptions{
		{MaxBytes: int64(encoded.Len() - 1)},
		{Chain: edgecheck.Options{MaxWork: 1}},
		{Shell: MappedChainShellProofOptions{Closing: edgecheck.Options{MaxWork: 1}}},
	} {
		if out, err := ReadMappedSplineChainShellJSON(ctx, bytes.NewReader(encoded.Bytes()), limited); err == nil || out.Topology().Faces != 0 {
			t.Fatal("source read quota")
		}
		var dst bytes.Buffer
		if err := original.WriteJSON(ctx, &dst, limited); err == nil || dst.Len() != 0 {
			t.Fatal("source write quota")
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if out, err := ReadMappedSplineChainShellJSON(canceled, bytes.NewReader(encoded.Bytes()), opts); !errors.Is(err, context.Canceled) || out.Topology().Faces != 0 {
		t.Fatal("source read cancellation", err)
	}
	var stale = original
	stale.seams = stale.seams[1:]
	var dst bytes.Buffer
	if err := stale.WriteJSON(ctx, &dst, opts); err == nil || dst.Len() != 0 {
		t.Fatal("stale source was written")
	}
	if err := original.WriteJSON(ctx, shortMappedChainShellWriter{}, opts); !errors.Is(err, io.ErrShortWrite) {
		t.Fatal("short source write", err)
	}
}

type shortMappedChainShellWriter struct{}

func (shortMappedChainShellWriter) Write(data []byte) (int, error) { return len(data) - 1, nil }
