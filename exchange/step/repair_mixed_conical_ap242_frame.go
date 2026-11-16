package step

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/big"
	"slices"
	"strconv"
	"strings"

	"plainsolid/geom"
)

// MixedConicalAP242FrameRepairOptions selects one original terminal rolling
// rim occurrence. MaxAxisChord limits each original unit direction's
// change (2*sin(angle/2)). MaxDisplacement bounds any material point's
// movement in the original file units. MaxPrimitiveComponent sets the
// finite integer-ray inference domain (1–4096, default 256).
type MixedConicalAP242FrameRepairOptions struct {
	OccurrenceID          int64
	MaxAxisChord          float64
	MaxDisplacement       float64
	MaxPrimitiveComponent int
}

// MixedConicalAP242FrameRepairEvidence retains the original owner IDs and
// exact source-axis correction. Axis arrays are in X, Z order. Rational
// strings in the bounds encode conservative enclosures in file units or
// unit-direction chord as appropriate.
type MixedConicalAP242FrameRepairEvidence struct {
	OccurrenceID, ParentID, ChildID, BodyID         int64
	CDSRID, RelationshipID, TransformationID        int64
	SourcePlacementID, TargetPlacementID, OriginID  int64
	DirectionIDs                                    [2]int64
	OriginalDirections                              [2][3]string
	PrimitiveDirections                             [2][3]int64
	AxisChordUpper                                  [2]string
	FrameChordUpper, LocalExtent, DisplacementUpper string
	CorrectedLocalPose                              [3][4]string
}

// RepairedMixedConicalAP242Frame joins the original STEP, its corrected
// carrier and the source-bound proof of one shared relationship change.
type RepairedMixedConicalAP242Frame struct {
	original, corrected string
	options             MixedConicalAP242FrameRepairOptions
	evidence            MixedConicalAP242FrameRepairEvidence
	assembly            MixedConicalAP242DAG
}

func (m RepairedMixedConicalAP242Frame) OriginalSTEP() string       { return m.original }
func (m RepairedMixedConicalAP242Frame) CorrectedModelSTEP() string { return m.corrected }
func (m RepairedMixedConicalAP242Frame) Evidence() MixedConicalAP242FrameRepairEvidence {
	return m.evidence
}

// Assembly returns the corrected exact assembly and its original expanded paths.
func (m RepairedMixedConicalAP242Frame) Assembly() MixedConicalAP242DAG {
	a := (RepairedMixedConicalAP242DAG{assembly: m.assembly}).Assembly()
	if m.corrected != "" {
		// Give callers a private index of an already strictly verified carrier.
		a.Assembly.Source, _ = Parse(strings.NewReader(m.corrected), Limits{})
	}
	return a
}

