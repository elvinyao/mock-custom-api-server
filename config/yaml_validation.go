package config

import (
	"fmt"
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"
)

func decodeStrict(data []byte, target any) error {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return err
	}
	return decodeStrictNode(doc, target, "")
}

// Inspect the original nodes before decoding, so unknown keys retain both their
// original YAML line and their nested field path. Re-encoding a node loses these.
func decodeStrictNode(node yaml.Node, target any, field string) error {
	if err := checkYAMLFields(&node, reflect.TypeOf(target), field, make(map[*yaml.Node]bool)); err != nil {
		return err
	}
	return node.Decode(target)
}

func checkYAMLFields(node *yaml.Node, target reflect.Type, field string, visiting map[*yaml.Node]bool) error {
	if node == nil || visiting[node] {
		return nil
	}
	visiting[node] = true
	defer delete(visiting, node)
	for target.Kind() == reflect.Pointer {
		target = target.Elem()
	}
	// rawConfig intentionally keeps endpoints as a node until its format is known.
	if target == reflect.TypeFor[yaml.Node]() {
		return nil
	}
	if node.Kind == yaml.DocumentNode || node.Kind == yaml.AliasNode {
		if node.Kind == yaml.AliasNode {
			return checkYAMLFields(node.Alias, target, field, visiting)
		}
		if len(node.Content) > 0 {
			return checkYAMLFields(node.Content[0], target, field, visiting)
		}
		return nil
	}
	switch {
	case node.Kind == yaml.MappingNode && target.Kind() == reflect.Struct:
		fields := yamlStructFields(target)
		for i := 0; i < len(node.Content); i += 2 {
			key, value := node.Content[i], node.Content[i+1]
			if key.Tag == "!!merge" {
				merged := []*yaml.Node{value}
				if value.Kind == yaml.SequenceNode {
					merged = value.Content
				}
				for _, item := range merged {
					if err := checkYAMLFields(item, target, field, visiting); err != nil {
						return err
					}
				}
				continue
			}
			path := key.Value
			if field != "" {
				path = field + "." + path
			}
			fieldType, known := fields[key.Value]
			if !known {
				return fmt.Errorf("line %d: %s: unknown field %q", key.Line, path, key.Value)
			}
			if err := checkYAMLFields(value, fieldType, path, visiting); err != nil {
				return err
			}
		}
	case node.Kind == yaml.SequenceNode && (target.Kind() == reflect.Slice || target.Kind() == reflect.Array):
		for i, child := range node.Content {
			if err := checkYAMLFields(child, target.Elem(), fmt.Sprintf("%s[%d]", field, i), visiting); err != nil {
				return err
			}
		}
	case node.Kind == yaml.MappingNode && target.Kind() == reflect.Map:
		for i := 0; i < len(node.Content); i += 2 {
			if err := checkYAMLFields(node.Content[i+1], target.Elem(), fmt.Sprintf("%s[%s]", field, node.Content[i].Value), visiting); err != nil {
				return err
			}
		}
	default:
		// Decode only leaves and mismatched shapes here. Valid containers are
		// walked above, so checking their descendants remains linear in size.
		if err := node.Decode(reflect.New(target).Interface()); err != nil {
			return fmt.Errorf("%s: %w", field, err)
		}
	}
	// The final yaml.v3 decode also handles duplicate keys and invalid aliases.
	return nil
}

func yamlStructFields(target reflect.Type) map[string]reflect.Type {
	fields := make(map[string]reflect.Type)
	for i := 0; i < target.NumField(); i++ {
		field := target.Field(i)
		if !field.IsExported() {
			continue
		}
		name, options, _ := strings.Cut(field.Tag.Get("yaml"), ",")
		if name == "-" {
			continue
		}
		if strings.Contains(","+options+",", ",inline,") {
			for key, value := range yamlStructFields(field.Type) {
				fields[key] = value
			}
			continue
		}
		if name == "" {
			name = strings.ToLower(field.Name)
		}
		fields[name] = field.Type
	}
	return fields
}
