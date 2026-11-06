package claude

import (
	"encoding/json"
	"maps"
	"strings"

	"github.com/MoeForge/moego/ojson"
	kitutil "github.com/QuantumNous/new-api/relaykit/relayconvert/kitutil"
)

// FunctionParametersToInputSchema returns function parameters as a Claude
// input_schema, adding type and properties when they are missing or null.
// Top-level oneOf/anyOf/allOf are flattened into a plain object root because
// Anthropic rejects those composition keywords at the top level of
// input_schema; nested composition is left unchanged. Otherwise the client's
// bytes are kept so property order is preserved when no flatten is needed.
// ok is false when parameters are not a JSON object; the result is then an
// empty object schema.
func FunctionParametersToInputSchema(parameters json.RawMessage) (schema json.RawMessage, ok bool) {
	empty := json.RawMessage(`{"type":"object","properties":{}}`)
	switch kitutil.GetJsonType(parameters) {
	case "unknown", "null":
		return empty, true
	case "object":
	default:
		return empty, false
	}
	var fields struct {
		Type       json.RawMessage `json:"type"`
		Properties json.RawMessage `json:"properties"`
	}
	if err := kitutil.Unmarshal(parameters, &fields); err != nil {
		return empty, false
	}
	schema = parameters
	var err error
	if len(fields.Type) == 0 || kitutil.GetJsonType(fields.Type) == "null" {
		if schema, err = ojson.SetRawBytes(schema, "type", []byte(`"object"`)); err != nil {
			return empty, false
		}
	}
	if len(fields.Properties) == 0 || kitutil.GetJsonType(fields.Properties) == "null" {
		if schema, err = ojson.SetRawBytes(schema, "properties", []byte(`{}`)); err != nil {
			return empty, false
		}
	}
	flattened, changed, flattenOK := flattenTopLevelComposition(schema)
	if !flattenOK {
		return empty, false
	}
	if changed {
		return flattened, true
	}
	return schema, true
}

func flattenTopLevelComposition(schema json.RawMessage) (json.RawMessage, bool, bool) {
	var root map[string]any
	if err := kitutil.Unmarshal(schema, &root); err != nil {
		return nil, false, false
	}
	defs, _ := root["$defs"].(map[string]any)
	if defs == nil {
		defs, _ = root["definitions"].(map[string]any)
	}
	changed := false
	for _, key := range []string{"allOf", "anyOf", "oneOf"} {
		raw, exists := root[key]
		if !exists {
			continue
		}
		branches, ok := raw.([]any)
		if !ok || len(branches) == 0 {
			continue
		}
		objects := make([]map[string]any, 0, len(branches))
		for _, branch := range branches {
			resolved := resolveSchemaBranch(branch, defs, 0)
			for _, object := range objectSchemas(resolved, defs, 0) {
				objects = append(objects, object)
			}
		}
		if len(objects) == 0 {
			continue
		}
		mergeObjectSchemasIntoRoot(root, objects, key == "allOf")
		delete(root, key)
		changed = true
	}
	if !changed {
		return schema, false, true
	}
	if root["type"] == nil {
		root["type"] = "object"
	}
	if root["properties"] == nil {
		root["properties"] = map[string]any{}
	}
	out, err := kitutil.Marshal(root)
	if err != nil {
		return nil, false, false
	}
	return out, true, true
}

func resolveSchemaBranch(branch any, defs map[string]any, depth int) any {
	if depth > 8 {
		return branch
	}
	obj, ok := branch.(map[string]any)
	if !ok {
		return branch
	}
	ref, _ := obj["$ref"].(string)
	if ref == "" || len(obj) != 1 {
		return obj
	}
	name, ok := localDefName(ref)
	if !ok || defs == nil {
		return obj
	}
	target, exists := defs[name]
	if !exists {
		return obj
	}
	return resolveSchemaBranch(target, defs, depth+1)
}

