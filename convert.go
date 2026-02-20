package hclconfig

import (
	"fmt"
	"reflect"

	"github.com/zclconf/go-cty/cty"
)

// structToCtyValue converts a Go struct with `hcl` struct tags into a cty.Value
// suitable for use in an HCL EvalContext. This is necessary because gocty uses
// `cty` struct tags, but consumer structs use `hcl` struct tags.
func structToCtyValue(v interface{}) (cty.Value, error) {
	return reflectToCtyValue(reflect.ValueOf(v))
}

func reflectToCtyValue(rv reflect.Value) (cty.Value, error) {
	// Dereference pointers
	for rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return cty.NilVal, nil
		}
		rv = rv.Elem()
	}

	switch rv.Kind() {
	case reflect.String:
		return cty.StringVal(rv.String()), nil
	case reflect.Bool:
		return cty.BoolVal(rv.Bool()), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return cty.NumberIntVal(rv.Int()), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return cty.NumberUIntVal(rv.Uint()), nil
	case reflect.Float32, reflect.Float64:
		return cty.NumberFloatVal(rv.Float()), nil
	case reflect.Struct:
		return structFieldsToCtyObject(rv)
	case reflect.Slice:
		return sliceToCtyValue(rv)
	case reflect.Map:
		return mapToCtyValue(rv)
	default:
		return cty.NilVal, fmt.Errorf("unsupported kind %s", rv.Kind())
	}
}

func structFieldsToCtyObject(rv reflect.Value) (cty.Value, error) {
	rt := rv.Type()
	attrs := make(map[string]cty.Value)

	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		tag := field.Tag.Get("hcl")
		if tag == "" || tag == "-" {
			continue
		}

		name, kind := parseHCLTag(tag)

		fv := rv.Field(i)

		switch kind {
		case "attr", "optional":
			val, err := reflectToCtyValue(fv)
			if err != nil {
				return cty.NilVal, fmt.Errorf("field %s: %w", name, err)
			}
			if val != cty.NilVal {
				attrs[name] = val
			}

		case "block":
			val, err := blockFieldToCtyValue(fv)
			if err != nil {
				return cty.NilVal, fmt.Errorf("block %s: %w", name, err)
			}
			if val != cty.NilVal {
				attrs[name] = val
			}

		case "label":
			// Labels are not added to the eval context
			continue
		}
	}

	if len(attrs) == 0 {
		return cty.EmptyObjectVal, nil
	}
	return cty.ObjectVal(attrs), nil
}

func blockFieldToCtyValue(fv reflect.Value) (cty.Value, error) {
	// Dereference pointer
	for fv.Kind() == reflect.Ptr {
		if fv.IsNil() {
			return cty.NilVal, nil
		}
		fv = fv.Elem()
	}

	switch fv.Kind() {
	case reflect.Struct:
		// Check if this is a labeled block by looking for a "label" tagged field
		if hasLabelField(fv.Type()) {
			// Single labeled block — wrap in map by label
			return labeledBlockToMap(fv)
		}
		return structFieldsToCtyObject(fv)

	case reflect.Slice:
		// Slice of blocks
		if fv.Len() == 0 {
			return cty.NilVal, nil
		}
		elemType := fv.Type().Elem()
		for elemType.Kind() == reflect.Ptr {
			elemType = elemType.Elem()
		}
		if elemType.Kind() == reflect.Struct && hasLabelField(elemType) {
			return labeledBlockSliceToMap(fv)
		}
		return sliceToCtyValue(fv)

	default:
		return reflectToCtyValue(fv)
	}
}

func labeledBlockToMap(rv reflect.Value) (cty.Value, error) {
	label := getLabelValue(rv)
	if label == "" {
		return structFieldsToCtyObject(rv)
	}
	val, err := structFieldsToCtyObject(rv)
	if err != nil {
		return cty.NilVal, err
	}
	return cty.ObjectVal(map[string]cty.Value{label: val}), nil
}

func labeledBlockSliceToMap(rv reflect.Value) (cty.Value, error) {
	m := make(map[string]cty.Value)
	for i := 0; i < rv.Len(); i++ {
		elem := rv.Index(i)
		for elem.Kind() == reflect.Ptr {
			if elem.IsNil() {
				continue
			}
			elem = elem.Elem()
		}
		label := getLabelValue(elem)
		val, err := structFieldsToCtyObject(elem)
		if err != nil {
			return cty.NilVal, err
		}
		m[label] = val
	}
	if len(m) == 0 {
		return cty.NilVal, nil
	}
	return cty.ObjectVal(m), nil
}

func hasLabelField(rt reflect.Type) bool {
	for rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
	}
	for i := 0; i < rt.NumField(); i++ {
		tag := rt.Field(i).Tag.Get("hcl")
		if tag == "" {
			continue
		}
		_, kind := parseHCLTag(tag)
		if kind == "label" {
			return true
		}
	}
	return false
}

func getLabelValue(rv reflect.Value) string {
	for rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		tag := rt.Field(i).Tag.Get("hcl")
		if tag == "" {
			continue
		}
		_, kind := parseHCLTag(tag)
		if kind == "label" {
			return rv.Field(i).String()
		}
	}
	return ""
}

func sliceToCtyValue(rv reflect.Value) (cty.Value, error) {
	if rv.Len() == 0 {
		return cty.EmptyTupleVal, nil
	}
	vals := make([]cty.Value, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		v, err := reflectToCtyValue(rv.Index(i))
		if err != nil {
			return cty.NilVal, err
		}
		vals[i] = v
	}
	return cty.TupleVal(vals), nil
}

func mapToCtyValue(rv reflect.Value) (cty.Value, error) {
	if rv.Len() == 0 {
		return cty.EmptyObjectVal, nil
	}
	attrs := make(map[string]cty.Value)
	for _, key := range rv.MapKeys() {
		v, err := reflectToCtyValue(rv.MapIndex(key))
		if err != nil {
			return cty.NilVal, err
		}
		attrs[key.String()] = v
	}
	return cty.ObjectVal(attrs), nil
}

func parseHCLTag(tag string) (name, kind string) {
	// Tags are "name,kind" e.g. "host,attr" or "database,block"
	for i := 0; i < len(tag); i++ {
		if tag[i] == ',' {
			return tag[:i], tag[i+1:]
		}
	}
	return tag, "attr"
}
