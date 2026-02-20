package hclconfig

import (
	"fmt"
	"os"
	"reflect"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/zclconf/go-cty/cty"
)

// Option configures the behavior of Load/LoadFile.
type Option func(*options)

type options struct {
	evalCtx *hcl.EvalContext
}

// WithEvalContext provides a custom HCL EvalContext that will be merged with
// the built-in context (env function, resolved block variables).
func WithEvalContext(ctx *hcl.EvalContext) Option {
	return func(o *options) {
		o.evalCtx = ctx
	}
}

// LoadFile reads and parses an HCL file with cross-block variable resolution.
func LoadFile(filename string, dst interface{}, opts ...Option) error {
	src, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("reading %s: %w", filename, err)
	}
	return Load(src, filename, dst, opts...)
}

// Load parses HCL source bytes with cross-block variable resolution.
func Load(src []byte, filename string, dst interface{}, opts ...Option) error {
	var o options
	for _, opt := range opts {
		opt(&o)
	}

	// 1. Parse
	parser := hclparse.NewParser()
	file, diags := parser.ParseHCL(src, filename)
	if diags.HasErrors() {
		return &DiagnosticsError{Diags: diags}
	}

	body := file.Body

	// 2. Extract schema from target struct
	schema, _ := gohcl.ImpliedBodySchema(dst)

	// 3. Extract blocks
	content, diags := body.Content(schema)
	if diags.HasErrors() {
		return &DiagnosticsError{Diags: diags}
	}

	// 4. Build block info list
	blockInfos := make([]blockInfo, len(content.Blocks))
	for i, block := range content.Blocks {
		label := ""
		if len(block.Labels) > 0 {
			label = block.Labels[0]
		}
		blockInfos[i] = blockInfo{
			typeName: block.Type,
			label:    label,
			index:    i,
		}
	}

	// 5. Build dependency graph and topological sort
	deps := buildDependencyGraph(content.Blocks, blockInfos)
	sortedKeys, err := topoSort(blockInfos, deps)
	if err != nil {
		return err
	}

	// 6. Build eval context
	evalCtx := newBaseEvalContext(o.evalCtx)

	// 7. Decode blocks in topological order
	dstVal := reflect.ValueOf(dst).Elem()
	dstType := dstVal.Type()

	// Build a map from block type -> field info
	type fieldInfo struct {
		fieldIndex int
		isSlice    bool
		isPtr      bool
	}
	fieldMap := make(map[string]fieldInfo)
	for i := 0; i < dstType.NumField(); i++ {
		field := dstType.Field(i)
		tag := field.Tag.Get("hcl")
		if tag == "" {
			continue
		}
		name, kind := parseHCLTag(tag)
		if kind == "block" {
			ft := field.Type
			isPtr := ft.Kind() == reflect.Ptr
			isSlice := ft.Kind() == reflect.Slice
			fieldMap[name] = fieldInfo{
				fieldIndex: i,
				isSlice:    isSlice,
				isPtr:      isPtr,
			}
		}
	}

	// Group blocks by key for decoding
	blocksByKey := make(map[string][]*hcl.Block)
	blockInfoByKey := make(map[string][]blockInfo)
	for i, bi := range blockInfos {
		key := bi.key()
		blocksByKey[key] = append(blocksByKey[key], content.Blocks[i])
		blockInfoByKey[key] = append(blockInfoByKey[key], bi)
	}

	for _, key := range sortedKeys {
		blocks := blocksByKey[key]
		if len(blocks) == 0 {
			continue
		}

		// Determine the block type name (first part of key)
		typeName := blocks[0].Type
		fi, ok := fieldMap[typeName]
		if !ok {
			continue
		}

		fieldVal := dstVal.Field(fi.fieldIndex)

		if fi.isSlice {
			// Slice of blocks (including labeled)
			err := decodeSliceBlocks(fieldVal, blocks, evalCtx)
			if err != nil {
				return err
			}
		} else if fi.isPtr {
			// Optional single block
			elemType := fieldVal.Type().Elem()
			newVal := reflect.New(elemType)
			diags := gohcl.DecodeBody(blocks[0].Body, evalCtx, newVal.Interface())
			if diags.HasErrors() {
				return &DiagnosticsError{Diags: diags}
			}
			fieldVal.Set(newVal)
		} else {
			// Single block
			diags := gohcl.DecodeBody(blocks[0].Body, evalCtx, fieldVal.Addr().Interface())
			if diags.HasErrors() {
				return &DiagnosticsError{Diags: diags}
			}
		}

		// After decoding, add to eval context
		infos := blockInfoByKey[key]
		if fi.isSlice && len(infos) > 0 && infos[0].label != "" {
			// Labeled blocks in a slice — each gets added to eval context under its label
			addLabeledSliceToEvalCtx(evalCtx, typeName, fieldVal)
		} else if fi.isSlice {
			// Unlabeled slice — add as the type name
			val, err := structToCtyValue(fieldVal.Interface())
			if err == nil && val != cty.NilVal {
				evalCtx.Variables[typeName] = val
			}
		} else {
			// Single block
			var iface interface{}
			if fi.isPtr {
				if !fieldVal.IsNil() {
					iface = fieldVal.Elem().Interface()
				}
			} else {
				iface = fieldVal.Interface()
			}
			if iface != nil {
				val, err := structToCtyValue(iface)
				if err == nil && val != cty.NilVal {
					evalCtx.Variables[typeName] = val
				}
			}
		}
	}

	return nil
}

func decodeSliceBlocks(fieldVal reflect.Value, blocks []*hcl.Block, evalCtx *hcl.EvalContext) error {
	elemType := fieldVal.Type().Elem()
	isElemPtr := elemType.Kind() == reflect.Ptr
	if isElemPtr {
		elemType = elemType.Elem()
	}

	for _, block := range blocks {
		newVal := reflect.New(elemType)
		// Set label fields before decoding
		setLabelFields(newVal.Elem(), block.Labels)

		diags := gohcl.DecodeBody(block.Body, evalCtx, newVal.Interface())
		if diags.HasErrors() {
			return &DiagnosticsError{Diags: diags}
		}

		if isElemPtr {
			fieldVal.Set(reflect.Append(fieldVal, newVal))
		} else {
			fieldVal.Set(reflect.Append(fieldVal, newVal.Elem()))
		}
	}
	return nil
}

func setLabelFields(rv reflect.Value, labels []string) {
	rt := rv.Type()
	labelIdx := 0
	for i := 0; i < rt.NumField(); i++ {
		tag := rt.Field(i).Tag.Get("hcl")
		if tag == "" {
			continue
		}
		_, kind := parseHCLTag(tag)
		if kind == "label" && labelIdx < len(labels) {
			rv.Field(i).SetString(labels[labelIdx])
			labelIdx++
		}
	}
}

func addLabeledSliceToEvalCtx(evalCtx *hcl.EvalContext, typeName string, sliceVal reflect.Value) {
	labelMap := make(map[string]cty.Value)
	for i := 0; i < sliceVal.Len(); i++ {
		elem := sliceVal.Index(i)
		for elem.Kind() == reflect.Ptr {
			elem = elem.Elem()
		}
		label := getLabelValue(elem)
		if label == "" {
			continue
		}
		val, err := structFieldsToCtyObject(elem)
		if err == nil && val != cty.NilVal {
			labelMap[label] = val
		}
	}
	if len(labelMap) > 0 {
		evalCtx.Variables[typeName] = cty.ObjectVal(labelMap)
	}
}
