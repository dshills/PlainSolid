package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math/big"
	"reflect"
	"slices"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

type nativeMappedChainOwner struct {
	Surface           json.RawMessage
	Source            *nativeUse  `json:",omitempty"`
	Sources           []nativeUse `json:",omitempty"`
	Ranges            []UVInterval
	Weights, Stations []string
}
type nativeMappedChainBoundary struct {
	Version              int
	Kind                 string
	Owners               []nativeMappedChainOwner
	JointSupportChanges  json.RawMessage `json:",omitempty"`
	OrderedSupportEvents json.RawMessage `json:",omitempty"`
}

type nativeMappedChainSupportEvent struct {
	ModelStation string
	Owners       uint8
	Kind         string
}

func mappedChainNativeSupportEvents(proof MappedSplineChainAgreement) []nativeMappedChainSupportEvent {
	out := make([]nativeMappedChainSupportEvent, len(proof.joinedSupportEvents))
	for i, event := range proof.joinedSupportEvents {
		kind := "interior"
		if event.OriginalJoint {
			kind = "joint"
		}
		var mask uint8
		for owner, change := range event.OwnersChangingCells {
			if change {
				mask |= 1 << owner
			}
		}
		out[i] = nativeMappedChainSupportEvent{event.ModelStation.RatString(), mask, kind}
	}
	return out
}

// MappedChainNativeOptions bounds this separate native boundary recipe, not a
// trimmed Solid. Default MaxBytes is 1 MiB (hard maximum 32 MiB). Source curves are
// limited to 64 controls each; supports to 4096. Proof budgets apply independently
// to each bounded span/union cell as documented by the chain constructors.
type MappedChainNativeOptions struct {
	MaxBytes int64
	Proof    edgecheck.Options
}

// WriteJSON persists only original geometry and exact maps. Full source and
// per-cell correspondence are re-proved before any bytes are written. This is
// a standalone boundary format. Version 2 retains independently authored
// whole coedge bases in addition to version 1's whole owner spline.
// Versions 3 through 13 rederive the original support-knot repairs.
// Version 8 derives its bounded cut count from the original roof supports.
// Version 9 rederives separate support events on two or three original cells.
// Version 10 rederives two asynchronous owner events in one original cell.
// Version 11 rederives one owner-only event per original cell and the shared
// support events at their original joints.
// Version 12 retains and re-proves the owner change masks for one shared
// and one unilateral original support joint.
// Version 13 checks the ordered masks at retained original joints and at
// paired interior cuts in four to seven original cells.
func (r MappedSplineChainAgreement) WriteJSON(ctx context.Context, w io.Writer, o MappedChainNativeOptions) error {
	if ctx == nil || w == nil || len(r.cuts) < 2 {
		return geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	max, e := mappedChainNativeMax(o.MaxBytes)
	if e != nil {
		return e
	}
	record := nativeMappedChainBoundary{Version: 1, Kind: "mapped-spline-chain-boundary"}
	var chains [2]MappedSplineChain
	limits := geom.SurfacePatchOptions{MaxOperations: o.Proof.MaxWork, MaxCoefficientBits: o.Proof.MaxBits}
	for owner, c := range r.sources {
		proved, e := newMappedSplineChain(ctx, c.parts, c.stations, limits, c.independent)
		if e != nil {
			return e
		}
		chains[owner] = proved
		surface, e := parametric.EncodeSurface(r.supports[owner])
		if e != nil {
			return e
		}
		x := nativeMappedChainOwner{Surface: surface}
		encoder := trimEncoder{}
		if c.independent {
			record.Version = 2
			for _, part := range proved.parts {
				uses, e := encoder.encodeUses([]Use{{Curve: part.source}})
				if e != nil {
					return e
				}
				x.Sources = append(x.Sources, uses[0])
			}
		} else {
			source := proved.parts[0].source
			lo, hi := source.Curve.Domain()
			source.Range = UVInterval{lo, hi}
			uses, e := encoder.encodeUses([]Use{{Curve: source}})
			if e != nil {
				return e
			}
			x.Source = &uses[0]
		}
		for _, p := range proved.parts {
			x.Ranges = append(x.Ranges, p.source.Range)
			x.Weights = append(x.Weights, p.weight.RatString())
		}
		for _, s := range proved.stations {
			x.Stations = append(x.Stations, s.RatString())
		}
		record.Owners = append(record.Owners, x)
	}
	if r.knotRepair != mappedChainNoKnotRepair {
		record.Version = 2 + int(r.knotRepair)
	}
	proof, e := certifyMappedSplineChainAgreement(ctx, r.supports[0], chains[0], r.supports[1], chains[1], o.Proof, r.knotRepair)
	if e != nil {
		return e
	}
	switch r.knotRepair {
	case mappedChainAsymmetricJoinedKnotsRepair:
		record.JointSupportChanges, e = json.Marshal(proof.OriginalJointSupportChanges())
	case mappedChainOrderedJoinedKnotsRepair:
		record.OrderedSupportEvents, e = json.Marshal(mappedChainNativeSupportEvents(proof))
	}
	if e != nil {
		return e
	}
	raw, e := json.Marshal(record)
	if e != nil {
		return e
	}
	if int64(len(raw)) > max {
		return geom.ErrConvergence
	}
	if e = ctx.Err(); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		e = io.ErrShortWrite
	}
	return e
}
func mappedChainNativeMax(n int64) (int64, error) {
	if n == 0 {
		n = 1 << 20
	}
	if n < 1 || n > 32<<20 {
		return 0, geom.ErrInvalid
	}
	return n, nil
}

