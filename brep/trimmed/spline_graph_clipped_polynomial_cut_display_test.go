package trimmed

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"slices"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/collision"
)

func TestNativeClippedPolynomialCutClosedDisplay(t *testing.T) {
	ctx := context.Background()
	opts := parametric.TessellationOptions{Deflection: .05, Angle: .15, MaxSubdivisions: 32, MaxTriangles: 80_000}
	for layout := range 4 {
		for axis := range 2 {
			for profile := range 2 {
				t.Run(fmt.Sprintf("layout%d/axis%d/profile%d", layout, axis, profile), func(t *testing.T) {
					t.Parallel()
					src := clippedPolynomialStock(t, axis, profile, layout%2)
					if layout >= 2 {
						src = twoAxisClippedFuseStock(t, axis, profile, layout-2)
					}
					cut, err := CutClippedPolynomialComponents(ctx, src[0], src[2], src[1], axis, 1e-9,
						PolynomialNodeCutOptions{})
					if err != nil {
						t.Fatal(err)
					}
					display, err := cut.Tessellate(ctx, opts)
					if err != nil {
						t.Fatal(err)
					}
					parts := cut.Components()
					volumes, _ := cut.ExactComponentVolumes()
					contacts := cut.Contacts()
					if len(display) != 3 || len(parts) != len(display) {
						t.Fatalf("three separate components, got %d", len(display))
					}
					var seen [2]bool
					var counts [3]int
					for i, m := range display {
						counts[i] = len(m.Triangles)
						vertices := parts[i].Vertices()
						if len(m.Vertices) < len(vertices) || !slices.Equal(m.Vertices[:len(vertices)], vertices) {
							t.Fatalf("component %d changed the native vertex indices", i)
						}
						if err := m.ValidateSolid(); err != nil {
							t.Fatal(err)
						}
						used := make([]bool, len(m.Vertices))
						for _, tri := range m.Triangles {
							for _, id := range tri {
								used[id] = true
							}
						}
						for id := range vertices {
							if !used[id] {
								t.Fatalf("unused native component %d vertex %d", i, id)
							}
						}
						volume := m.SignedVolume()
						if volume <= 0 {
							t.Fatalf("component %d display has no positive oriented volume: %g", i, volume)
						}
						if len(cut.Chambers(i)) > 1 {
							want, _ := volumes[i].Float64()
							if math.Abs(volume-want) > .003*want {
								t.Fatalf("strip-bearing component volume %g, native %g", volume, want)
							}
						}
						for j := i + 1; j < len(display); j++ {
							report, e := collision.MeshIntersections(m, display[j], collision.Options{})
							if e != nil || !report.Complete {
								t.Fatalf("component %d/%d intersection scan: %v", i, j, e)
							}
							for _, hit := range report.Contacts {
								accepted := false
								for c, contact := range contacts {
									a, b := contact.Vertices[0], contact.Vertices[1]
									if contact.Components == [2]int{i, j} && hit.Kind == collision.Touching &&
										len(hit.Points) == 1 && hit.Points[0] == m.Vertices[a] &&
										slices.Contains(m.Triangles[hit.TriangleA][:], a) &&
										slices.Contains(display[j].Triangles[hit.TriangleB][:], b) {
										seen[c], accepted = true, true
									}
								}
								if !accepted {
									t.Fatalf("component %d/%d has an undeclared contact: %+v", i, j, hit)
								}
							}
						}
					}
					if seen != [2]bool{true, true} {
						t.Fatalf("missing an original point contact: %v", seen)
					}
					t.Logf("component triangles %v, both original point contacts realized", counts)
				})
			}
		}
	}
}

func TestNativeClippedPolynomialCutDisplayLimitsAndReload(t *testing.T) {
	ctx := context.Background()
	src := twoAxisClippedFuseStock(t, 1, 1, 1)
	sourceOpts := PolynomialNodeCutOptions{}
	cut, err := CutClippedPolynomialComponents(ctx, src[0], src[2], src[1], 1, 1e-9, sourceOpts)
	if err != nil {
		t.Fatal(err)
	}
	opts := parametric.TessellationOptions{Deflection: .05, Angle: .15, MaxSubdivisions: 32, MaxTriangles: 80_000}
	m, err := cut.Tessellate(ctx, opts)
	if err != nil {
		t.Fatal(err)
	}
	total := 0
	for _, component := range m {
		total += len(component.Triangles)
	}
	for _, o := range []parametric.TessellationOptions{
		{Deflection: .05, Angle: .15, MaxSubdivisions: 32, MaxTriangles: total - 1},
		{Deflection: .05, Angle: .15, MaxSubdivisions: 1, MaxTriangles: 80_000},
	} {
		if partial, err := cut.Tessellate(ctx, o); partial != nil || !errors.Is(err, parametric.ErrBudget) {
			t.Fatalf("Cut exhausted the display budget: %+v %v", o, err)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if partial, err := cut.Tessellate(cancelled, opts); partial != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled Cut display: %v", err)
	}
	analytic := opts
	analytic.AnalyticBound = true
	if partial, err := cut.Tessellate(ctx, analytic); partial != nil || !errors.Is(err, parametric.ErrDeflectionUnsupported) {
		t.Fatalf("Cut display requires sampled deflection: %v", err)
	}

	var record bytes.Buffer
	if err := cut.WriteJSON(ctx, &record); err != nil {
		t.Fatal(err)
	}
	reloaded, err := ReadClippedPolynomialCutJSON(ctx, bytes.NewReader(record.Bytes()), JSONLimits{}, sourceOpts)
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := reloaded.Tessellate(ctx, opts)
	if err != nil || !reflect.DeepEqual(m, replayed) || !reflect.DeepEqual(cut.Contacts(), reloaded.Contacts()) {
		t.Fatalf("source replay changed independent display meshes or contacts: %v", err)
	}
}
