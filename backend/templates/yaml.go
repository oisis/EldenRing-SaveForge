package templates

import (
	"bytes"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

// MarshalBuildTemplateYAML serialises a validated BuildTemplate to YAML.
// The output is a public sharing format — same v1 semantic payload as
// the JSON exporter, just a different on-the-wire representation. Keys
// and structure match schema.go's yaml: tags exactly, including the
// literal "inventory.workspace" section key with a dot.
//
// Indentation is set to 2 spaces so the file diffs cleanly and matches
// the human-readable feel of the JSON exporter's MarshalIndent output.
func MarshalBuildTemplateYAML(tpl *BuildTemplate) ([]byte, error) {
	if tpl == nil {
		return nil, fmt.Errorf("MarshalBuildTemplateYAML: nil template")
	}
	if err := ValidateBuildTemplate(tpl); err != nil {
		return nil, fmt.Errorf("MarshalBuildTemplateYAML: %w", err)
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(tpl); err != nil {
		_ = enc.Close()
		return nil, fmt.Errorf("MarshalBuildTemplateYAML: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("MarshalBuildTemplateYAML: %w", err)
	}
	return buf.Bytes(), nil
}

// ParseBuildTemplateYAML decodes a YAML public payload into a
// BuildTemplate and runs the structural validator. Decoding is strict:
// unknown YAML fields are rejected via KnownFields(true). Typed
// struct-targeted decode means YAML custom tags cannot reroute fields
// to alternate Go types — the decoder only honours the yaml: tags
// declared on schema.go's structs.
//
// Empty payloads are rejected up front; this matches the contract of
// ParseBuildTemplateJSON. Returned errors are flat (not wrapped into a
// preview report) — the App-level caller decides how to translate a
// parse failure into the right UX (typically a single
// IssueCodeStructureInvalid issue).
//
// Multi-document YAML is refused: the public template format is exactly
// one document per file. After successfully decoding the first document
// into the typed BuildTemplate, a second decode attempt must return
// io.EOF; any other result (a second document, even empty or malformed)
// fails the whole payload. This prevents a confused-deputy attack where
// a sharable YAML file hides a second document behind a leading `---`
// separator that some consumers would silently drop. The second decode
// targets a yaml.Node sentinel so the rejection logic does not depend
// on the second document also satisfying BuildTemplate / KnownFields —
// the strict typed decode of the first document remains the only path
// by which template content is accepted.
func ParseBuildTemplateYAML(data []byte) (*BuildTemplate, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, fmt.Errorf("ParseBuildTemplateYAML: empty payload")
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var tpl BuildTemplate
	if err := dec.Decode(&tpl); err != nil {
		return nil, fmt.Errorf("ParseBuildTemplateYAML: %w", err)
	}
	var sentinel yaml.Node
	if err := dec.Decode(&sentinel); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("ParseBuildTemplateYAML: multi-document YAML payloads are not supported")
		}
		return nil, fmt.Errorf("ParseBuildTemplateYAML: multi-document YAML payloads are not supported: %w", err)
	}
	if err := ValidateBuildTemplate(&tpl); err != nil {
		return nil, fmt.Errorf("ParseBuildTemplateYAML: %w", err)
	}
	return &tpl, nil
}