// ReadMappedSplineChainBoundaryJSON reconstructs the immutable original-span
// proof and shared exact model positions. All source ranges, maps, C0 joins and
// complete original support compositions are revalidated. The result can
// supply certified polylines or face fills. ReadMappedSplineChainShellJSON
// loads complete original closing faces and seam incidence separately.
func ReadMappedSplineChainBoundaryJSON(ctx context.Context, r io.Reader, o MappedChainNativeOptions) (MappedSplineChainAgreement, error) {
	var zero MappedSplineChainAgreement
	if ctx == nil || r == nil {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	maxBytes, e := mappedChainNativeMax(o.MaxBytes)
	if e != nil {
		return zero, e
	}
	raw, e := io.ReadAll(io.LimitReader(r, maxBytes+1))
	if e != nil {
		return zero, e
	}
	if int64(len(raw)) > maxBytes {
		return zero, geom.ErrConvergence
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var record nativeMappedChainBoundary
	if e = decoder.Decode(&record); e != nil {
		return zero, e
	}
	var extra any
	if e = decoder.Decode(&extra); e != io.EOF {
		return zero, geom.ErrInvalid
	}
	if record.Version < 1 || record.Version > 13 || record.Kind != "mapped-spline-chain-boundary" || len(record.Owners) != 2 {
		return zero, geom.ErrInvalid
	}
	if record.Version != 12 && len(record.JointSupportChanges) != 0 ||
		record.Version != 13 && len(record.OrderedSupportEvents) != 0 {
		return zero, geom.ErrInvalid
	}
	var chains [2]MappedSplineChain
	var surfaces [2]geom.Surface
	limits := geom.SurfacePatchOptions{MaxOperations: o.Proof.MaxWork, MaxCoefficientBits: o.Proof.MaxBits}
	var anyIndependent bool
	for owner, x := range record.Owners {
		if e = ctx.Err(); e != nil {
			return zero, e
		}
		if len(x.Ranges) < 2 || len(x.Ranges) > 8 || len(x.Weights) != len(x.Ranges) || len(x.Stations) != len(x.Ranges)-1 {
			return zero, geom.ErrInvalid
		}
		independent := len(x.Sources) != 0
		var source SplineUV
		if independent {
			if record.Version == 1 || x.Source != nil || len(x.Sources) != len(x.Ranges) {
				return zero, geom.ErrInvalid
			}
			anyIndependent = true
		} else {
			if x.Source == nil {
				return zero, geom.ErrInvalid
			}
			source, e = decodeMappedChainNativeSource(ctx, *x.Source)
			if e != nil {
				return zero, e
			}
		}
		surface, e := parametric.DecodeSurface(x.Surface, 4096)
		if e != nil {
			return zero, e
		}
		surfaces[owner] = surface
		var parts []MappedSplineUV
		var stations []*big.Rat
		for i, span := range x.Ranges {
			weight, e := parseNativeExactTrimCoefficient(x.Weights[i])
			if e != nil {
				return zero, e
			}
			part := source
			if independent {
				part, e = decodeMappedChainNativeSource(ctx, x.Sources[i])
				if e != nil {
					return zero, e
				}
				if part.Range != span {
					return zero, geom.ErrConvergence
				}
			} else {
				part.Range = span
			}
			q, e := NewMappedSplineUV(ctx, part, weight, limits)
			if e != nil {
				return zero, e
			}
			parts = append(parts, q)
		}
		for _, s := range x.Stations {
			v, e := parseNativeExactTrimCoefficient(s)
			if e != nil {
				return zero, e
			}
			stations = append(stations, v)
		}
		chains[owner], e = newMappedSplineChain(ctx, parts, stations, limits, independent)
		if e != nil {
			return zero, e
		}
	}
	if record.Version == 2 && !anyIndependent {
		return zero, geom.ErrInvalid
	}
	repair := mappedChainKnotRepair(max(0, record.Version-2))
	proof, e := certifyMappedSplineChainAgreement(ctx, surfaces[0], chains[0], surfaces[1], chains[1], o.Proof, repair)
	if e != nil {
		return zero, e
	}
	switch repair {
	case mappedChainAsymmetricJoinedKnotsRepair:
		jointChanges := proof.OriginalJointSupportChanges()
		var declared [][]*bool
		if e := json.Unmarshal(record.JointSupportChanges, &declared); e != nil || len(declared) != len(jointChanges) {
			return zero, geom.ErrInvalid
		}
		for i, changes := range jointChanges {
			if len(declared[i]) != len(changes) {
				return zero, geom.ErrInvalid
			}
			for owner, change := range changes {
				if declared[i][owner] == nil || *declared[i][owner] != change {
					return zero, geom.ErrConvergence
				}
			}
		}
	case mappedChainOrderedJoinedKnotsRepair:
		var declared []nativeMappedChainSupportEvent
		reader := json.NewDecoder(bytes.NewReader(record.OrderedSupportEvents))
		reader.DisallowUnknownFields()
		if e := reader.Decode(&declared); e != nil {
			return zero, geom.ErrInvalid
		}
		if !slices.Equal(declared, mappedChainNativeSupportEvents(proof)) {
			return zero, geom.ErrConvergence
		}
	}
	return proof, nil
}

func decodeMappedChainNativeSource(ctx context.Context, raw nativeUse) (SplineUV, error) {
	var zero SplineUV
	allowed := nativeUse{Spline: raw.Spline, SplineRange: raw.SplineRange, SplineReversed: raw.SplineReversed, SplineExactEndpoints: raw.SplineExactEndpoints}
	if len(raw.Spline) == 0 || !reflect.DeepEqual(allowed, raw) {
		return zero, geom.ErrInvalid
	}
	d := trimDecoder{ctx: ctx, version: 8, maxControls: 64}
	uses, err := d.decodeUses([]nativeUse{raw}, 0)
	if err != nil {
		return zero, err
	}
	source, ok := uses[0].Curve.(SplineUV)
	if !ok {
		return zero, geom.ErrInvalid
	}
	lo, hi := source.Curve.Domain()
	if source.Range != (UVInterval{lo, hi}) {
		return zero, geom.ErrConvergence
	}
	return source, nil
}
