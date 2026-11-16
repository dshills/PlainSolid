package trimmed

import (
	"context"
	"errors"
	"math"
	"math/big"
	"slices"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func authoredParabolaCircleSimilarityStock(t *testing.T, source [3]*geom.BSplineSurface) (out [3]*geom.BSplineSurface) {
	t.Helper()
	for i, raw := range source {
		d := raw.Data()
		for j, p := range d.Poles {
			// The stored world control net uses integer basis vectors.
			// Dividing those vectors by five gives a tilted 3-4-5 rotation.
			d.Poles[j] = geom.Vec3{X: 5*p.X + 17./8, Y: 3*p.Y - 4*p.Z - 3, Z: 4*p.Y + 3*p.Z + 5}
		}
		var err error
		out[i], err = geom.NewBSplineSurfaceWithExactWeights(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
		if err != nil {
			t.Fatal(err)
		}
	}
	return out
}

func TestCertifiedParabolaCircleSimilarity(t *testing.T) {
	ctx := context.Background()
	local := parabolaTwoCirclesStock(t, 0, twoCirclePrimaryStock(), false)
	world := authoredParabolaCircleSimilarityStock(t, local)
	const tolerance = 1e-9
	for order := range 2 {
		cut, err := CutPlacedParabolaTwoCirclesSplineGraphs(ctx, world[0], world[1+order], world[2-order], 0, tolerance, ParabolaCircleGraphOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if cut.chart.signedAxisUnit || cut.chart.scale.Cmp(big.NewRat(5, 1)) != 0 ||
			cut.chart.inverse[1][2].Cmp(big.NewRat(4, 25)) != 0 || cut.chart.volume.Cmp(big.NewRat(125, 1)) != 0 {
			t.Fatal("lost the exact nonsigned-axis similarity")
		}
		original := [3]*geom.BSplineSurface{world[0], world[1+order], world[2-order]}
		for i, part := range cut.Components() {
			for side, face := range cut.History()[i].RoofFaces {
				if part.faces[face].Surface != original[side+1] {
					t.Fatal("placed face lost its original authored owner", i, face)
				}
			}
		}
		normalized := [3]*geom.BSplineSurface{local[0], local[1+order], local[2-order]}
		for i, roof := range cut.local.sources {
			if !geom.SameSplineSurface(roof, normalized[i]) {
				t.Fatal("exact inverse did not recover the original local basis", i)
			}
		}
		if order == 1 {
			cut = reopenPlacedParabolaTwoCircles(t, cut)
			if cut.tolerance != tolerance || cut.local.tolerance == tolerance {
				t.Fatal("placed recipe lost its physical tolerance")
			}
		}
		opts := ParabolaTwoCirclesMeshOptions{Deflection: .25, NormalAngle: .1, NormalTurn: .2, MaxSubdivisions: 16}
		worldDisplay, err := cut.TessellateCertified(ctx, opts)
		if err != nil {
			t.Fatal(err)
		}
		localOpts := opts
		localOpts.Deflection = .05
		localDisplay, err := cut.local.TessellateCertified(ctx, localOpts)
		if err != nil || len(worldDisplay) != [2]int{3, 1}[order] || len(worldDisplay) != len(localDisplay) {
			t.Fatal("similarity display component counts", len(worldDisplay), len(localDisplay), err)
		}
		for i, wn := range worldDisplay {
			checkSimilarityParabolaCircleComponent(t, cut.world[i], wn, localDisplay[i])
		}
		if order == 1 {
			opts.Deflection = .1
			if out, err := cut.TessellateCertified(ctx, opts); out != nil || !errors.Is(err, parametric.ErrBudget) {
				t.Fatal("accepted an unproved physical distance", len(out), err)
			}
			t.Log("genus-two .25 physical distance passed and .10 refused at 16 subdivisions")
		}
	}
}

func checkSimilarityParabolaCircleComponent(t *testing.T, s Solid, world, local CertifiedParabolaTwoCirclesComponentMesh) {
	t.Helper()
	if world.Region != local.Region || world.Subdivisions != local.Subdivisions ||
		!slices.Equal(world.Mesh.Triangles, local.Mesh.Triangles) || !slices.Equal(world.OriginalFace, local.OriginalFace) ||
		!world.Collision.Complete || len(world.Collision.Contacts) != 0 || world.Mesh.ValidateSolid() != nil {
		t.Fatal("scaled display changed the certified footprint or model incidence", world.Region)
	}
	for i, q := range local.SourceVolume {
		if new(big.Rat).Mul(q, big.NewRat(125, 1)).Cmp(world.SourceVolume[i]) != 0 {
			t.Fatal("source volume interval did not scale exactly", world.Region, i)
		}
	}
	scaled, _ := world.SignedDisplayVolume.Float64()
	unscaled, _ := local.SignedDisplayVolume.Float64()
	if math.Abs(scaled-125*unscaled) > 1e-12 || math.Abs(scaled-world.Mesh.SignedVolume()) > 1e-12 {
		t.Fatal("world tetrahedral volume does not have the physical scale", scaled, unscaled)
	}
	actualDistance, _ := world.DistanceUpper.Float64()
	localDistance, _ := local.DistanceUpper.Float64()
	if actualDistance > .25 || math.Abs(actualDistance-5*localDistance) > 1e-12 {
		t.Fatal("world distance proof did not scale", actualDistance, localDistance)
	}
	for _, bounds := range [2][2]*big.Rat{
		{world.SquaredSourceNormalTurn, local.SquaredSourceNormalTurn},
		{world.SquaredFacetNormalTangent, local.SquaredFacetNormalTangent},
	} {
		if bounds[0] == nil || bounds[1] == nil {
			t.Fatal("lost one of the normal certificates")
		}
		delta := new(big.Rat).Abs(new(big.Rat).Sub(bounds[0], bounds[1]))
		if delta.Cmp(big.NewRat(1, 1_000_000_000_000)) > 0 {
			t.Fatal("world angular certificate changed under similarity", delta.FloatString(15))
		}
	}
	for i, p := range s.Vertices() {
		if world.Mesh.Vertices[world.OriginalVertices[i]] != p {
			t.Fatal("world mesh changed an original native vertex", i)
		}
	}
	for i, e := range s.Edges() {
		w, l := world.OriginalEdges[i], local.OriginalEdges[i]
		if !slices.EqualFunc(w.Parameters, l.Parameters, func(a, b *big.Rat) bool { return a.Cmp(b) == 0 }) {
			t.Fatal("similarity changed original edge stations", i)
		}
		k := len(w.Parameters) / 2
		station, _ := w.Parameters[k].Float64()
		p, err := e.Curve.Eval(station)
		if err != nil || p.Distance(world.Mesh.Vertices[w.Vertices[k]]) > 1e-12 {
			t.Fatal("model trace lost its world station", i, station, err)
		}
	}
	checkParabolaCircleFacetDisplay(t, s, world, .1, func(p geom.Vec3) geom.Vec2 {
		return geom.Vec2{X: (p.X - 17.0/8) / 5, Y: (3*(p.Y+3) + 4*(p.Z-5)) / 25}
	})
	t.Logf("region=%d physical bound %.12f world volume %.15g local volume %.15g triangles=%d",
		world.Region, actualDistance, scaled, unscaled, len(world.Mesh.Triangles))
}

func TestPlacedParabolaCircleRejectsNonSimilarity(t *testing.T) {
	ctx := context.Background()
	local := parabolaTwoCirclesStock(t, 0, twoCirclePrimaryStock(), false)
	world := authoredParabolaCircleSimilarityStock(t, local)
	d := world[0].Data()
	k := (d.CountU - 1) * d.CountV
	d.Poles[k].Z = math.Nextafter(d.Poles[k].Z, math.Inf(1))
	skew, err := geom.NewBSplineSurfaceWithExactWeights(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
	if err != nil {
		t.Fatal(err)
	}
	if cut, err := CutPlacedParabolaTwoCirclesSplineGraphs(ctx, skew, world[1], world[2], 0, 1e-9, ParabolaCircleGraphOptions{}); cut != nil ||
		!errors.Is(err, geom.ErrConvergence) {
		t.Fatal("accepted a skewed original base frame", cut, err)
	}
}