func localDefName(ref string) (string, bool) {
	for _, prefix := range []string{"#/$defs/", "#/definitions/"} {
		if name, ok := strings.CutPrefix(ref, prefix); ok && name != "" && !strings.Contains(name, "/") {
			return name, true
		}
	}
	return "", false
}

func objectSchemas(schema any, defs map[string]any, depth int) []map[string]any {
	if depth > 8 {
		return nil
	}
	resolved := resolveSchemaBranch(schema, defs, 0)
	obj, ok := resolved.(map[string]any)
	if !ok {
		return nil
	}
	for _, key := range []string{"allOf", "anyOf", "oneOf"} {
		raw, exists := obj[key]
		if !exists {
			continue
		}
		branches, ok := raw.([]any)
		if !ok {
			continue
		}
		out := make([]map[string]any, 0, len(branches))
		for _, branch := range branches {
			out = append(out, objectSchemas(branch, defs, depth+1)...)
		}
		if len(out) > 0 {
			return out
		}
	}
	if isObjectSchema(obj) {
		return []map[string]any{obj}
	}
	return nil
}

func isObjectSchema(obj map[string]any) bool {
	if props, ok := obj["properties"].(map[string]any); ok && props != nil {
		return true
	}
	switch typ := obj["type"].(type) {
	case string:
		return typ == "object"
	case []any:
		for _, item := range typ {
			if s, ok := item.(string); ok && s == "object" {
				return true
			}
		}
	}
	return false
}

func mergeObjectSchemasIntoRoot(root map[string]any, objects []map[string]any, allOf bool) {
	existingProps, _ := root["properties"].(map[string]any)
	if existingProps == nil {
		existingProps = map[string]any{}
	}
	merged := map[string][]any{}
	for key, prop := range existingProps {
		merged[key] = []any{prop}
	}
	requiredSets := make([]map[string]struct{}, 0, len(objects))
	for _, object := range objects {
		props, _ := object["properties"].(map[string]any)
		for key, prop := range props {
			variants := merged[key]
			if !containsSchemaVariant(variants, prop) {
				merged[key] = append(variants, prop)
			}
		}
		requiredSets = append(requiredSets, requiredSet(object["required"]))
	}
	properties := make(map[string]any, len(merged))
	for key, variants := range merged {
		if len(variants) == 1 {
			properties[key] = variants[0]
			continue
		}
		properties[key] = map[string]any{"anyOf": variants}
	}
	root["properties"] = properties
	if required := mergeRequired(requiredSets, allOf); len(required) > 0 {
		root["required"] = required
	} else {
		delete(root, "required")
	}
}

func containsSchemaVariant(variants []any, candidate any) bool {
	cand, err := kitutil.Marshal(candidate)
	if err != nil {
		return false
	}
	for _, existing := range variants {
		got, err := kitutil.Marshal(existing)
		if err != nil {
			continue
		}
		if string(got) == string(cand) {
			return true
		}
	}
	return false
}

func requiredSet(raw any) map[string]struct{} {
	out := map[string]struct{}{}
	list, ok := raw.([]any)
	if !ok {
		return out
	}
	for _, item := range list {
		if name, ok := item.(string); ok && name != "" {
			out[name] = struct{}{}
		}
	}
	return out
}

func mergeRequired(sets []map[string]struct{}, allOf bool) []any {
	if len(sets) == 0 {
		return nil
	}
	var merged map[string]struct{}
	if allOf {
		merged = map[string]struct{}{}
		for _, set := range sets {
			for key := range set {
				merged[key] = struct{}{}
			}
		}
	} else {
		merged = maps.Clone(sets[0])
		for _, set := range sets[1:] {
			next := map[string]struct{}{}
			for key := range merged {
				if _, ok := set[key]; ok {
					next[key] = struct{}{}
				}
			}
			merged = next
		}
	}
	if len(merged) == 0 {
		return nil
	}
	keys := make([]any, 0, len(merged))
	for key := range merged {
		keys = append(keys, key)
	}
	return keys
}
