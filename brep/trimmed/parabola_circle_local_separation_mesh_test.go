//go:build research

package trimmed

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/mesh"
)

func TestCertifiedParabolaLocalSeparationDisplay(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	opts := ParabolaCircleGraphOptions{}
	for axis := range 2 {
		src := parabolaTwoCirclesStock(t, axis, localOppositeCircleStock(), false)
		for order := range 2 {
			t.Run(fmt.Sprintf("%d/%d", axis, order), func(t *testing.T) {
				cut, err := CutParabolaTwoCirclesSplineGraphs(ctx, src[0], src[1+order], src[2-order], axis, 1e-9, opts)
				if err != nil {
					t.Fatal(err)
				}
				var sourceOnly, canonical bytes.Buffer
				if err := cut.WriteJSON(ctx, &sourceOnly); err != nil {
					t.Fatal(err)
				}
				replay, err := ReadParabolaTwoCirclesGraphCutJSON(ctx, bytes.NewReader(sourceOnly.Bytes()), JSONLimits{}, opts)
				if err != nil {
					t.Fatal(err)
				}
				if err := replay.WriteJSON(ctx, &canonical); err != nil || !bytes.Equal(sourceOnly.Bytes(), canonical.Bytes()) {
					t.Fatal("local Cut changed its exact source-only recipe", err)
				}
				request := ParabolaTwoCirclesMeshOptions{NormalTurn: .2}
				if order == 0 {
					request.NormalAngle = .1
				}
				display, err := replay.TessellateCertified(ctx, request)
				if err != nil {
					t.Fatal(err)
				}
				uv := func(p geom.Vec3) geom.Vec2 { return geom.Vec2{X: p.X, Y: p.Y} }
				checkParabolaLocalSeparationDisplay(t, replay.Components(), display, order, 1, .05, uv)
				if request.NormalAngle > 0 {
					for i, m := range display {
						checkParabolaCircleFacetDisplay(t, replay.components[i], m, request.NormalAngle, uv)
					}
				}
				if axis == 0 && order == 0 {
					report, err := collision.ValidateMeshVertexContactAssembly(ctx, []mesh.Mesh{display[0].Mesh, display[1].Mesh},
						nil, collision.VertexContactAssemblyOptions{
							CrossBVH: true,
							Validation: collision.LocationOptions{
								MaxTriangles: collision.MaxTriangles, MaxCollisionPairs: collision.DefaultMaxPairs,
							},
						})
					if err != nil || len(report.Contacts) != 0 || report.BroadPairs <= 1 {
						t.Fatal("did not prove exact exterior membership behind overlapping assembly boxes", report, err)
					}
					t.Logf("overlapping-box cross proof inspected %d pairs and found zero contacts", report.BroadPairs)
				}
			})
		}
	}
}

func TestCertifiedPlacedParabolaLocalSeparationTinyLens(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	local := parabolaTwoCirclesStock(t, 1, localOppositeCircleStock(), false)
	world := authoredParabolaCircleSimilarityStock(t, local)
	cut, err := CutPlacedParabolaTwoCirclesSplineGraphs(ctx, world[0], world[2], world[1], 1, 1e-9, ParabolaCircleGraphOptions{})
	if err != nil {
		t.Fatal(err)
	}
	replay := reopenPlacedParabolaTwoCircles(t, cut)
	display, err := replay.TessellateCertified(ctx, ParabolaTwoCirclesMeshOptions{Deflection: .25, NormalTurn: .2})
	if err != nil {
		t.Fatal(err)
	}
	uv := func(p geom.Vec3) geom.Vec2 {
		return geom.Vec2{X: min(1, max(0, (p.X-17.0/8)/5)), Y: min(1, max(0, (3*(p.Y+3)+4*(p.Z-5))/25))}
	}
	checkParabolaLocalSeparationDisplay(t, replay.Components(), display, 1, 125, .25, uv)
}

func checkParabolaLocalSeparationDisplay(t *testing.T, components []Solid, displays []CertifiedParabolaTwoCirclesComponentMesh,
	order int, volumeScale, deflection float64, uv func(geom.Vec3) geom.Vec2) {
	t.Helper()
	if len(components) != 2 || len(displays) != 2 {
		t.Fatal("local Cut must retain wall layer and lens", len(components), len(displays))
	}
	expected := [2][2]float64{
		{3765044493245.0/16980857579372544 + localLowerLens, localUpperLens},
		{117380058565805.0/16980857579372544 + localUpperLens, localLowerLens},
	}
	var boxes [2]geom.Box
	for i, m := range displays {
		native := components[i]
		want := expected[order][i] * volumeScale
		lo, _ := m.SourceVolume[0].Float64()
		hi, _ := m.SourceVolume[1].Float64()
		volume, _ := m.SignedDisplayVolume.Float64()
		distance, _ := m.DistanceUpper.Float64()
		if m.Region != [2]int{order, 2 + order}[i] || len(m.Mesh.Vertices)-len(m.Mesh.Triangles)/2 != 2*i ||
			m.Mesh.ValidateSolid() != nil || !m.Collision.Complete || len(m.Collision.Contacts) != 0 {
			t.Fatal("invalid original local shell display", i, m.Region)
		}
		if math.Abs(lo-want) > want*1e-14 || math.Abs(hi-want) > want*1e-14 ||
			math.Abs(volume-want) > want*.02 || distance <= 0 || distance > deflection {
			t.Fatal("wrong original physical volume or deflection", i, lo, hi, volume, distance, want, deflection)
		}
		for id, vertex := range native.Vertices() {
			if m.Mesh.Vertices[m.OriginalVertices[id]] != vertex {
				t.Fatal("display changed an original shell-local vertex", i, id)
			}
		}
		verifyParabolaTwoCirclesSourceTurnInChart(t, native, m, .2, uv)
		t.Logf("region=%d Euler=%d signed volume=%.16g original=%.16g distance=%.12g triangles=%d self pairs=%d",
			m.Region, 2*i, volume, want, distance, len(m.Mesh.Triangles), m.Collision.BroadPhasePairs)
		var err error
		boxes[i], err = m.Mesh.Bounds()
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, overlap := boxes[0].Intersect(boxes[1]); !overlap {
		t.Fatal("fixture must require disjointness proof beyond a global box split", boxes)
	}
}
