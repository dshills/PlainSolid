package trimmed

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestSubdivideClippedCommonLens(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	for axis := range 2 {
		for profile := range 2 {
			for layout := range 2 {
				s := clippedPolynomialStock(t, axis, profile, layout)
				for order := range 2 {
					t.Run(fmt.Sprintf("axis%d/profile%d/layout%d/order%d", axis, profile, layout, order), func(t *testing.T) {
						r, err := CommonClippedPolynomialEnvelope(ctx, s[0], s[1+order], s[2-order], axis, 1e-9, PolynomialCubicCrossingTangencyEnvelopeOptions{})
						if err != nil {
							t.Fatal(err)
						}
						split, err := r.SubdivideClippedCommonLens(ctx, geom.GraphPointContactOptions{})
						if err != nil {
							t.Fatal(err)
						}
						checkClippedCommonLensSplit(t, r, split)
						a, v := clippedCommonMeasures(axis, profile)
						m, err := split.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
						if err != nil || math.Abs(m.Area-a) > 1e-8 || math.Abs(m.Volume-v) > 1e-8 {
							t.Fatal("split changes independent original physical measures", m.Area, a, m.Volume, v, err)
						}
					})
				}
			}
		}
	}
}

func checkClippedCommonLensSplit(t *testing.T, r *AlignedPolynomialEnvelope, split Solid) {
	t.Helper()
	before := r.Boundary()
	after := split.Topology()
	seam := len(before.edges)
	lens := r.history.RoofFaces[2]
	otherHalf := len(before.faces)
	if len(after.Shells) != 1 || after.EulerCharacteristic != 2 || after.CollapsedEdges != 0 ||
		len(split.edges) != seam+1 || len(split.faces) != otherHalf+1 ||
		!reflect.DeepEqual(before.vertices, split.vertices) || !reflect.DeepEqual(before.edges, split.edges[:seam]) {
		t.Fatal("the split preserves all native vertex and edge identities", after)
	}
	for i, f := range before.faces {
		if i != lens && !reflect.DeepEqual(f, split.faces[i]) {
			t.Fatal("split changes a face outside the original lens", i)
		}
	}

	hits := [2]int{}
	for half, id := range [2]int{lens, otherHalf} {
		f := split.faces[id]
		original := before.faces[lens]
		if f.Surface != original.Surface || f.Reversed != original.Reversed || len(f.Outer) != 3 || len(f.Holes) != 0 {
			t.Fatal("each half retains its original owning roof and orientation", half)
		}
		for i, u := range f.Outer {
			if u.Edge == seam {
				if i != 2 || u.Curve != nil {
					t.Fatal("new coedge is an affine path on the original chart", half, u)
				}
				if u.Reversed {
					hits[1]++
				} else {
					hits[0]++
				}
				continue
			}
			found := false
			for _, old := range original.Outer {
				found = found || reflect.DeepEqual(old, u)
			}
			if !found {
				t.Fatal("split rewrites an original coedge", half, u)
			}
		}
		edge := split.edges[f.Outer[1].Edge]
		start := edge.Start
		if f.Outer[1].Reversed {
			start = edge.End
		}
		if start != r.contacts[half].Vertex {
			t.Fatal("each half retains its own original contact vertex", half, start)
		}
	}

	e := split.edges[seam]
	if hits != ([2]int{1, 1}) || [2]int{e.Start, e.End} != r.history.Midpoints {
		t.Fatal("the seam uses the two original midpoint indices and opposite coedges", e, hits)
	}
}

func TestSubdivideClippedCommonLensReplayAndRefusal(t *testing.T) {
	ctx := context.Background()
	s := clippedPolynomialStock(t, 1, 0, 0)
	opts := PolynomialCubicCrossingTangencyEnvelopeOptions{}
	proof := geom.GraphPointContactOptions{}
	r, err := CommonClippedPolynomialEnvelope(ctx, s[0], s[2], s[1], 1, 1e-9, opts)
	if err != nil {
		t.Fatal(err)
	}
	var before, after bytes.Buffer
	if err := r.WriteJSON(ctx, &before); err != nil {
		t.Fatal(err)
	}
	b, err := r.SubdivideClippedCommonLens(ctx, proof)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.WriteJSON(ctx, &after); err != nil || !bytes.Equal(before.Bytes(), after.Bytes()) {
		t.Fatal("portable split changes original native recipe", err)
	}
	reloaded, err := ReadClippedPolynomialCommonJSON(ctx, &before, JSONLimits{}, opts)
	if err != nil {
		t.Fatal(err)
	}
	rebuilt, err := reloaded.SubdivideClippedCommonLens(ctx, proof)
	if err != nil {
		t.Fatal(err)
	}
	x, err := b.ExtractExactBezierBoundary(ctx, edgecheck.Options{})
	if err != nil {
		t.Fatal(err)
	}
	y, err := rebuilt.ExtractExactBezierBoundary(ctx, edgecheck.Options{})
	if err != nil || !reflect.DeepEqual(x, y) {
		t.Fatal("replayed source changes an exported control or incidence", err)
	}

	_, a, _ := nativeWideTwoPolynomialNodeFixture(t, 1)
	transverse := clippedTransverseStock(t, 1, 1, a)
	tr, err := CommonClippedPolynomialEnvelope(ctx, transverse[0], transverse[1], transverse[2], 1, 1e-9, opts)
	if err != nil {
		t.Fatal(err)
	}
	if out, err := tr.SubdivideClippedCommonLens(ctx, proof); !errors.Is(err, geom.ErrDomain) || len(out.faces) != 0 {
		t.Fatal("transverse source was sent to cusp subdivision", err)
	}
	if out, err := r.SubdivideClippedCommonLens(ctx, geom.GraphPointContactOptions{MaxOperations: 1}); err == nil || len(out.faces) != 0 {
		t.Fatal("limited proof publishes a partially checked boundary", err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if out, err := r.SubdivideClippedCommonLens(cancelled, proof); !errors.Is(err, context.Canceled) || len(out.faces) != 0 {
		t.Fatal("cancelled proof publishes a boundary", err)
	}
}
