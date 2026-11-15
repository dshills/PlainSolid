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
	"plainsolid/geom"
)

// EdgeContactPairJSONOptions bounds the overall receipt, the two original
// planar boundaries, and the exact proof independently.
type EdgeContactPairJSONOptions struct {
	MaxBytes  int64
	Component BRepLimits
	Proof     collision.PlanarEdgeContactOptions
}

type edgeContactPairDocument struct {
	Version    int                     `json:"version"`
	Kind       string                  `json:"kind"`
	Components []brepDocument          `json:"components"`
	Witness    *edgeContactPairWitness `json:"witness"`
}

type edgeContactPairWitness struct {
	Edges     []int       `json:"edges"`
	Faces     [][]int     `json:"faces"`
	Endpoints []geom.Vec3 `json:"endpoints"`
	Reversed  bool        `json:"reversed"`
	Volume    string      `json:"volume"`
	Intervals [][]string  `json:"intervals,omitempty"`
}

func edgeWitness(p collision.PlanarEdgeContact, interval bool) edgeContactPairWitness {
	w := edgeContactPairWitness{
		Edges: []int{p.EdgeA, p.EdgeB}, Faces: [][]int{p.FacesA[:], p.FacesB[:]},
		Endpoints: []geom.Vec3{p.Start, p.End}, Reversed: p.Reversed, Volume: p.SignedVolume.RatString(),
	}
	if interval {
		w.Intervals = [][]string{
			{p.ParameterA[0].RatString(), p.ParameterA[1].RatString()},
			{p.ParameterB[0].RatString(), p.ParameterB[1].RatString()},
		}
	}
	return w
}

// WriteEdgeContactPairJSON writes the separate original native part graphs
// and the exact full-edge witness. No welded topology or mesh is persisted.
func WriteEdgeContactPairJSON(ctx context.Context, w io.Writer, parts [2]brep.Solid, o EdgeContactPairJSONOptions) error {
	return writeEdgeContactPairJSON(ctx, w, parts, o, nil)
}

// WriteEdgeIntervalContactPairJSON preserves two specified original edge IDs
// and the exact common interval on each without welding or modifying either
// part. The persisted ranges are canonical rational strings.
func WriteEdgeIntervalContactPairJSON(ctx context.Context, w io.Writer, parts [2]brep.Solid, edges [2]int, o EdgeContactPairJSONOptions) error {
	return writeEdgeContactPairJSON(ctx, w, parts, o, &edges)
}

func writeEdgeContactPairJSON(ctx context.Context, w io.Writer, parts [2]brep.Solid, o EdgeContactPairJSONOptions, edges *[2]int) error {
	if ctx == nil || w == nil {
		return collision.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	maxBytes, component, err := planarAssemblyJSONBounds(o.MaxBytes, o.Component)
	if err != nil {
		return err
	}
	doc := edgeContactPairDocument{Version: 1, Kind: "planar-edge-contact-pair", Components: make([]brepDocument, 2)}
	if edges != nil {
		doc.Kind = "planar-edge-interval-contact-pair"
	}
	for i, s := range parts {
		doc.Components[i] = brepDocument{1, "planar-brep", s.Vertices(), s.Edges(), s.Faces()}
		if err := checkPlanarAssemblyComponent(doc.Components[i], component, maxBytes); err != nil {
			return fmt.Errorf("edge contact component %d: %w", i, err)
		}
	}
	var p collision.PlanarEdgeContact
	if edges == nil {
		p, err = collision.DiscoverPlanarEdgeContact(ctx, parts[0], parts[1], o.Proof)
	} else {
		p, err = collision.ValidatePlanarEdgeIntervalContact(ctx, parts[0], parts[1], edges[0], edges[1], o.Proof)
	}
	if err != nil {
		return err
	}
	witness := edgeWitness(p, edges != nil)
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

// ReadEdgeContactPairJSON reconstructs both original part graphs, re-proves
// their exact contact, and checks every persisted witness field.
func ReadEdgeContactPairJSON(ctx context.Context, r io.Reader, o EdgeContactPairJSONOptions) ([2]brep.Solid, collision.PlanarEdgeContact, error) {
	return readEdgeContactPairJSON(ctx, r, o, false)
}

// ReadEdgeIntervalContactPairJSON re-proves the two original boundaries and
// both exact directed original edge ranges from their source coordinates.
func ReadEdgeIntervalContactPairJSON(ctx context.Context, r io.Reader, o EdgeContactPairJSONOptions) ([2]brep.Solid, collision.PlanarEdgeContact, error) {
	return readEdgeContactPairJSON(ctx, r, o, true)
}

func readEdgeContactPairJSON(ctx context.Context, r io.Reader, o EdgeContactPairJSONOptions, interval bool) ([2]brep.Solid, collision.PlanarEdgeContact, error) {
	var zero [2]brep.Solid
	var empty collision.PlanarEdgeContact
	if ctx == nil || r == nil {
		return zero, empty, collision.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, empty, err
	}
	maxBytes, component, err := planarAssemblyJSONBounds(o.MaxBytes, o.Component)
	if err != nil {
		return zero, empty, err
	}
	raw, err := io.ReadAll(io.LimitReader(r, maxBytes+1))
	if err != nil {
		return zero, empty, err
	}
	if int64(len(raw)) > maxBytes {
		return zero, empty, collision.ErrLimit
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	var doc edgeContactPairDocument
	if err := d.Decode(&doc); err != nil {
		return zero, empty, err
	}
	var extra any
	kind := "planar-edge-contact-pair"
	if interval {
		kind = "planar-edge-interval-contact-pair"
	}
	if err := d.Decode(&extra); err != io.EOF || doc.Version != 1 || doc.Kind != kind ||
		len(doc.Components) != 2 || doc.Witness == nil || len(doc.Witness.Edges) != 2 ||
		(doc.Witness.Intervals != nil) != interval {
		return zero, empty, collision.ErrInvalid
	}
	var parts [2]brep.Solid
	for i, c := range doc.Components {
		if err := ctx.Err(); err != nil {
			return zero, empty, err
		}
		if c.Version != 1 || c.Kind != "planar-brep" {
			return zero, empty, collision.ErrInvalid
		}
		if err := checkPlanarAssemblyComponent(c, component, maxBytes); err != nil {
			return zero, empty, fmt.Errorf("edge contact component %d: %w", i, err)
		}
		parts[i], err = brep.NewSolid(c.Vertices, c.Edges, c.Faces)
		if err != nil {
			return zero, empty, err
		}
	}
	var p collision.PlanarEdgeContact
	if interval {
		p, err = collision.ValidatePlanarEdgeIntervalContact(ctx, parts[0], parts[1], doc.Witness.Edges[0], doc.Witness.Edges[1], o.Proof)
	} else {
		p, err = collision.ValidatePlanarEdgeContact(ctx, parts[0], parts[1], doc.Witness.Edges[0], doc.Witness.Edges[1], o.Proof)
	}
	if err != nil {
		return zero, empty, err
	}
	if !reflect.DeepEqual(edgeWitness(p, interval), *doc.Witness) {
		return zero, empty, fmt.Errorf("%w: original edge contact witness changed", collision.ErrInvalid)
	}
	return parts, p, nil
}
