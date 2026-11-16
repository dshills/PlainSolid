package step

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"reflect"
	"slices"

	"plainsolid/geom"
)

// MixedConicalAP242DAGRepairOptions specifies a displacement bound in the
// common length unit of the original assembly.
type MixedConicalAP242DAGRepairOptions struct {
	MaxDistance float64
}

// RepairedMixedConicalAP242DAG binds the original AP242 bytes to the exact
// repaired local shapes and preserved occurrences.
type RepairedMixedConicalAP242DAG struct {
	original, corrected string
	maxDistance         float64
	assembly            MixedConicalAP242DAG
	parts               [2]NestedMixedConicalAP242PartRepair
}

// OriginalSTEP returns the byte-preserved original assembly.
func (m RepairedMixedConicalAP242DAG) OriginalSTEP() string { return m.original }

// CorrectedModelSTEP returns the strictly validated repaired assembly.
func (m RepairedMixedConicalAP242DAG) CorrectedModelSTEP() string { return m.corrected }

// Assembly returns the strict DAG with its original occurrence identities.
func (m RepairedMixedConicalAP242DAG) Assembly() MixedConicalAP242DAG {
	a := m.assembly
	mixed := cloneRepairedMixedConicalAP242(MixedConicalAP242Assembly{Assembly: a.Assembly, Parts: a.Parts, Occurrences: a.Occurrences})
	a.Assembly, a.Parts, a.Occurrences = mixed.Assembly, mixed.Parts, mixed.Occurrences
	a.Instances = slices.Clone(a.Instances)
	for i := range a.Instances {
		a.Instances[i].Path = slices.Clone(a.Instances[i].Path)
	}
	return a
}

// PartRepairs returns the proof of each original local definition.
func (m RepairedMixedConicalAP242DAG) PartRepairs() [2]NestedMixedConicalAP242PartRepair {
	return cloneMixedConicalAP242PartRepairs(m.parts)
}

// RepairMixedConicalAP242DAG authenticates the original DAG and bounds its
// expansion before repairing its two original local definitions. The merge
// retains all original relationships, exact variable-length paths, shared
// subassembly identities and unmodified source record IDs.
func RepairMixedConicalAP242DAG(ctx context.Context, r io.Reader, l MixedConicalAP242DAGLimits, o MixedConicalAP242DAGRepairOptions) (RepairedMixedConicalAP242DAG, error) {
	var zero RepairedMixedConicalAP242DAG
	if ctx == nil || r == nil || !finiteNumber(o.MaxDistance) || o.MaxDistance <= 0 {
		return zero, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	l, err := mixedConicalAP242DAGLimits(l)
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
	h, err := readMixedConicalAP242HierarchyMode(bytes.NewReader(raw), l.Source, mixedConicalAP242DAG)
	if err != nil {
		return zero, fmt.Errorf("step: authenticate rounded AP242 DAG: %w", err)
	}
	worlds, err := mixedConicalAP242DAGWorlds(ctx, h.assembly.Roots[0], h.records.occurrences, l)
	if err != nil {
		return zero, err
	}
	corrected, parts, err := repairMixedConicalAP242Parts(ctx, h, l.Source, o.MaxDistance)
	if err != nil {
		return zero, err
	}
	a, err := ReadMixedConicalAP242DAG(ctx, bytes.NewReader(corrected), l, geom.SurfacePatchOptions{})
	if err != nil {
		return zero, fmt.Errorf("step: verify corrected AP242 DAG: %w", err)
	}
	post := MixedConicalAP242Assembly{Assembly: a.Assembly, Parts: a.Parts, Occurrences: a.Occurrences}
	if err := verifyMixedConicalAP242Repair(h, post, parts); err != nil {
		return zero, err
	}
	if !reflect.DeepEqual(worlds, a.Instances) {
		return zero, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	return RepairedMixedConicalAP242DAG{original: string(raw), corrected: string(corrected), maxDistance: o.MaxDistance,
		assembly: a, parts: parts}, nil
}