// RepairMixedConicalAP242Frame performs one opt-in correction of a rounded
// occurrence frame in an unstyled bounded mixed conical DAG. Every original
// geometry record, origin and relationship remains fixed. The corrected
// graph must pass the ordinary strict mixed conical reader.
func RepairMixedConicalAP242Frame(ctx context.Context, r io.Reader, limits MixedConicalAP242DAGLimits, o MixedConicalAP242FrameRepairOptions) (RepairedMixedConicalAP242Frame, error) {
	var zero RepairedMixedConicalAP242Frame
	if o.MaxPrimitiveComponent == 0 {
		o.MaxPrimitiveComponent = 256
	}
	if ctx == nil || r == nil || o.OccurrenceID <= 0 || !finiteNumber(o.MaxAxisChord) || o.MaxAxisChord <= 0 || o.MaxAxisChord >= .25 ||
		!finiteNumber(o.MaxDisplacement) || o.MaxDisplacement <= 0 || o.MaxPrimitiveComponent < 1 || o.MaxPrimitiveComponent > 4096 {
		return zero, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	l, err := mixedConicalAP242DAGLimits(limits)
	if err != nil {
		return zero, err
	}
	raw, err := io.ReadAll(io.LimitReader(r, l.Source.MaxBytes+1))
	if err != nil {
		return zero, err
	}
	if int64(len(raw)) > l.Source.MaxBytes {
		return zero, geom.ErrConvergence
	}
	a, err := ReadAssembly(bytes.NewReader(raw), l.Source)
	if err != nil {
		return zero, err
	}
	occ, ids, originID, err := mixedConicalFrameRepairOwner(a, o.OccurrenceID)
	if err != nil {
		return zero, err
	}
	f := a.Source
	var original [2][3]*big.Rat
	for i, id := range ids {
		original[i], err = mappedRationalCoordinate(f, id, "DIRECTION", 3)
		if err != nil {
			return zero, err
		}
	}
	if mappedRationalDot(original[0], original[1]).Sign() == 0 {
		return zero, geom.ErrUnrepresentable
	}
	evidence := MixedConicalAP242FrameRepairEvidence{
		OccurrenceID: occ.NAUOID, ParentID: occ.ParentID, ChildID: occ.ChildID,
		CDSRID: occ.CDSRID, RelationshipID: occ.RelationshipID, TransformationID: occ.TransformationID,
		SourcePlacementID: occ.FromPlacementID, TargetPlacementID: occ.ToPlacementID, OriginID: originID, DirectionIDs: ids,
	}
	chord := new(big.Rat).SetFloat64(o.MaxAxisChord)
	var unit [2][3]*big.Rat
	var upper [2]*big.Rat
	for i, axis := range original {
		evidence.PrimitiveDirections[i], unit[i], upper[i], err = reconstructMixedConicalFrameAxis(ctx, axis, chord, o.MaxPrimitiveComponent)
		if err != nil {
			return zero, fmt.Errorf("step: recover occurrence direction #%d: %w", ids[i], err)
		}
		evidence.AxisChordUpper[i] = upper[i].RatString()
		for j, q := range axis {
			evidence.OriginalDirections[i][j] = q.RatString()
		}
	}
	if mappedRationalDot(unit[0], unit[1]).Sign() != 0 {
		return zero, geom.ErrUnrepresentable
	}
	origin, err := mappedRationalCoordinate(f, originID, "CARTESIAN_POINT", 3)
	if err != nil {
		return zero, err
	}
	occ.pose = mappedRationalBasis{Origin: origin, X: unit[0], Y: mappedRationalCross(unit[1], unit[0]), Z: unit[1]}
	for i, id := range ids {
		record, _ := simple(f, id, "DIRECTION", 2)
		record.Args = slices.Clone(record.Args)
		items := make([]Value, 3)
		for j, value := range evidence.PrimitiveDirections[i] {
			items[j] = Value{Kind: Number, Text: strconv.FormatInt(value, 10) + "."}
		}
		record.Args[1] = Value{Kind: Aggregate, Items: items}
		f.Entities[id] = Entity{ID: id, Records: []Record{record}}
	}
	corrected, err := mappedPlanarSourceBytes(f)
	if err != nil {
		return zero, err
	}
	model, err := ReadMixedConicalAP242DAG(ctx, bytes.NewReader(corrected), l, geom.SurfacePatchOptions{})
	if err != nil {
		return zero, fmt.Errorf("step: verify repaired occurrence carrier: %w", err)
	}
	if !slices.ContainsFunc(model.Occurrences, func(use ExactNAUOOccurrence) bool {
		return use.NAUOID == occ.NAUOID && equalMixedConicalOccurrences(use, occ)
	}) {
		return zero, geom.ErrUnrepresentable
	}
	part := model.Parts[occ.ChildID].Rounded
	if part == nil {
		return zero, geom.ErrUnrepresentable
	}
	extent, err := mixedConicalFrameRepairExtent(*part)
	if err != nil {
		return zero, err
	}
	// For u=unit(raw Z) and v=unit(raw X), STEP forms Y'=unit(u×v)
	// and X'=Y'×u. The three column differences are bounded by
	// (2dx+3dz,2dx+2dz,dz). Their sum bounds the frame chord, and its
	// product with the exact local coordinate extent bounds every move.
	frameBound := new(big.Rat).Add(new(big.Rat).Mul(upper[0], big.NewRat(4, 1)), new(big.Rat).Mul(upper[1], big.NewRat(6, 1)))
	displacement := new(big.Rat).Mul(frameBound, extent)
	if displacement.Cmp(new(big.Rat).SetFloat64(o.MaxDisplacement)) > 0 {
		return zero, fmt.Errorf("step: occurrence displacement proof exceeds source-unit budget: %w", geom.ErrConvergence)
	}
	evidence.BodyID = part.SourceBodyID
	evidence.FrameChordUpper, evidence.LocalExtent, evidence.DisplacementUpper =
		frameBound.RatString(), extent.RatString(), displacement.RatString()
	for i, row := range occ.Matrix() {
		for j, q := range row {
			evidence.CorrectedLocalPose[i][j] = q.RatString()
		}
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	return RepairedMixedConicalAP242Frame{original: string(raw), corrected: string(corrected), options: o, evidence: evidence, assembly: model}, nil
}

func mixedConicalFrameRepairOwner(a Assembly, target int64) (occ ExactNAUOOccurrence, ids [2]int64, originID int64, err error) {
	fail := geom.ErrUnrepresentable
	uses := make(map[int64]AssemblyOccurrence, len(a.Occurrences))
	for _, use := range a.Occurrences {
		uses[use.ID] = use
	}
	use, ok := uses[target]
	if !ok || slices.ContainsFunc(a.Occurrences, func(u AssemblyOccurrence) bool { return u.Parent == use.Child }) {
		return occ, ids, 0, fail
	}
	f := a.Source
	for _, id := range f.Order {
		records := f.Entities[id].Records
		if len(records) != 1 || records[0].Type != "CONTEXT_DEPENDENT_SHAPE_REPRESENTATION" {
			continue
		}
		link, err := mixedConicalOccurrenceRecords(f, id, uses)
		if err != nil {
			return occ, ids, 0, err
		}
		if link.NAUOID == target {
			occ = link
			break
		}
	}
	from, err := mappedRationalFrame(f, occ.FromPlacementID)
	if err != nil {
		return occ, ids, 0, err
	}
	identity, err := independentTorusAssemblyPose(from)
	if err != nil || identity != geom.Identity() {
		return occ, ids, 0, fail
	}
	frame, err := simple(f, occ.ToPlacementID, "AXIS2_PLACEMENT_3D", 4)
	if err != nil {
		return occ, ids, 0, err
	}
	r, err := namedReferences(frame, 1, 3, 2)
	if err != nil || r[1] == r[2] {
		return occ, ids, 0, fail
	}
	// ReadAssembly has proved the target's IDT plus parent representation
	// membership. Require those to be its only references, and each
	// edited direction to be owned solely by this frame.
	incoming := conicalRepairIncoming(f)
	if incoming[occ.ToPlacementID] != 2 || incoming[r[1]] != 1 || incoming[r[2]] != 1 {
		return occ, ids, 0, fail
	}
	return occ, [2]int64{r[1], r[2]}, r[0], nil
}

func mixedConicalFrameRepairExtent(part ConicalRimFilletBoreSTEP) (*big.Rat, error) {
	shell, err := simple(part.Source, part.SourceShellID, "CLOSED_SHELL", 2)
	if err != nil {
		return nil, err
	}
	ids, err := listRefs(shell.Args[1], 6)
	if err != nil {
		return nil, err
	}
	faces, err := readConicalRimFilletBoreSTEPFaces(part.Source, ids)
	if err != nil {
		return nil, err
	}
	s := faces.sourceGeometry()
	extent := new(big.Rat)
	abs := func(x float64) *big.Rat {
		q := new(big.Rat).SetFloat64(x)
		return q.Abs(q)
	}
	bound := func(center, radius float64) {
		q := new(big.Rat).Add(abs(center), abs(radius))
		if q.Cmp(extent) > 0 {
			extent = q
		}
	}
	// The checked axial generators bound the cone and bore. Both cap
	// rims are enclosed by their associated torus support. Extrema of
	// the closed material are bounded by extrema of this boundary.
	for _, station := range s.ConeStations {
		bound(s.ConeCenter.X, station.Radius)
		bound(s.ConeCenter.Y, station.Radius)
		bound(station.Z, 0)
	}
	for i, rim := range s.Toroids {
		for _, center := range []float64{rim.Center.X, rim.Center.Y} {
			q := new(big.Rat).Add(abs(center), abs(rim.Major))
			q.Add(q, abs(rim.Minor))
			if q.Cmp(extent) > 0 {
				extent = q
			}
		}
		bound(rim.Center.Z, rim.Minor)
		bound(s.CapZ[i], 0)
	}
	bound(s.BoreCenter.X, s.BoreRadius)
	bound(s.BoreCenter.Y, s.BoreRadius)
	return extent, nil
}
