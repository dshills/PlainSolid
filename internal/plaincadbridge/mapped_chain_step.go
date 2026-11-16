package plaincadbridge

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"math"
	"math/big"
	"strings"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/collision"
	"plainsolid/exchange/step"
	"plainsolid/geom"
	"plainsolid/mesh"
)

const (
	orderedMappedSTEPFamily      = "mapped-chain-v13"
	orderedMappedDisplayDistance = .36
	orderedMappedDisplayTurn     = .35
)

type orderedMappedSTEPBody struct {
	source      step.MappedChainSTEPModel
	display     *mesh.Mesh
	turnDisplay *mesh.Mesh
}

type mappedChainStepEvidence struct {
	BoundaryVersion int                    `json:"boundaryVersion"`
	OriginalFaces   []int64                `json:"originalFaces"`
	OriginalEdges   []int64                `json:"originalEdges"`
	SupportEvents   []mappedChainStepEvent `json:"supportEvents"`
}

type mappedChainStepEvent struct {
	ModelStation        string  `json:"modelStation"`
	OwnersChangingCells [2]bool `json:"ownersChangingCells"`
	OriginalJoint       bool    `json:"originalJoint"`
}

func orderedMappedSTEPOptions() step.MappedChainShellSTEPOptions {
	return step.MappedChainShellSTEPOptions{
		AllowMultiSpanRoofs: true, AllowTwoInteriorRoofKnots: true,
		AllowUnsplitSupportKnot: true, AllowMixedOwnerBases: true,
		AllowJoinedSupportKnotCells: true, AllowOrderedJoinedSupportKnotCells: true,
	}
}

func (b *Bridge) importOrderedMappedSTEP(ctx context.Context, source string, q stepRepairRequest, units stepFileUnit) (Shape, error) {
	if len(source) == 0 || len(source) > maxSTEPBytes {
		return Shape{}, &failureError{"LIMIT", "STEP source must contain between 1 byte and 1 MiB"}
	}
	if units != "" {
		return Shape{}, &failureError{"INVALID", "ordered mapped STEP requires the default millimetre source unit"}
	}
	options := orderedMappedSTEPOptions()
	m, err := step.ReadMappedChainRectangularShell(ctx, strings.NewReader(source), stepLimits(), options)
	if err != nil {
		m, err = step.ReadOCCTMappedChainRectangularShell(ctx, strings.NewReader(source), stepLimits(), options)
	}
	if err != nil {
		return Shape{}, unsupportedEdge(fmt.Sprintf("ordered mapped STEP source could not be proved: %v", err))
	}
	proof, err := proveOrderedMappedSTEP(ctx, m)
	if err != nil {
		return Shape{}, err
	}
	var receipt bytes.Buffer
	if q.Native || q.NativeText {
		if err := m.Shell.WriteJSON(ctx, &receipt, trimmed.MappedChainShellNativeOptions{MaxBytes: MaxRequestBytes}); err != nil {
			return Shape{}, err
		}
	}
	graph := mappedChainStepEvidence{BoundaryVersion: 13, OriginalFaces: m.FaceIDs}
	for _, edge := range m.Chain {
		graph.OriginalEdges = append(graph.OriginalEdges, edge.EdgeID)
	}
	for _, event := range m.Shell.JoinedSupportEvents() {
		graph.SupportEvents = append(graph.SupportEvents, mappedChainStepEvent{
			ModelStation: event.ModelStation.RatString(), OwnersChangingCells: event.OwnersChangingCells, OriginalJoint: event.OriginalJoint,
		})
	}
	evidence := stepRepairEvidence{
		Family: orderedMappedSTEPFamily, SourceSHA256: fmt.Sprintf("%x", sha256.Sum256([]byte(m.OriginalSTEP()))),
		LengthUnit: m.LengthUnit, MaxDistance: float64(q.MaxDistance), Repairs: []step.ConvexArcRepairSite{},
		PCurveOwners: 2 * (len(m.Native.Solid.Edges()) - len(m.Crossings)), MappedChain: &graph,
	}
	shape, err := b.publish(stored{
		mappedChainSTEP: &orderedMappedSTEPBody{source: m},
		result:          Shape{GeometryAssertions: proof.Assertions, Bounds: proof.Bounds},
	})
	if err != nil {
		return Shape{}, err
	}
	return b.attachSTEPRepairEvidence(shape, evidence, q, receipt), nil
}

func proveOrderedMappedSTEP(ctx context.Context, m step.MappedChainSTEPModel) (stepProof, error) {
	if len(m.Chain) != 4 || len(m.Crossings) != 4 || len(m.SupportJoints) != 3 || m.LengthUnit != .001 {
		return stepProof{}, unsupportedEdge("ordered mapped STEP requires four original model cells and a millimetre source")
	}
	body := m.Native.Solid
	if len(body.Topology().Shells) != 1 {
		return stepProof{}, geom.ErrDomain
	}
	p, err := body.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
	if err != nil {
		return stepProof{}, err
	}
	if !finite(p.Volume) || !finite(p.Area) || p.Volume <= 0 || p.Area <= 0 {
		return stepProof{}, geom.ErrDomain
	}
	bounds, err := body.Bounds()
	if err != nil {
		return stepProof{}, err
	}
	return stepProof{
		Assertions: Assertions{Valid: true, Volume: p.Volume, SurfaceArea: p.Area, SolidCount: 1},
		Bounds:     Bounds{Min: coordinates(bounds.Min), Max: coordinates(bounds.Max)},
	}, nil
}

