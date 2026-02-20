package hclconfig

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

// blockInfo holds metadata about a block or top-level attribute for dependency analysis.
type blockInfo struct {
	typeName string
	label    string // empty for unlabeled blocks and attributes
	index    int    // position in the original block list
	isAttr   bool   // true if this represents a top-level attribute
}

func (b blockInfo) key() string {
	if b.label != "" {
		return b.typeName + "." + b.label
	}
	return b.typeName
}

// buildDependencyGraph analyzes blocks and top-level attributes, returning a
// map of node key -> set of node keys it depends on.
func buildDependencyGraph(blocks []*hcl.Block, blockInfos []blockInfo, attrs map[string]*hcl.Attribute) map[string]map[string]bool {
	// Build set of known names (block types + attribute names)
	knownTypes := make(map[string]bool)
	for _, bi := range blockInfos {
		knownTypes[bi.typeName] = true
	}
	for name := range attrs {
		knownTypes[name] = true
	}

	// Combine block and attribute infos for labeled-block lookups in addDependency
	var allInfos []blockInfo
	allInfos = append(allInfos, blockInfos...)
	for name := range attrs {
		allInfos = append(allInfos, blockInfo{typeName: name, isAttr: true})
	}

	deps := make(map[string]map[string]bool)

	// Analyze block dependencies
	for i, block := range blocks {
		bi := blockInfos[i]
		key := bi.key()
		if deps[key] == nil {
			deps[key] = make(map[string]bool)
		}

		bodyAttrs, _ := block.Body.JustAttributes()
		for _, attr := range bodyAttrs {
			for _, traversal := range attr.Expr.Variables() {
				addDependency(deps, key, traversal, knownTypes, allInfos)
			}
		}

		if syntaxBody, ok := block.Body.(*hclsyntax.Body); ok {
			extractNestedBlockDeps(deps, key, syntaxBody.Blocks, knownTypes, allInfos)
		}
	}

	// Analyze top-level attribute dependencies
	for name, attr := range attrs {
		if deps[name] == nil {
			deps[name] = make(map[string]bool)
		}
		for _, traversal := range attr.Expr.Variables() {
			addDependency(deps, name, traversal, knownTypes, allInfos)
		}
	}

	return deps
}

func extractNestedBlockDeps(deps map[string]map[string]bool, parentKey string, blocks []*hclsyntax.Block, knownTypes map[string]bool, blockInfos []blockInfo) {
	for _, block := range blocks {
		attrs, _ := block.Body.JustAttributes()
		for _, attr := range attrs {
			for _, traversal := range attr.Expr.Variables() {
				addDependency(deps, parentKey, traversal, knownTypes, blockInfos)
			}
		}
		// Recurse into deeper nested blocks
		extractNestedBlockDeps(deps, parentKey, block.Body.Blocks, knownTypes, blockInfos)
	}
}

func addDependency(deps map[string]map[string]bool, fromKey string, traversal hcl.Traversal, knownTypes map[string]bool, blockInfos []blockInfo) {
	if len(traversal) == 0 {
		return
	}

	root := traversal.RootName()
	if !knownTypes[root] {
		return
	}

	// Check if it references a specific label: e.g. service.api.port
	// The traversal would be: root="service", then GetAttr "api", then GetAttr "port"
	targetKey := root
	if len(traversal) > 1 {
		if attr, ok := traversal[1].(hcl.TraverseAttr); ok {
			// Check if there's a labeled block with this label
			for _, bi := range blockInfos {
				if bi.typeName == root && bi.label == attr.Name {
					targetKey = bi.key()
					break
				}
			}
		}
	}

	// Don't add self-dependency
	if targetKey != fromKey {
		if deps[fromKey] == nil {
			deps[fromKey] = make(map[string]bool)
		}
		deps[fromKey][targetKey] = true
	}
}

// topoSort performs a topological sort using Kahn's algorithm.
// Returns the sorted order of block keys and an error if cycles are detected.
func topoSort(blockInfos []blockInfo, deps map[string]map[string]bool) ([]string, error) {
	// Build unique keys in order
	seen := make(map[string]bool)
	var keys []string
	for _, bi := range blockInfos {
		k := bi.key()
		if !seen[k] {
			seen[k] = true
			keys = append(keys, k)
		}
	}

	// Ensure all keys are in the deps map
	for _, k := range keys {
		if deps[k] == nil {
			deps[k] = make(map[string]bool)
		}
	}

	// Calculate in-degrees
	inDegree := make(map[string]int)
	for _, k := range keys {
		inDegree[k] = 0
	}
	for node, nodeDeps := range deps {
		if !seen[node] {
			continue
		}
		for dep := range nodeDeps {
			if seen[dep] {
				inDegree[node]++
			}
		}
	}

	// Queue nodes with no dependencies
	var queue []string
	for _, k := range keys {
		if inDegree[k] == 0 {
			queue = append(queue, k)
		}
	}

	var sorted []string
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		sorted = append(sorted, node)

		// For each node that depends on the current node, decrease its in-degree
		for _, k := range keys {
			if deps[k][node] {
				inDegree[k]--
				if inDegree[k] == 0 {
					queue = append(queue, k)
				}
			}
		}
	}

	if len(sorted) != len(keys) {
		// Find the cycle
		cycle := findCycle(keys, deps)
		return nil, &CycleError{Cycle: cycle}
	}

	return sorted, nil
}

func findCycle(keys []string, deps map[string]map[string]bool) []string {
	// Simple DFS-based cycle detection
	visited := make(map[string]int) // 0=unvisited, 1=in-stack, 2=done
	parent := make(map[string]string)

	var cycle []string
	var dfs func(node string) bool
	dfs = func(node string) bool {
		visited[node] = 1
		for dep := range deps[node] {
			if visited[dep] == 1 {
				// Found cycle — reconstruct
				cycle = []string{dep, node}
				cur := node
				for cur != dep {
					cur = parent[cur]
					if cur == dep {
						break
					}
					cycle = append(cycle, cur)
				}
				// Reverse and add closing node
				for i, j := 0, len(cycle)-1; i < j; i, j = i+1, j-1 {
					cycle[i], cycle[j] = cycle[j], cycle[i]
				}
				cycle = append(cycle, dep)
				return true
			}
			if visited[dep] == 0 {
				parent[dep] = node
				if dfs(dep) {
					return true
				}
			}
		}
		visited[node] = 2
		return false
	}

	for _, k := range keys {
		if visited[k] == 0 {
			if dfs(k) {
				return cycle
			}
		}
	}
	return []string{"unknown cycle"}
}
