package trimmed

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestCertifiedParabolaOppositeCirclesCutDisplay(t *testing.T) {
	ctx := context.Background()
	opts := ParabolaCircleGraphOptions{}
	for axis := range 2 {
		source := parabolaTwoCirclesStock(t, axis, oppositeCircleStock(), false)
		for order := range 2 {
			t.Run(fmt.Sprintf("%d/%d", axis, order), func(t *testing.T) {
				cut, err := CutParabolaTwoCirclesSplineGraphs(ctx, source[0], source[1+order], source[2-order], axis, 1e-9, opts)
				if err != nil {
					t.Fatal(err)
				}
				if axis == 1 && order == 0 {
					var sourceOnly bytes.Buffer
					if err := cut.WriteJSON(ctx, &sourceOnly); err != nil {
						t.Fatal(err)
					}
					cut, err = ReadParabolaTwoCirclesGraphCutJSON(ctx, &sourceOnly, JSONLimits{}, opts)
					if err != nil {
						t.Fatal(err)
					}
				}
				options := ParabolaTwoCirclesMeshOptions{NormalTurn: .2}
				display, err := cut.TessellateCertified(ctx, options)
				if err != nil {
					t.Fatal(err)
				}
				checkOppositeCirclesSourceDisplay(t, cut.Components(), cut.History(), display, order, func(p geom.Vec3) geom.Vec2 {
					return geom.Vec2{X: p.X, Y: p.Y}
				})
				if axis == 0 && order == 0 {
					// The wall layer alone fits this budget, leaving no
					// triangles for the independently positive lens.
					options.MaxTriangles = len(display[0].Mesh.Triangles)
					if partial, err := cut.TessellateCertified(ctx, options); partial != nil || !errors.Is(err, parametric.ErrBudget) {
						t.Fatal("budgeted Cut leaked a partial wall layer", len(partial), err)
					}
				}
			})
		}
	}
}

func TestCertifiedPlacedParabolaOppositeCirclesCutDisplay(t *testing.T) {
	ctx := context.Background()
	const axis = 1
	local := parabolaTwoCirclesStock(t, axis, oppositeCircleStock(), false)
	stock := placeParabolaTwoCirclesTestStock(t, local)
	for order := range 2 {
		cut, err := CutPlacedParabolaTwoCirclesSplineGraphs(ctx, stock[0], stock[1+order], stock[2-order], axis, 1e-9, ParabolaCircleGraphOptions{})
		if err != nil {
			t.Fatal(err)
		}
		original := [2]*geom.BSplineSurface{stock[1+order], stock[2-order]}
		for i, h := range cut.History() {
			for owner, face := range h.RoofFaces {
				if cut.world[i].faces[face].Surface != original[owner] {
					t.Fatal("placed Cut changed an original world roof", h.Region, face)
				}
			}
		}
		if order == 1 {
			cut = reopenPlacedParabolaTwoCircles(t, cut)
		}
		display, err := cut.TessellateCertified(ctx, ParabolaTwoCirclesMeshOptions{NormalTurn: .2})
		if err != nil {
			t.Fatal(err)
		}
		checkOppositeCirclesSourceDisplay(t, cut.Components(), cut.History(), display, order, func(p geom.Vec3) geom.Vec2 {
			return geom.Vec2{X: p.Y + 3, Y: 17.0/8 - p.Z}
		})
	}
}

func checkOppositeCirclesSourceDisplay(t *testing.T, parts []Solid, history []ParabolaCircleCutComponentHistory,
	display []CertifiedParabolaTwoCirclesComponentMesh, order int, uv func(geom.Vec3) geom.Vec2) {
	t.Helper()
	if len(display) != 2 || len(parts) != 2 || len(history) != 2 {
		t.Fatal("expected a wall layer and a separate lens", len(display), len(parts), len(history))
	}
	// Independent polynomial moments over the two original half-strips
	// and disks. Each strip keeps only its own circle removed.
	upperLens := 102489625.0 * math.Pi / 17592186044416
	lowerLens := 316788675.0 * math.Pi / 2251799813685248
	volumes := [2][2]float64{
		{5438485955.0/14740327759872 + lowerLens, upperLens},
		{12928519905.0/4913442586624 + upperLens, lowerLens},
	}
	for i, m := range display {
		part, h := parts[i], history[i]
		volume, _ := m.SignedDisplayVolume.Float64()
		lo, _ := m.SourceVolume[0].Float64()
		hi, _ := m.SourceVolume[1].Float64()
		distance, _ := m.DistanceUpper.Float64()
		if m.Region != [2]int{order, 2 + order}[i] || h.Region != m.Region ||
			len(m.Mesh.Vertices)-len(m.Mesh.Triangles)/2 != 2*i || len(h.CircleEdges) != 4 ||
			m.Mesh.ValidateSolid() != nil || !m.Collision.Complete || len(m.Collision.Contacts) != 0 {
			t.Fatal("changed native component topology or closure", m.Region, h)
		}
		want := volumes[order][i]
		if math.Abs(lo-want) > want*1e-14 || math.Abs(hi-want) > want*1e-14 ||
			math.Abs(volume-want) > want*.02 || distance <= 0 || distance > .05 {
			t.Fatal("wrong source integral or display bound", m.Region, lo, hi, want, volume, distance)
		}
		for _, circleEdge := range h.CircleEdges {
			samples := m.OriginalEdges[circleEdge]
			if len(samples.Vertices) < 3 {
				t.Fatal("lost original circle-edge samples", circleEdge)
			}
			k := len(samples.Parameters) / 2
			at, _ := samples.Parameters[k].Float64()
			p, err := part.edges[circleEdge].Curve.Eval(at)
			if err != nil || p.Distance(m.Mesh.Vertices[samples.Vertices[k]]) > 1e-12 {
				t.Fatal("circle edge lost its raw source station", circleEdge, at, err)
			}
		}
		verifyParabolaTwoCirclesSourceTurnInChart(t, part, m, .2, uv)
		t.Logf("region=%d Euler=%d signed volume=%.16g source=%.16g distance=%.12g candidate pairs=%d",
			m.Region, 2*i, volume, want, distance, m.Collision.BroadPhasePairs)
	}
}
