package hclconfig

import (
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
)

func TestTopoSort_NoDeps(t *testing.T) {
	infos := []blockInfo{
		{typeName: "database", index: 0},
		{typeName: "app", index: 1},
	}
	deps := map[string]map[string]bool{
		"database": {},
		"app":      {},
	}

	sorted, err := topoSort(infos, deps)
	if err != nil {
		t.Fatal(err)
	}
	if len(sorted) != 2 {
		t.Fatalf("expected 2 items, got %d", len(sorted))
	}
}

func TestTopoSort_WithDeps(t *testing.T) {
	infos := []blockInfo{
		{typeName: "database", index: 0},
		{typeName: "app", index: 1},
	}
	deps := map[string]map[string]bool{
		"database": {},
		"app":      {"database": true},
	}

	sorted, err := topoSort(infos, deps)
	if err != nil {
		t.Fatal(err)
	}

	// database must come before app
	dbIdx, appIdx := -1, -1
	for i, k := range sorted {
		if k == "database" {
			dbIdx = i
		}
		if k == "app" {
			appIdx = i
		}
	}
	if dbIdx >= appIdx {
		t.Errorf("database (idx=%d) should come before app (idx=%d)", dbIdx, appIdx)
	}
}

func TestTopoSort_Cycle(t *testing.T) {
	infos := []blockInfo{
		{typeName: "alpha", index: 0},
		{typeName: "beta", index: 1},
	}
	deps := map[string]map[string]bool{
		"alpha": {"beta": true},
		"beta":  {"alpha": true},
	}

	_, err := topoSort(infos, deps)
	if err == nil {
		t.Fatal("expected cycle error")
	}
	cycleErr, ok := err.(*CycleError)
	if !ok {
		t.Fatalf("expected CycleError, got %T", err)
	}
	if len(cycleErr.Cycle) < 2 {
		t.Errorf("cycle too short: %v", cycleErr.Cycle)
	}
}

func TestBuildDependencyGraph(t *testing.T) {
	src := []byte(`
database {
    host = "localhost"
    port = 5432
}
app {
    db_url = "postgres://${database.host}:${database.port}/mydb"
}
`)
	parser := hclparse.NewParser()
	file, diags := parser.ParseHCL(src, "test.hcl")
	if diags.HasErrors() {
		t.Fatal(diags.Error())
	}

	schema := &hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{Type: "database"},
			{Type: "app"},
		},
	}

	content, diags := file.Body.Content(schema)
	if diags.HasErrors() {
		t.Fatal(diags.Error())
	}

	infos := []blockInfo{
		{typeName: "database", index: 0},
		{typeName: "app", index: 1},
	}

	deps := buildDependencyGraph(content.Blocks, infos)

	if !deps["app"]["database"] {
		t.Errorf("expected app to depend on database, got: %v", deps["app"])
	}
	if len(deps["database"]) != 0 {
		t.Errorf("expected database to have no deps, got: %v", deps["database"])
	}
}

func TestTopoSort_LabeledBlocks(t *testing.T) {
	infos := []blockInfo{
		{typeName: "service", label: "api", index: 0},
		{typeName: "service", label: "web", index: 1},
		{typeName: "app", index: 2},
	}
	deps := map[string]map[string]bool{
		"service.api": {},
		"service.web": {"service.api": true},
		"app":         {"service.api": true, "service.web": true},
	}

	sorted, err := topoSort(infos, deps)
	if err != nil {
		t.Fatal(err)
	}

	idxOf := func(key string) int {
		for i, k := range sorted {
			if k == key {
				return i
			}
		}
		return -1
	}

	if idxOf("service.api") >= idxOf("service.web") {
		t.Error("service.api should come before service.web")
	}
	if idxOf("service.web") >= idxOf("app") {
		t.Error("service.web should come before app")
	}
}
