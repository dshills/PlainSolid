package trimmed

import (
	"math/big"
	"plainsolid/geom"
	"plainsolid/intersect"
	"testing"
)

func mixedWallFixture(t *testing.T) (Solid, Solid, []RationalIsoShellPairProposal, MixedAlgebraicShellSectionOptions) {
	a, _ := crossingFootprintRoofShell(t, false)
	b, _ := crossingFootprintRoofShell(t, true)
	o := AlgebraicShellSectionOptions{}
	o.Trace.UniformOverlapGraphFallback = intersect.UniformGraphDiscoveryOptions{GridDivisions: 16, AxisDivisions: [2]int{8, 16}}
	o.Trace.UniformOverlapGraphFallback.Enabled = true
	o.Trace.UniformOverlapGraphFallback.Plane.Arcs.ResolveSubdivisionCorners = true
	o.Trace.UniformOverlapGraphFallback.MaxArcs = 1024
	o.Trace.UniformOverlapGraphFallback.MaxTubes = 1024
	o.Trace.Coverage.Isolation.ParameterTolerance = .125
	o.Trace.Coverage.Cell.MaxOperations = 16000000
	o.Trace.Branches.PruneSeparatedEndpoints = true
	o.Trace.Branches.MaxComparisons = 4096
	o.Trace.MaxTopologyPairs = 1000000
	o.Trace.Coverage.MaxCandidateTests = 4096
	o.Trace.Coverage.IsoPlaneFallback = true
	o.Trace.Coverage.IsoPlane.AllowTrapezoidCharts = true
	o.Trace.PlaneFallback.Enabled = true
	o.Trace.PlaneFallback.TrapezoidCharts = true
	o.Trace.Coverage.IsoPlane.MaxOperations = 64000000
	o.Trace.PlaneFallback.Plane.Arcs.ResolveSubdivisionCorners = true

	pieces := []geom.RationalIsoTubePiece{
		{Box: geom.SurfacePairParameterBox{A: geom.ParameterRectangle{UMin: .9375, UMax: 1.0625, VMin: 0, VMax: .5}, B: geom.ParameterRectangle{UMin: .9375, UMax: 1.0625, VMin: .375, VMax: .9375}}, FreeAxis: 1},
		{Box: geom.SurfacePairParameterBox{A: geom.ParameterRectangle{UMin: .9375, UMax: 1.0625, VMin: .25, VMax: .875}, B: geom.ParameterRectangle{UMin: .9375, UMax: 1.0625, VMin: .75, VMax: 1}}, FreeAxis: 3},
	}
	cuts := []*big.Rat{new(big.Rat), big.NewRat(1, 2), big.NewRat(11, 15)}
	return a, b, []RationalIsoShellPairProposal{{Faces: [2]int{5, 3}, SourceOperand: 1, FixedAxis: 0, Fixed: 1, Cuts: cuts, Pieces: pieces}}, MixedAlgebraicShellSectionOptions{Section: o}
}
