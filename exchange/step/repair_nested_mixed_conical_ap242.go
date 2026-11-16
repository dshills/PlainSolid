package step

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"reflect"
	"slices"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// NestedMixedConicalAP242RepairOptions bounds displacement in the common
// length unit of the original assembly. Exact rigid occurrences preserve it.
type NestedMixedConicalAP242RepairOptions struct {
	MaxDistance float64
}

// NestedMixedConicalAP242PartRepair records a single repaired local
// definition shared by all its occurrences. GeneratedEntityIDs maps added
// entity IDs in the isolated corrected part to their IDs in the assembly.
type NestedMixedConicalAP242PartRepair struct {
	DefinitionID, BodyID, ShellID                         int64
	LocalSourceSHA256, LocalCorrectedSHA256, NativeSHA256 string
	FaceIDs, EdgeIDs, VertexIDs                           []int64
	GeneratedEntityIDs                                    map[int64]int64
	RollingRim                                            *ConicalRimFilletBoreRepairEvidence
	ChamferWalls                                          []EccentricAnnularChamferWallRepair
}

// RepairedNestedMixedConicalAP242Assembly retains both STEP files, the
// strictly reconstructed nested assembly and its per-definition repairs.
type RepairedNestedMixedConicalAP242Assembly struct {
	original, corrected string
	maxDistance         float64
	assembly            NestedMixedConicalAP242Assembly
	parts               [2]NestedMixedConicalAP242PartRepair
}

// OriginalSTEP returns the unmodified AP242 source.
func (m RepairedNestedMixedConicalAP242Assembly) OriginalSTEP() string { return m.original }

// CorrectedModelSTEP returns the strictly reopenable repaired AP242 model.
func (m RepairedNestedMixedConicalAP242Assembly) CorrectedModelSTEP() string { return m.corrected }

// Assembly returns the strictly reconstructed material and hierarchy.
func (m RepairedNestedMixedConicalAP242Assembly) Assembly() NestedMixedConicalAP242Assembly {
	a := m.assembly
	mixed := cloneRepairedMixedConicalAP242(MixedConicalAP242Assembly{Assembly: a.Assembly, Parts: a.Parts, Occurrences: a.Occurrences})
	a.Assembly, a.Parts, a.Occurrences = mixed.Assembly, mixed.Parts, mixed.Occurrences
	a.Group.Representations = slices.Clone(a.Group.Representations)
	a.Instances = slices.Clone(a.Instances)
	return a
}

func cloneRepairedMixedConicalAP242(a MixedConicalAP242Assembly) MixedConicalAP242Assembly {
	a.Assembly.Roots = slices.Clone(a.Assembly.Roots)
	a.Assembly.Occurrences = slices.Clone(a.Assembly.Occurrences)
	a.Assembly.Products = slices.Clone(a.Assembly.Products)
	for i := range a.Assembly.Products {
		p := &a.Assembly.Products[i]
		p.Representations = slices.Clone(p.Representations)
	}
	a.Parts = maps.Clone(a.Parts)
	for id, part := range a.Parts {
		part.Product.Representations = slices.Clone(part.Product.Representations)
		if part.Rounded != nil {
			p := *part.Rounded
			p.NativeFaceSourceIDs = slices.Clone(p.NativeFaceSourceIDs)
			p.NativeEdgeSourceIDs = slices.Clone(p.NativeEdgeSourceIDs)
			p.NativeVertexSourceIDs = slices.Clone(p.NativeVertexSourceIDs)
			part.Rounded = &p
		}
		if part.Chamfered != nil {
			p := *part.Chamfered
			p.NativeFaceSourceIDs = slices.Clone(p.NativeFaceSourceIDs)
			p.NativeEdgeSourceIDs = slices.Clone(p.NativeEdgeSourceIDs)
			p.NativeVertexSourceIDs = slices.Clone(p.NativeVertexSourceIDs)
			part.Chamfered = &p
		}
		a.Parts[id] = part
	}
	a.Occurrences = slices.Clone(a.Occurrences)
	return a
}

// PartRepairs returns the two original local definitions' proof claims.
func (m RepairedNestedMixedConicalAP242Assembly) PartRepairs() [2]NestedMixedConicalAP242PartRepair {
	return cloneMixedConicalAP242PartRepairs(m.parts)
}

func cloneMixedConicalAP242PartRepairs(out [2]NestedMixedConicalAP242PartRepair) [2]NestedMixedConicalAP242PartRepair {
	for i := range out {
		p := &out[i]
		p.FaceIDs, p.EdgeIDs, p.VertexIDs = slices.Clone(p.FaceIDs), slices.Clone(p.EdgeIDs), slices.Clone(p.VertexIDs)
		p.GeneratedEntityIDs, p.ChamferWalls = maps.Clone(p.GeneratedEntityIDs), slices.Clone(p.ChamferWalls)
		if p.RollingRim != nil {
			q := *p.RollingRim
			q.Frames = slices.Clone(q.Frames)
			p.RollingRim = &q
		}
	}
	return out
}