func (p *orderedMappedSTEPBody) tessellate(ctx context.Context, q request) (mesh.Mesh, error) {
	if q.LinearDeflection.set && float64(q.LinearDeflection.value) < orderedMappedDisplayDistance {
		return mesh.Mesh{}, unsupportedEdge("ordered mapped STEP display requires linear deflection of at least 0.36 mm")
	}
	if q.AngularDeflection.set && float64(q.AngularDeflection.value) < math.Pi {
		if float64(q.AngularDeflection.value) < orderedMappedDisplayTurn {
			return mesh.Mesh{}, unsupportedEdge("ordered mapped STEP display certifies linear distance and requires angular deflection pi on the distance-only path. Certified normal-turn display requires at least 0.35 radians")
		}
		return p.tessellateNormalTurn(ctx)
	}
	if p.display != nil {
		return *p.display, nil
	}
	fill := trimmed.MixedExactMultiSpanMeshOptions{
		Span: trimmed.MultiSpanTriangleBoundOptions{MaxDegree: 4}, MaxResidualOperations: 512 << 20,
		Face: trimmed.MixedExactCurvedMeshOptions{
			BoundaryModelBudget: .065,
			Interior:            trimmed.CurvedQuadraticTrimMeshOptions{Deflection: orderedMappedDisplayDistance, MaxSubdivisions: 64},
		},
	}
	display, err := p.source.Shell.TessellateClosedCertified(ctx, nil, trimmed.MappedChainClosedMeshOptions{
		Boundary:  trimmed.MappedChainPolylineOptions{Deflection: .01875},
		Curved:    [2]trimmed.MixedExactMultiSpanMeshOptions{fill, fill},
		Collision: collision.Options{MaxPairs: 4_000_000},
	})
	if err != nil {
		return mesh.Mesh{}, err
	}
	limit := new(big.Rat).SetFloat64(orderedMappedDisplayDistance)
	if !display.DisplayCollision.Complete || len(display.DisplayCollision.Contacts) != 0 ||
		display.SquaredDistance.Cmp(new(big.Rat).Mul(limit, limit)) > 0 {
		return mesh.Mesh{}, geom.ErrConvergence
	}
	p.display = &display.Mesh
	return *p.display, nil
}

func (p *orderedMappedSTEPBody) tessellateNormalTurn(ctx context.Context) (mesh.Mesh, error) {
	if p.turnDisplay != nil {
		return *p.turnDisplay, nil
	}
	fill := trimmed.MixedExactMultiSpanMeshOptions{
		Span: trimmed.MultiSpanTriangleBoundOptions{MaxDegree: 4}, MaxResidualOperations: 1000000000,
		Face: trimmed.MixedExactCurvedMeshOptions{
			BoundaryModelBudget: .065,
			Interior:            trimmed.CurvedQuadraticTrimMeshOptions{Deflection: .30, MaxSubdivisions: 128},
		},
	}
	display, err := p.source.Shell.TessellateClosedCertified(ctx, nil, trimmed.MappedChainClosedMeshOptions{
		AlignSupportKnots: true, MaxKnotCellFlips: 100000, NormalTurn: orderedMappedDisplayTurn, MaxTriangles: 10000,
		Boundary:  trimmed.MappedChainPolylineOptions{Deflection: .01875},
		Curved:    [2]trimmed.MixedExactMultiSpanMeshOptions{fill, fill},
		Collision: collision.Options{MaxPairs: 4_000_000},
	})
	if err != nil {
		return mesh.Mesh{}, err
	}
	if display.SquaredNormalTurnTangent == nil || display.SquaredDistance.Cmp(big.NewRat(9, 100)) > 0 ||
		!display.DisplayCollision.Complete || len(display.DisplayCollision.Contacts) != 0 {
		return mesh.Mesh{}, geom.ErrConvergence
	}
	p.turnDisplay = &display.Mesh
	return *p.turnDisplay, nil
}

func (s stored) exportOrderedMappedSTEP(ctx context.Context) (stepExport, error) {
	options := orderedMappedSTEPOptions()
	var canonical bytes.Buffer
	if err := step.WriteMappedChainRectangularOriginalShell(ctx, &canonical, s.mappedChainSTEP.source, stepLimits(), options); err != nil {
		return stepExport{}, err
	}
	m, err := step.ReadMappedChainRectangularShell(ctx, bytes.NewReader(canonical.Bytes()), stepLimits(), options)
	if err != nil {
		return stepExport{}, err
	}
	after, err := proveOrderedMappedSTEP(ctx, m)
	if err != nil {
		return stepExport{}, err
	}
	before := stepProof{s.result.GeometryAssertions, s.result.Bounds}
	if before != after {
		return stepExport{}, fmt.Errorf("ordered mapped STEP readback changed analytic measures or bounds: %w", geom.ErrConvergence)
	}
	return stepExport{
		Base64: base64.StdEncoding.EncodeToString(canonical.Bytes()),
		Before: []stepProof{before}, After: []stepProof{after}, Units: stepMillimetre,
	}, nil
}
