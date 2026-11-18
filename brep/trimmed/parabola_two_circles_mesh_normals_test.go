package trimmed

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestCertifiedParabolaTwoCirclesSourceNormalTurn(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	source := parabolaTwoCirclesStock(t, 0, twoCirclePrimaryStock(), false)
	legacyDigest, angularDigest := sha256.New(), sha256.New()
	legacyMesh, angularMesh := json.NewEncoder(legacyDigest), json.NewEncoder(angularDigest)
	const wantLegacy = "852da96df4cd88889f49f0a39a864c82cf584b6f8937492e4284d6ea9f3fd50d"
	for order := range 2 {
		cut, err := CutParabolaTwoCirclesSplineGraphs(ctx, source[0], source[1+order], source[2-order], 0, 1e-9, ParabolaCircleGraphOptions{})
		if err != nil {
			t.Fatal(err)
		}
		plain, err := cut.TessellateCertified(ctx, ParabolaTwoCirclesMeshOptions{})
		if err != nil {
			t.Fatal(err)
		}
		angular, err := cut.TessellateCertified(ctx, ParabolaTwoCirclesMeshOptions{NormalTurn: .2})
		if err != nil || len(angular) != len(plain) {
			t.Fatal("angular assembly", len(angular), err)
		}
		for i, display := range angular {
			if plain[i].SquaredSourceNormalTurn != nil || plain[i].SquaredFacetNormalTangent != nil ||
				display.SquaredFacetNormalTangent != nil {
				t.Fatal("display emitted an unrequested angular certificate")
			}
			if err := legacyMesh.Encode(plain[i].Mesh); err != nil {
				t.Fatal(err)
			}
			if err := angularMesh.Encode(display.Mesh); err != nil {
				t.Fatal(err)
			}
			verifyParabolaTwoCirclesSourceTurn(t, cut.components[i], display, .2)
		}
		if order == 1 {
			checkParabolaTwoCirclesSourceTurnLimit(t, cut)
		}
	}
	for _, digest := range [][]byte{legacyDigest.Sum(nil), angularDigest.Sum(nil)} {
		if fmt.Sprintf("%x", digest) != wantLegacy {
			t.Fatalf("original ordered component mesh bytes changed: %x", digest)
		}
	}
	t.Logf("legacy and .2-radian mesh digest %s", wantLegacy)
}

func checkParabolaTwoCirclesSourceTurnLimit(t *testing.T, cut *ParabolaTwoCirclesGraphCut) {
	t.Helper()
	ctx := context.Background()
	options := ParabolaTwoCirclesMeshOptions{NormalTurn: .102, MaxSubdivisions: 16}
	near, err := cut.TessellateCertified(ctx, options)
	if err != nil || len(near) != 1 || near[0].Subdivisions != 16 {
		t.Fatal("near-limit genus-two display", len(near), err)
	}
	verifyParabolaTwoCirclesSourceTurn(t, cut.components[0], near[0], .102)
	options.NormalTurn = .101
	if got, err := cut.TessellateCertified(ctx, options); got != nil || !errors.Is(err, parametric.ErrBudget) {
		t.Fatal("the same proof budget must refuse the tighter turn", len(got), err)
	}
	t.Log("genus-two .102-radian request passed at 16 subdivisions and .101 refused atomically")
	options.NormalTurn = math.NaN()
	if got, err := cut.TessellateCertified(ctx, options); got != nil || !errors.Is(err, geom.ErrInvalid) {
		t.Fatal("invalid angle", len(got), err)
	}
}

func verifyParabolaTwoCirclesSourceTurn(t *testing.T, s Solid, m CertifiedParabolaTwoCirclesComponentMesh, request float64) {
	t.Helper()
	verifyParabolaTwoCirclesSourceTurnInChart(t, s, m, request, func(p geom.Vec3) geom.Vec2 {
		return geom.Vec2{X: p.X, Y: p.Y}
	})
}

func verifyParabolaTwoCirclesSourceTurnInChart(t *testing.T, s Solid, m CertifiedParabolaTwoCirclesComponentMesh, request float64,
	chart func(geom.Vec3) geom.Vec2) {
	t.Helper()
	limit := ratInset(request)
	limit.Mul(limit, limit)
	if m.SquaredSourceNormalTurn == nil || m.SquaredSourceNormalTurn.Sign() <= 0 ||
		m.SquaredSourceNormalTurn.Cmp(limit) > 0 || !m.Collision.Complete || len(m.Collision.Contacts) != 0 {
		t.Fatal("missing angular or closure proof", m.SquaredSourceNormalTurn, request)
	}
	boundSquared, _ := m.SquaredSourceNormalTurn.Float64()
	// Independently evaluate both native roof surfaces at represented
	// triangle vertices and barycenters. The certificate also bounds
	// unsampled points and includes the boundary transport neighborhood.
	observed := 0.
	for i, tri := range m.Mesh.Triangles {
		roof, isRoof := s.faces[m.OriginalFace[i]].Surface.(*geom.BSplineSurface)
		if !isRoof {
			continue
		}
		a, b, c := m.Mesh.Vertices[tri[0]], m.Mesh.Vertices[tri[1]], m.Mesh.Vertices[tri[2]]
		samples := [4]geom.Vec3{a, b, c, a.Add(b).Add(c).Scale(1.0 / 3)}
		var normals [4]geom.Vec3
		for j, p := range samples {
			uv := chart(p)
			n, err := roof.Normal(uv.X, uv.Y)
			if err != nil {
				t.Fatal(err)
			}
			normals[j] = n
			for _, prev := range normals[:j] {
				angle := math.Atan2(n.Cross(prev).Length(), n.Dot(prev))
				observed = max(observed, angle)
			}
		}
	}
	if !(observed > 0 && observed <= math.Sqrt(boundSquared)) {
		t.Fatalf("source samples reached %g radians, certificate %g", observed, math.Sqrt(boundSquared))
	}
	t.Logf("region %d requested %.3f radians, certificate %.12f, sampled turn %.12f, %d triangles",
		m.Region, request, math.Sqrt(boundSquared), observed, len(m.Mesh.Triangles))
}
