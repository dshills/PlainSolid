package trimmed

import (
	"context"
	"errors"
	"math"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestCertifiedParabolaTwoCirclesFacetAngle(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	source := parabolaTwoCirclesStock(t, 0, twoCirclePrimaryStock(), false)
	for order := range 2 {
		cut, err := CutParabolaTwoCirclesSplineGraphs(ctx, source[0], source[1+order], source[2-order], 0, 1e-9, ParabolaCircleGraphOptions{})
		if err != nil {
			t.Fatal(err)
		}
		request := .1
		if order == 1 {
			request = .051
		}
		opts := ParabolaTwoCirclesMeshOptions{NormalAngle: request, MaxSubdivisions: 16}
		result, err := cut.TessellateCertified(ctx, opts)
		if err != nil || len(result) != [2]int{3, 1}[order] {
			t.Fatal("facet-certified identity assembly", len(result), err)
		}
		for i, m := range result {
			if m.SquaredSourceNormalTurn != nil {
				t.Fatal("facet option emitted an unrequested source-turn proof")
			}
			checkParabolaCircleFacetDisplay(t, cut.components[i], m, request, func(p geom.Vec3) geom.Vec2 {
				return geom.Vec2{X: p.X, Y: p.Y}
			})
		}
		if order == 1 {
			opts.NormalAngle = .05
			if out, err := cut.TessellateCertified(ctx, opts); out != nil || !errors.Is(err, parametric.ErrBudget) {
				t.Fatal("tight facet bound must refuse under the same subdivisions", len(out), err)
			}
			t.Log("genus-two accepted .051 radians and atomically refused .050 at 16 subdivisions")
		}
	}
}

func checkParabolaCircleFacetDisplay(t *testing.T, s Solid, m CertifiedParabolaTwoCirclesComponentMesh,
	request float64, uv func(geom.Vec3) geom.Vec2) {
	t.Helper()
	upper := m.SquaredFacetNormalTangent
	if upper == nil || upper.Sign() <= 0 || upper.Cmp(mappedAngleTangentLimit(request)) > 0 ||
		!m.Collision.Complete || len(m.Collision.Contacts) != 0 {
		t.Fatal("missing exact facet or closure proof", upper, request)
	}
	q, _ := upper.Float64()
	bound := math.Atan(math.Sqrt(q))
	observed := 0.
	walls := 0
	for i, tri := range m.Mesh.Triangles {
		a, b, c := m.Mesh.Vertices[tri[0]], m.Mesh.Vertices[tri[1]], m.Mesh.Vertices[tri[2]]
		facet := b.Sub(a).Cross(c.Sub(a))
		face := s.faces[m.OriginalFace[i]]
		for _, p := range [4]geom.Vec3{a, b, c, a.Add(b).Add(c).Scale(1.0 / 3)} {
			var native geom.Vec3
			switch surface := face.Surface.(type) {
			case geom.PlaneSurface:
				native = surface.Frame().ZAxis()
				walls++
			case parabolaCircleSimilarityWall:
				a, b, err := surface.Partials(0, 0)
				if err != nil {
					t.Fatal(err)
				}
				native = a.Cross(b)
				walls++
			case *geom.BSplineSurface:
				q := uv(p)
				var err error
				native, err = surface.Normal(q.X, q.Y)
				if err != nil {
					t.Fatal(err)
				}
			default:
				t.Fatalf("unexpected original face type %T", surface)
			}
			if face.Reversed {
				native = native.Scale(-1)
			}
			deviation := math.Atan2(native.Cross(facet).Length(), native.Dot(facet))
			if _, wall := face.Surface.(geom.PlaneSurface); wall && deviation != 0 {
				t.Fatal("stock wall facet changed its exact normal", deviation)
			}
			observed = max(observed, deviation)
		}
	}
	if !(observed > 0 && observed <= bound) || (m.Region < 2) != (walls > 0) {
		t.Fatal("original facet alignment check", observed, bound, walls)
	}
	t.Logf("region=%d request=%.3f facet bound=%.12f sampled maximum=%.12f wall samples=%d triangles=%d",
		m.Region, request, bound, observed, walls, len(m.Mesh.Triangles))
}
