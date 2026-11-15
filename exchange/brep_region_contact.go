package exchange

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"

	"plainsolid/brep"
	"plainsolid/collision"
)

// PlanarRegionContactJSONOptions bounds the native parts and the exact proof
// separately from the complete file.
type PlanarRegionContactJSONOptions struct {
	MaxBytes  int64
	Component BRepLimits
	Proof     collision.PlanarFaceContactOptions
}

const planarRegionContactKind = "planar-region-contact-pair"

type planarRegionContactDocument struct {
	Version    int                         `json:"version"`
	Kind       string                      `json:"kind"`
	Components []brepDocument              `json:"components"`
	Witness    *planarRegionContactWitness `json:"witness"`
}

type planarRegionContactWitness struct {
	Faces               [][]int    `json:"faces"`
	Vertices            [][]int    `json:"vertices"`
	Edges               [][]int    `json:"edges"`
	TwiceProjectedAreas [][]string `json:"twiceProjectedAreas"`
	DropAxes            [][]int    `json:"dropAxes"`
	Volume              string     `json:"volume"`
}

func planarRegionWitness(p collision.PlanarRegionContact) planarRegionContactWitness {
	pairs := func(src [][2]int) [][]int {
		out := make([][]int, len(src))
		for i, a := range src {
			out[i] = []int{a[0], a[1]}
		}
		return out
	}
	w := planarRegionContactWitness{
		Faces: p.Faces[:], Vertices: pairs(p.Vertices), Edges: pairs(p.Edges),
		DropAxes: p.DropAxes[:], Volume: p.SignedVolume.RatString(),
		TwiceProjectedAreas: make([][]string, 2),
	}
	for i, areas := range p.TwiceProjectedAreas {
		for _, q := range areas {
			w.TwiceProjectedAreas[i] = append(w.TwiceProjectedAreas[i], q.RatString())
		}
	}
	return w
}

// WritePlanarRegionContactPairJSON writes two independent original boundary
// graphs and their exact contact witness. Different original face divisions
// are retained. No joined or tessellated shape is included in the file.
func WritePlanarRegionContactPairJSON(ctx context.Context, w io.Writer, parts [2]brep.Solid, o PlanarRegionContactJSONOptions) error {
	if ctx == nil || w == nil {
		return collision.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	maxBytes, opts, err := planarAssemblyJSONBounds(o.MaxBytes, o.Component)
	if err != nil {
		return err
	}
	doc := planarRegionContactDocument{Version: 1, Kind: planarRegionContactKind, Components: make([]brepDocument, 2)}
	for i, s := range parts {
		doc.Components[i] = brepDocument{1, "planar-brep", s.Vertices(), s.Edges(), s.Faces()}
		if err := checkPlanarAssemblyComponent(doc.Components[i], opts, maxBytes); err != nil {
			return fmt.Errorf("planar region component %d: %w", i, err)
		}
	}
	p, err := collision.DiscoverPlanarRegionContact(ctx, parts[0], parts[1], o.Proof)
	if err != nil {
		return err
	}
	witness := planarRegionWitness(p)
	doc.Witness = &witness
	raw, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	if int64(len(raw)) > maxBytes {
		return collision.ErrLimit
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	n, err := w.Write(raw)
	if err == nil && n != len(raw) {
		return io.ErrShortWrite
	}
	return err
}

// ReadPlanarRegionContactPairJSON reconstructs both original bodies, discovers
// the exact complete interface again, and compares every stored witness field
// before returning any material.
func ReadPlanarRegionContactPairJSON(ctx context.Context, r io.Reader, o PlanarRegionContactJSONOptions) ([2]brep.Solid, collision.PlanarRegionContact, error) {
	var zero [2]brep.Solid
	var none collision.PlanarRegionContact
	if ctx == nil || r == nil {
		return zero, none, collision.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, none, err
	}
	maxBytes, opts, err := planarAssemblyJSONBounds(o.MaxBytes, o.Component)
	if err != nil {
		return zero, none, err
	}
	raw, err := io.ReadAll(io.LimitReader(r, maxBytes+1))
	if err != nil {
		return zero, none, err
	}
	if int64(len(raw)) > maxBytes {
		return zero, none, collision.ErrLimit
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var doc planarRegionContactDocument
	if err := dec.Decode(&doc); err != nil {
		return zero, none, err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF || doc.Version != 1 || doc.Kind != planarRegionContactKind || len(doc.Components) != 2 || doc.Witness == nil {
		return zero, none, collision.ErrInvalid
	}
	var parts [2]brep.Solid
	for i, d := range doc.Components {
		if err := ctx.Err(); err != nil {
			return zero, none, err
		}
		if d.Version != 1 || d.Kind != "planar-brep" {
			return zero, none, collision.ErrInvalid
		}
		if err := checkPlanarAssemblyComponent(d, opts, maxBytes); err != nil {
			return zero, none, fmt.Errorf("planar region component %d: %w", i, err)
		}
		parts[i], err = brep.NewSolid(d.Vertices, d.Edges, d.Faces)
		if err != nil {
			return zero, none, err
		}
	}
	p, err := collision.DiscoverPlanarRegionContact(ctx, parts[0], parts[1], o.Proof)
	if err != nil {
		return zero, none, err
	}
	if !reflect.DeepEqual(planarRegionWitness(p), *doc.Witness) {
		return zero, none, fmt.Errorf("%w: stored original planar region witness changed", collision.ErrInvalid)
	}
	return parts, p, nil
}
