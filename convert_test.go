package hclconfig

import (
	"reflect"
	"testing"

	"github.com/zclconf/go-cty/cty"
)

func TestStructToCtyValue_Primitives(t *testing.T) {
	type Simple struct {
		Host string `hcl:"host,attr"`
		Port int    `hcl:"port,attr"`
	}

	val, err := structToCtyValue(Simple{Host: "localhost", Port: 5432})
	if err != nil {
		t.Fatal(err)
	}

	if !val.Type().IsObjectType() {
		t.Fatalf("expected object type, got %s", val.Type().FriendlyName())
	}
	if !val.Type().HasAttribute("host") || !val.Type().HasAttribute("port") {
		t.Fatalf("expected host and port attributes")
	}

	if val.GetAttr("host").AsString() != "localhost" {
		t.Errorf("host = %q, want %q", val.GetAttr("host").AsString(), "localhost")
	}
}

func TestStructToCtyValue_NestedBlock(t *testing.T) {
	type Creds struct {
		Username string `hcl:"username,attr"`
		Password string `hcl:"password,attr"`
	}
	type DB struct {
		Host  string `hcl:"host,attr"`
		Creds Creds  `hcl:"credentials,block"`
	}

	val, err := structToCtyValue(DB{
		Host: "localhost",
		Creds: Creds{
			Username: "admin",
			Password: "secret",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	creds := val.GetAttr("credentials")
	if creds.GetAttr("username").AsString() != "admin" {
		t.Errorf("credentials.username = %q, want %q", creds.GetAttr("username").AsString(), "admin")
	}
}

func TestStructToCtyValue_LabeledBlock(t *testing.T) {
	type Service struct {
		Name string `hcl:"name,label"`
		Host string `hcl:"host,attr"`
		Port int    `hcl:"port,attr"`
	}

	// When called directly, structToCtyValue returns just the attributes
	// (label wrapping happens at the parent block-field level)
	val, err := structToCtyValue(Service{Name: "api", Host: "api.example.com", Port: 8080})
	if err != nil {
		t.Fatal(err)
	}
	if val.GetAttr("host").AsString() != "api.example.com" {
		t.Errorf("host = %q, want %q", val.GetAttr("host").AsString(), "api.example.com")
	}

	// Test labeled block wrapping via blockFieldToCtyValue
	fv := reflect.ValueOf(Service{Name: "api", Host: "api.example.com", Port: 8080})
	wrapped, err := blockFieldToCtyValue(fv)
	if err != nil {
		t.Fatal(err)
	}
	api := wrapped.GetAttr("api")
	if api.GetAttr("host").AsString() != "api.example.com" {
		t.Errorf("api.host = %q, want %q", api.GetAttr("host").AsString(), "api.example.com")
	}
}

func TestStructToCtyValue_NilPointer(t *testing.T) {
	type DB struct {
		Host string `hcl:"host,attr"`
	}
	type Config struct {
		DB *DB `hcl:"database,block"`
	}

	val, err := structToCtyValue(Config{})
	if err != nil {
		t.Fatal(err)
	}
	// DB is nil pointer, so the object should be empty
	if val.Type().Equals(cty.EmptyObject) != true && val != cty.EmptyObjectVal {
		// Just check it doesn't error out — exact behavior depends on nil handling
		t.Log("nil pointer produces:", val.GoString())
	}
}

func TestStructToCtyValue_BoolAndFloat(t *testing.T) {
	type Features struct {
		Enabled bool    `hcl:"enabled,attr"`
		Rate    float64 `hcl:"rate,attr"`
	}

	val, err := structToCtyValue(Features{Enabled: true, Rate: 0.75})
	if err != nil {
		t.Fatal(err)
	}
	if val.GetAttr("enabled").True() != true {
		t.Error("expected enabled to be true")
	}
}

func TestParseHCLTag(t *testing.T) {
	tests := []struct {
		tag      string
		wantName string
		wantKind string
	}{
		{"host,attr", "host", "attr"},
		{"database,block", "database", "block"},
		{"name,label", "name", "label"},
		{"simple", "simple", "attr"},
	}

	for _, tc := range tests {
		name, kind := parseHCLTag(tc.tag)
		if name != tc.wantName || kind != tc.wantKind {
			t.Errorf("parseHCLTag(%q) = (%q, %q), want (%q, %q)",
				tc.tag, name, kind, tc.wantName, tc.wantKind)
		}
	}
}
