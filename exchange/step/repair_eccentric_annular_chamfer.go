package step

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"slices"

	"plainsolid/geom"
)

// EccentricAnnularChamferRepairOptions sets the maximum geometric
// displacement in the original STEP length unit.
type EccentricAnnularChamferRepairOptions struct {
	MaxDistance float64
}

// EccentricAnnularChamferWallRepair records one original cone, its finite
// seam and delimiting rims. The two stations follow the original edge's
// vertex order. ExactSlope is radial change along the signed cone axis.
// Bounds and stations are exact rationals in the source length unit.
type EccentricAnnularChamferWallRepair struct {
	FaceID, ConeID, PlacementID, AxisDirectionID  int64
	SeamEdgeID, SeamCurveID, CorrectedGeneratorID int64
	RimEdgeIDs, SeamVertexIDs                     [2]int64
	AxisSign                                      int
	OriginalAngle, ExactSlope                     string
	OriginalZStations, OriginalAxialStations      [2]string
	ConeDisplacementBound, SeamDisplacementBound  string
}

// RepairedEccentricAnnularChamferSTEP joins all six finite cone and seam
// proofs to the unchanged source and a strictly reopenable STEP model.
type RepairedEccentricAnnularChamferSTEP struct {
	original, corrected string
	maxDistance         float64
	material            EccentricAnnularChamferSTEP
	walls               [6]EccentricAnnularChamferWallRepair
}

// OriginalSTEP returns the byte-preserved input.
func (m RepairedEccentricAnnularChamferSTEP) OriginalSTEP() string { return m.original }

// CorrectedModelSTEP returns the finite exact STEP carrier.
func (m RepairedEccentricAnnularChamferSTEP) CorrectedModelSTEP() string { return m.corrected }

// Material returns the reconstructed solid and its original source owners.
func (m RepairedEccentricAnnularChamferSTEP) Material() EccentricAnnularChamferSTEP {
	p := m.material
	p.NativeFaceSourceIDs = slices.Clone(p.NativeFaceSourceIDs)
	p.NativeEdgeSourceIDs = slices.Clone(p.NativeEdgeSourceIDs)
	p.NativeVertexSourceIDs = slices.Clone(p.NativeVertexSourceIDs)
	return p
}

// Walls returns the six original conical support and seam proofs in source
// shell order.
func (m RepairedEccentricAnnularChamferSTEP) Walls() [6]EccentricAnnularChamferWallRepair {
	return m.walls
}

var errEccentricAnnularChamferRepair = errors.New("step: unsupported bounded eccentric annular chamfer repair")

// RepairEccentricAnnularChamfer explicitly repairs the rounded model-only
// OCCT dialect with six conical supports and six line seams. The original
// vertices, rims, signed axes and every directed coedge remain authoritative;
// the corrected carriers must pass ReadEccentricAnnularChamfer.
func RepairEccentricAnnularChamfer(ctx context.Context, r io.Reader, l Limits, o EccentricAnnularChamferRepairOptions) (RepairedEccentricAnnularChamferSTEP, error) {
	var zero RepairedEccentricAnnularChamferSTEP
	if ctx == nil || r == nil || !finiteNumber(o.MaxDistance) || o.MaxDistance <= 0 {
		return zero, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	l, err := l.defaults()
	if err != nil {
		return zero, err
	}
	l.MaxBytes = min(l.MaxBytes, 128<<10)
	l.MaxEntities = min(l.MaxEntities, 512)
	l.MaxValues = min(l.MaxValues, 8192)
	l.MaxDepth = min(l.MaxDepth, 16)
	raw, err := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if err != nil {
		return zero, err
	}
	if int64(len(raw)) > l.MaxBytes {
		return zero, geom.ErrConvergence
	}
	f, rep, unit, items, err := parseCanonicalCircularExtrusions(bytes.NewReader(raw), l)
	if err != nil {
		return zero, err
	}
	representation, _ := simple(f, rep, "ADVANCED_BREP_SHAPE_REPRESENTATION", 3)
	if err := occtLoopContextMode(f, representation.Args[2].Ref, true); err != nil {
		return zero, err
	}
	body, err := modelOnlyCircularSTEPBody(f, rep, items, map[string]bool{"CONICAL_SURFACE": true})
	if err != nil {
		return zero, err
	}
	g, err := readModelOnlyConicalRepairGraph(f, body, 8, 14, 8)
	if err != nil {
		return zero, fmt.Errorf("%w: %v", errEccentricAnnularChamferRepair, err)
	}
	for _, uses := range g.uses {
		if uses[0].sense == uses[1].sense {
			return zero, errEccentricAnnularChamferRepair
		}
	}
	fixed := cloneOvalFile(f)
	fixed.Order = slices.Clone(f.Order)
	var walls [6]EccentricAnnularChamferWallRepair
	var obsolete []int64
	n := 0
	budget := sideRat(o.MaxDistance)
	for i, fc := range g.faces {
		if err := ctx.Err(); err != nil {
			return zero, err
		}
		if fc.kind == "PLANE" {
			continue
		}
		if fc.kind != "CONICAL_SURFACE" || n == len(walls) {
			return zero, errEccentricAnnularChamferRepair
		}
		wall, removed, err := repairEccentricAnnularWall(f, fixed, g, i, budget)
		if err != nil {
			return zero, fmt.Errorf("step: repair annular wall #%d: %w", fc.id, err)
		}
		walls[n] = wall
		n++
		obsolete = append(obsolete, removed...)
	}
	if n != len(walls) {
		return zero, errEccentricAnnularChamferRepair
	}
	if err := pruneConicalRepairSupports(f, fixed, rep, obsolete); err != nil {
		return zero, err
	}
	corrected, err := mappedPlanarSourceBytes(fixed)
	if err != nil {
		return zero, err
	}
	m, err := ReadEccentricAnnularChamfer(ctx, bytes.NewReader(corrected), l, geom.SurfacePatchOptions{})
	if err != nil {
		return zero, fmt.Errorf("step: verify corrected annular chamfer source: %w", err)
	}
	if m.LengthUnit != unit || m.SourceBodyID != body {
		return zero, errEccentricAnnularChamferRepair
	}
	return RepairedEccentricAnnularChamferSTEP{original: string(raw), corrected: string(corrected), maxDistance: o.MaxDistance, material: m, walls: walls}, nil
}
