package trimmed

import (
	"context"
	"errors"
	"math"
	"slices"
	"testing"

	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func explodeExactSewSource(source Solid) ([]geom.Vec3, []parametric.Edge, []Face) {
	faces := source.Faces()
	var vertices []geom.Vec3
	var edges []parametric.Edge
	for i := range faces {
		local := make(map[int]int)
		for _, wire := range append([][]Use{faces[i].Outer}, faces[i].Holes...) {
			for j := range wire {
				original := source.edges[wire[j].Edge]
				ends := [2]int{}
				for k, v := range [2]int{original.Start, original.End} {
					id, exists := local[v]
					if !exists {
						id = len(vertices)
						vertices = append(vertices, source.vertices[v])
						local[v] = id
					}
					ends[k] = id
				}
				original.Start, original.End = ends[0], ends[1]
				wire[j].Edge = len(edges)
				edges = append(edges, original)
			}
		}
	}
	return vertices, edges, faces
}

func reparameterizeExactSewTestEdge(t *testing.T, edge parametric.Edge) parametric.Edge {
	t.Helper()
	c := edge.Curve.(*geom.BSpline)
	p, k, w := c.Poles(), c.Knots(), c.Weights()
	slices.Reverse(p)
	slices.Reverse(k)
	slices.Reverse(w)
	for i := range k {
		k[i] = 10 - 2*k[i]
	}
	reversed, err := geom.NewBSpline(c.Degree(), p, k, w)
	if err != nil {
		t.Fatal(err)
	}
	return parametric.Edge{Start: edge.End, End: edge.Start, Curve: reversed,
		Range: parametric.Interval{Min: 10 - 2*edge.Range.Max, Max: 10 - 2*edge.Range.Min}}
}

func TestExactSewWeightedStation(t *testing.T) {
	ctx := context.Background()
	original := stationMassFixture(t, true, -4)
	vertices, edges, faces := explodeExactSewSource(original)
	if len(edges) <= len(original.edges) || len(vertices) <= len(original.vertices) {
		t.Fatal("fixture must begin as independent faces")
	}
	k := len(faces) - 1
	changed := faces[k].Outer[0].Edge
	edges[changed] = reparameterizeExactSewTestEdge(t, edges[changed])
	faces[k].Outer[0].Reversed = !faces[k].Outer[0].Reversed
	faces[k].Reversed = !faces[k].Reversed
	if denied, report, err := SewExactTopology(ctx, vertices, edges, faces, ExactSewOptions{}); !errors.Is(err, geom.ErrConvergence) || len(denied.faces) != 0 || len(report.Edges) != 0 {
		t.Fatal("inconsistent orientation must reject without repair", err)
	}
	o := ExactSewOptions{RepairOrientation: true}
	sewn, report, err := SewExactTopology(ctx, vertices, edges, faces, o)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(report.FlippedFaces, []int{k}) || !report.Edges[changed].Reversed || len(sewn.edges) != len(original.edges) || len(sewn.vertices) != len(original.vertices) {
		t.Fatal("original source mapping or orientation", report, sewn.Topology())
	}
	for i, sourceEdge := range edges {
		entry := report.Edges[i]
		canonical := sewn.edges[entry.Edge]
		if entry.Reversed {
			canonical.Start, canonical.End = canonical.End, canonical.Start
		}
		if canonical.Start != report.Vertices[sourceEdge.Start] || canonical.End != report.Vertices[sourceEdge.End] {
			t.Fatal("sewn original source endpoints", i, entry)
		}
	}
	for i, f := range faces {
		if sewn.faces[i].Surface != f.Surface {
			t.Fatal("replaced original supporting surface", i)
		}
		for j, use := range f.Outer {
			current := sewn.faces[i].Outer[j]
			if current.UVStart != use.UVStart || current.UVEnd != use.UVEnd || current.Curve != use.Curve {
				t.Fatal("replaced original p-curve", i, j)
			}
		}
	}
	proof, err := sewn.ValidateWrappedRationalStationLoft(ctx, geom.SurfacePatchOptions{})
	if err != nil || !proof.Complete || len(sewn.Topology().Shells) != 1 {
		t.Fatal("sewn output failed independent source embedding reproof", proof, err)
	}
	o.MaxComparisons = report.Comparisons
	if _, check, err := SewExactTopology(ctx, vertices, edges, faces, o); err != nil || check.Comparisons != o.MaxComparisons {
		t.Fatal("exact search budget", err, check.Comparisons)
	}
	o.MaxComparisons--
	if denied, check, err := SewExactTopology(ctx, vertices, edges, faces, o); !errors.Is(err, geom.ErrConvergence) || len(denied.faces) != 0 || len(check.Edges) != 0 {
		t.Fatal("one-short comparison budget returned geometry", err)
	}
}

func TestExactSewRejectsGapsAndInteriorDrift(t *testing.T) {
	ctx := context.Background()
	body := stationMassFixture(t, false, 0)
	for _, name := range []string{"missing cap", "same endpoints with changed edge interior", "one ulp endpoint"} {
		t.Run(name, func(t *testing.T) {
			source := body
			if name == "missing cap" {
				source.faces = source.faces[:len(source.faces)-1]
			}
			vertices, edges, faces := explodeExactSewSource(source)
			if name == "same endpoints with changed edge interior" {
				curve := edges[0].Curve.(*geom.BSpline)
				weights := curve.Weights()
				weights[1] = .5
				altered, err := geom.NewBSpline(curve.Degree(), curve.Poles(), curve.Knots(), weights)
				if err != nil {
					t.Fatal(err)
				}
				edges[0].Curve = altered
			}
			if name == "one ulp endpoint" {
				vertices[0].X = math.Nextafter(vertices[0].X, math.Inf(1))
			}
			if out, report, err := SewExactTopology(ctx, vertices, edges, faces, ExactSewOptions{}); err == nil || len(out.faces) != 0 || len(report.Edges) != 0 {
				t.Fatal("invalid source yielded a sewn shell", err)
			}
		})
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	vertices, edges, faces := explodeExactSewSource(body)
	if out, report, err := SewExactTopology(cancelled, vertices, edges, faces, ExactSewOptions{}); !errors.Is(err, context.Canceled) || len(out.faces) != 0 || len(report.Edges) != 0 {
		t.Fatal("cancelled source yielded a sewn shell", err)
	}
}

func TestExactSewDistinguishesTouchingVertexFans(t *testing.T) {
	ctx := context.Background()
	var components []Solid
	for _, limits := range [][2]geom.Vec3{{{X: -1, Y: -1, Z: -1}, {}}, {{}, {X: 1, Y: 1, Z: 1}}} {
		planar, err := brep.Box(limits[0], limits[1])
		if err != nil {
			t.Fatal(err)
		}
		component, err := FromPlanar(planar, 1e-8)
		if err != nil {
			t.Fatal(err)
		}
		components = append(components, component)
	}
	var vertices []geom.Vec3
	var edges []parametric.Edge
	var faces []Face
	for _, component := range components {
		v, e, f := explodeExactSewSource(component)
		for i := range e {
			e[i].Start += len(vertices)
			e[i].End += len(vertices)
		}
		for i := range f {
			for j := range f[i].Outer {
				f[i].Outer[j].Edge += len(edges)
			}
		}
		vertices = append(vertices, v...)
		edges = append(edges, e...)
		faces = append(faces, f...)
	}
	out, _, err := SewExactTopology(ctx, vertices, edges, faces, ExactSewOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if top := out.Topology(); len(top.Shells) != 2 || top.Vertices != 16 || top.Edges != 24 {
		t.Fatal("coincident independent vertex fans must retain separate identities", top)
	}
}
