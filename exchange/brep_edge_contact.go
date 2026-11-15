package exchange

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

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
	Edges     [2]int       `json:"edges"`
	Faces     [2][2]int    `json:"faces"`
	Endpoints [2]geom.Vec3 `json:"endpoints"`
	Reversed  bool         `json:"reversed"`
	Volume    string       `json:"volume"`
}

func edgeWitness(p collision.PlanarEdgeContact) edgeContactPairWitness {
	return edgeContactPairWitness{
		Edges: [2]int{p.EdgeA, p.EdgeB}, Faces: [2][2]int{p.FacesA, p.FacesB},
		Endpoints: [2]geom.Vec3{p.Start, p.End}, Reversed: p.Reversed, Volume: p.SignedVolume.RatString(),
	}
}

// WriteEdgeContactPairJSON writes the separate original native part graphs
// and the exact full-edge witness. No welded topology or mesh is persisted.
func WriteEdgeContactPairJSON(ctx context.Context, w io.Writer, parts [2]brep.Solid, o EdgeContactPairJSONOptions) error {
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
	for i, s := range parts {
		doc.Components[i] = brepDocument{1, "planar-brep", s.Vertices(), s.Edges(), s.Faces()}
		if err := checkPlanarAssemblyComponent(doc.Components[i], component, maxBytes); err != nil {
			return fmt.Errorf("edge contact component %d: %w", i, err)
		}
	}
	p, err := collision.DiscoverPlanarEdgeContact(ctx, parts[0], parts[1], o.Proof)
	if err != nil {
		return err
	}
	witness := edgeWitness(p)
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
	if err := d.Decode(&extra); err != io.EOF || doc.Version != 1 || doc.Kind != "planar-edge-contact-pair" ||
		len(doc.Components) != 2 || doc.Witness == nil {
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
	p, err := collision.ValidatePlanarEdgeContact(ctx, parts[0], parts[1], doc.Witness.Edges[0], doc.Witness.Edges[1], o.Proof)
	if err != nil {
		return zero, empty, err
	}
	if edgeWitness(p) != *doc.Witness {
		return zero, empty, fmt.Errorf("%w: original edge contact witness changed", collision.ErrInvalid)
	}
	return parts, p, nil
}