var errNestedMixedConicalAP242Repair = errors.New("step: unsupported bounded nested mixed conical AP242 repair")

// RepairNestedMixedConicalAP242Assembly authenticates the original AP242
// occurrence graph, repairs each of its two original local solids once and
// verifies the merged assembly with the ordinary strict nested reader.
// Every retained entity keeps its source ID. Only generated local support
// entities receive fresh assembly IDs.
func RepairNestedMixedConicalAP242Assembly(ctx context.Context, r io.Reader, l Limits, o NestedMixedConicalAP242RepairOptions) (RepairedNestedMixedConicalAP242Assembly, error) {
	var zero RepairedNestedMixedConicalAP242Assembly
	if ctx == nil || r == nil || !finiteNumber(o.MaxDistance) || o.MaxDistance <= 0 {
		return zero, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	l, err := occurrenceColorSourceLimits(l)
	if err != nil {
		return zero, err
	}
	original, err := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if err != nil {
		return zero, err
	}
	if int64(len(original)) > l.MaxBytes {
		return zero, geom.ErrConvergence
	}
	h, err := readMixedConicalAP242Hierarchy(bytes.NewReader(original), l, true)
	if err != nil {
		return zero, fmt.Errorf("step: authenticate rounded nested AP242 hierarchy: %w", err)
	}
	corrected, parts, err := repairMixedConicalAP242Parts(ctx, h, l, o.MaxDistance)
	if err != nil {
		return zero, err
	}
	material, err := ReadNestedMixedConicalAP242Assembly(ctx, bytes.NewReader(corrected), l, geom.SurfacePatchOptions{})
	if err != nil {
		return zero, fmt.Errorf("step: verify corrected nested AP242 assembly: %w", err)
	}
	if err := verifyNestedMixedConicalRepair(h, material, parts); err != nil {
		return zero, err
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	return RepairedNestedMixedConicalAP242Assembly{original: string(original), corrected: string(corrected), maxDistance: o.MaxDistance,
		assembly: material, parts: parts}, nil
}

func repairMixedConicalAP242Parts(ctx context.Context, h mixedConicalAP242Hierarchy, l Limits, distance float64) ([]byte, [2]NestedMixedConicalAP242PartRepair, error) {
	return repairMixedConicalAP242PartsInto(ctx, h, h.assembly.Source, l, distance)
}

// The enclosing source can also retain authenticated presentation records
// erased from the private geometry proof. Their references and IDs must count
// for local ownership and fresh entity allocation.
func repairMixedConicalAP242PartsInto(ctx context.Context, h mixedConicalAP242Hierarchy, source *File, l Limits, distance float64) ([]byte, [2]NestedMixedConicalAP242PartRepair, error) {
	fixed := cloneOvalFile(source)
	fixed.Order = slices.Clone(source.Order)
	incoming := conicalRepairIncoming(source)
	freshID := slices.Max(source.Order)
	var parts [2]NestedMixedConicalAP242PartRepair
	n := 0
	for _, product := range h.assembly.Products {
		bodyID, isPart := h.layout.parts[product.DefinitionID]
		if !isPart {
			continue
		}
		if err := ctx.Err(); err != nil {
			return nil, parts, err
		}
		roots := []int64{h.records.definitionShapes[product.DefinitionID], h.records.protocol, h.records.categories[product.ProductID]}
		view, err := mixedConicalAP242ViewFile(h.assembly.Source, roots, l)
		if err != nil {
			return nil, parts, err
		}
		p, after, err := repairNestedMixedConicalLocalPart(ctx, view, l, distance, product.DefinitionID, bodyID, h.assembly.LengthUnit)
		if err != nil {
			return nil, parts, fmt.Errorf("step: repair nested AP242 part #%d: %w", product.DefinitionID, err)
		}
		p.GeneratedEntityIDs, err = mergeNestedMixedConicalPart(fixed, view, after, bodyID, incoming, &freshID, l)
		if err != nil {
			return nil, parts, err
		}
		for i := range p.ChamferWalls {
			wall := &p.ChamferWalls[i]
			wall.CorrectedGeneratorID = p.GeneratedEntityIDs[wall.CorrectedGeneratorID]
			if wall.CorrectedGeneratorID == 0 {
				return nil, parts, errNestedMixedConicalAP242Repair
			}
		}
		parts[n] = p
		n++
	}
	if n != len(parts) {
		return nil, parts, errNestedMixedConicalAP242Repair
	}
	fixed.Order = slices.DeleteFunc(fixed.Order, func(id int64) bool { _, ok := fixed.Entities[id]; return !ok })
	corrected, err := mappedPlanarSourceBytes(fixed)
	return corrected, parts, err
}

func repairNestedMixedConicalLocalPart(ctx context.Context, view *File, l Limits, distance float64, definitionID, bodyID int64, unit float64) (NestedMixedConicalAP242PartRepair, *File, error) {
	var p NestedMixedConicalAP242PartRepair
	raw, err := mappedPlanarSourceBytes(view)
	if err != nil {
		return p, nil, err
	}
	p.DefinitionID, p.BodyID = definitionID, bodyID
	p.LocalSourceSHA256 = rationalPosedRepairDigest(raw)
	var corrected *File
	var native trimmed.Solid
	var sourceBody int64
	var sourceUnit float64
	if hasMixedConicalToroid(view, bodyID) {
		m, err := RepairConicalRimFilletBore(ctx, bytes.NewReader(raw), l, ConicalRimFilletBoreRepairOptions{distance})
		if err != nil {
			return p, nil, err
		}
		q, evidence := m.Material(), m.Evidence()
		p.RollingRim = &evidence
		p.LocalCorrectedSHA256 = rationalPosedRepairDigest([]byte(m.CorrectedModelSTEP()))
		p.ShellID, p.FaceIDs, p.EdgeIDs, p.VertexIDs = q.SourceShellID, q.NativeFaceSourceIDs, q.NativeEdgeSourceIDs, q.NativeVertexSourceIDs
		sourceBody, sourceUnit, native, corrected = q.SourceBodyID, q.LengthUnit, q.Solid, q.Source
	} else {
		m, err := RepairEccentricAnnularChamfer(ctx, bytes.NewReader(raw), l, EccentricAnnularChamferRepairOptions{distance})
		if err != nil {
			return p, nil, err
		}
		q, walls := m.Material(), m.Walls()
		p.ChamferWalls = walls[:]
		p.LocalCorrectedSHA256 = rationalPosedRepairDigest([]byte(m.CorrectedModelSTEP()))
		p.ShellID, p.FaceIDs, p.EdgeIDs, p.VertexIDs = q.SourceShellID, q.NativeFaceSourceIDs, q.NativeEdgeSourceIDs, q.NativeVertexSourceIDs
		sourceBody, sourceUnit, native, corrected = q.SourceBodyID, q.LengthUnit, q.Solid, q.Source
	}
	if sourceBody != bodyID || sourceUnit != unit {
		return p, nil, errNestedMixedConicalAP242Repair
	}
	nativeBytes, err := prismNativeBytes(native)
	if err != nil {
		return p, nil, err
	}
	p.NativeSHA256 = rationalPosedRepairDigest(nativeBytes)
	return p, corrected, nil
}

func verifyNestedMixedConicalRepair(before mixedConicalAP242Hierarchy, after NestedMixedConicalAP242Assembly, parts [2]NestedMixedConicalAP242PartRepair) error {
	if !reflect.DeepEqual(before.layout.group, after.Group) {
		return errNestedMixedConicalAP242Repair
	}
	return verifyMixedConicalAP242Repair(before, MixedConicalAP242Assembly{
		Assembly: after.Assembly, Parts: after.Parts, Occurrences: after.Occurrences,
	}, parts)
}

func verifyMixedConicalAP242Repair(before mixedConicalAP242Hierarchy, after MixedConicalAP242Assembly, parts [2]NestedMixedConicalAP242PartRepair) error {
	if before.assembly.LengthUnit != after.Assembly.LengthUnit || !slices.Equal(before.assembly.Roots, after.Assembly.Roots) ||
		!reflect.DeepEqual(before.assembly.Products, after.Assembly.Products) ||
		!reflect.DeepEqual(before.records.occurrences, after.Occurrences) || len(after.Parts) != len(parts) {
		return errNestedMixedConicalAP242Repair
	}
	for _, p := range parts {
		part, ok := after.Parts[p.DefinitionID]
		if !ok {
			return errNestedMixedConicalAP242Repair
		}
		var solid trimmed.Solid
		var body, shell int64
		var faces, edges, vertices []int64
		switch {
		case p.RollingRim != nil && part.Rounded != nil && part.Chamfered == nil:
			q := part.Rounded
			body, shell, faces, edges, vertices, solid = q.SourceBodyID, q.SourceShellID, q.NativeFaceSourceIDs, q.NativeEdgeSourceIDs, q.NativeVertexSourceIDs, q.Solid
		case len(p.ChamferWalls) == 6 && part.Chamfered != nil && part.Rounded == nil:
			q := part.Chamfered
			body, shell, faces, edges, vertices, solid = q.SourceBodyID, q.SourceShellID, q.NativeFaceSourceIDs, q.NativeEdgeSourceIDs, q.NativeVertexSourceIDs, q.Solid
		default:
			return errNestedMixedConicalAP242Repair
		}
		if p.BodyID != body || p.ShellID != shell || !slices.Equal(p.FaceIDs, faces) || !slices.Equal(p.EdgeIDs, edges) || !slices.Equal(p.VertexIDs, vertices) {
			return errNestedMixedConicalAP242Repair
		}
		data, err := prismNativeBytes(solid)
		if err != nil {
			return err
		}
		if p.NativeSHA256 != rationalPosedRepairDigest(data) {
			return errNestedMixedConicalAP242Repair
		}
	}
	return nil
}
