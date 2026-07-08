package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsBasicType(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"string is basic", "string", true},
		{"int is basic", "int", true},
		{"float64 is basic", "float64", true},
		{"bool is basic", "bool", true},
		{"custom type is not basic", "MyStruct", false},
		{"external type is not basic", "time.Duration", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isBasicType(tt.input); got != tt.want {
				t.Errorf("isBasicType(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestZeroValue(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"string", "string", `""`},
		{"int", "int", "0"},
		{"float64", "float64", "0"},
		{"bool", "bool", "false"},
		{"unknown", "MyStruct", "nil"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := zeroValue(tt.name); got != tt.want {
				t.Errorf("zeroValue(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestHasGenerateReset(t *testing.T) {
	tests := []struct {
		name string
		doc  string
		want bool
	}{
		{"has annotation", "// generate:reset", true},
		{"no annotation", "// some comment", false},
		{"nil doc", "", false},
		{"multiple comments", "// package doc\n// generate:reset\n", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cg *ast.CommentGroup
			if tt.doc != "" {
				lines := strings.Split(strings.TrimSuffix(tt.doc, "\n"), "\n")
				var list []*ast.Comment
				for _, l := range lines {
					list = append(list, &ast.Comment{Text: strings.TrimSpace(l)})
				}
				cg = &ast.CommentGroup{List: list}
			}
			if got := hasGenerateReset(cg); got != tt.want {
				t.Errorf("hasGenerateReset(%q) = %v, want %v", tt.doc, got, tt.want)
			}
		})
	}
}

func TestFieldName(t *testing.T) {
	tests := []struct {
		src  string
		want string
	}{
		{`Name string`, "Name"},
		{`X int`, "X"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, "", fmt.Sprintf("package p; type S struct { %s }", tt.src), 0)
			if err != nil {
				t.Fatal(err)
			}
			ts := f.Decls[0].(*ast.GenDecl).Specs[0].(*ast.TypeSpec)
			st := ts.Type.(*ast.StructType)
			fd := st.Fields.List[0]
			if got := fieldName(fd); got != tt.want {
				t.Errorf("fieldName(%q) = %q, want %q", tt.src, got, tt.want)
			}
		})
	}
}

func TestEmbeddedFieldName(t *testing.T) {
	tests := []struct {
		src  string
		want string
	}{
		{`time.Time`, "Time"},
		{`*MyStruct`, "MyStruct"},
		{`http.Handler`, "Handler"},
		{`int`, "int"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, "", fmt.Sprintf("package p; type S struct { %s }", tt.src), 0)
			if err != nil {
				t.Fatal(err)
			}
			ts := f.Decls[0].(*ast.GenDecl).Specs[0].(*ast.TypeSpec)
			st := ts.Type.(*ast.StructType)
			fd := st.Fields.List[0]
			if got := embeddedFieldName(fd.Type); got != tt.want {
				t.Errorf("embeddedFieldName(%q) = %q, want %q", tt.src, got, tt.want)
			}
		})
	}
}

func TestGenerateResetMethod_BasicTypes(t *testing.T) {
	gen := &Generator{fset: token.NewFileSet()}

	// Parse a struct with basic field types.
	src := "package test\ntype Config struct {\n\tName string\n\tCount int\n\tValue float64\n\tEnabled bool\n}"
	f, err := parser.ParseFile(gen.fset, "", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	ts := f.Decls[0].(*ast.GenDecl).Specs[0].(*ast.TypeSpec)
	st := ts.Type.(*ast.StructType)

	knownStructs := map[string]bool{}
	si := structInfo{name: "Config", fields: st.Fields.List}
	result := gen.generateResetMethod(si, knownStructs)

	if !strings.Contains(result, "func (rs *Config) Reset()") {
		t.Error("missing Reset function signature")
	}
	if !strings.Contains(result, `rs.Name = ""`) {
		t.Error("missing Name reset")
	}
	if !strings.Contains(result, "rs.Count = 0") {
		t.Error("missing Count reset")
	}
	if !strings.Contains(result, "rs.Value = 0") {
		t.Error("missing Value reset")
	}
	if !strings.Contains(result, "rs.Enabled = false") {
		t.Error("missing Enabled reset")
	}
}

func TestGenerateResetMethod_SliceAndMap(t *testing.T) {
	gen := &Generator{fset: token.NewFileSet()}
	src := "package test\ntype Data struct {\n\tItems []string\n\tLookup map[string]int\n}"
	f, err := parser.ParseFile(gen.fset, "", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	ts := f.Decls[0].(*ast.GenDecl).Specs[0].(*ast.TypeSpec)
	st := ts.Type.(*ast.StructType)

	si := structInfo{name: "Data", fields: st.Fields.List}
	result := gen.generateResetMethod(si, map[string]bool{})

	if !strings.Contains(result, "rs.Items = rs.Items[:0]") {
		t.Error("missing slice truncation")
	}
	if !strings.Contains(result, "clear(rs.Lookup)") {
		t.Error("missing map clear")
	}
}

func TestGenerateResetMethod_PointerField(t *testing.T) {
	gen := &Generator{fset: token.NewFileSet()}
	src := "package test\ntype Outer struct {\n\tInner *Inner\n}"
	f, err := parser.ParseFile(gen.fset, "", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	ts := f.Decls[0].(*ast.GenDecl).Specs[0].(*ast.TypeSpec)
	st := ts.Type.(*ast.StructType)

	si := structInfo{name: "Outer", fields: st.Fields.List}
	known := map[string]bool{"Inner": true}
	result := gen.generateResetMethod(si, known)

	if !strings.Contains(result, "rs.Inner.Reset()") {
		t.Errorf("expected Reset() call for known struct pointer, got:\n%s", result)
	}
}

func TestGenerateResetMethod_NestedStruct(t *testing.T) {
	gen := &Generator{fset: token.NewFileSet()}

	// Parse Inner first.
	srcInner := "package test\ntype Inner struct {\n\tValue string\n}"
	fInner, err := parser.ParseFile(gen.fset, "", srcInner, 0)
	if err != nil {
		t.Fatal(err)
	}
	tsInner := fInner.Decls[0].(*ast.GenDecl).Specs[0].(*ast.TypeSpec)
	stInner := tsInner.Type.(*ast.StructType)

	// Parse Outer.
	srcOuter := "package test\ntype Outer struct {\n\tInner Inner\n\tX int\n}"
	fOuter, err := parser.ParseFile(gen.fset, "", srcOuter, 0)
	if err != nil {
		t.Fatal(err)
	}
	tsOuter := fOuter.Decls[0].(*ast.GenDecl).Specs[0].(*ast.TypeSpec)
	stOuter := tsOuter.Type.(*ast.StructType)

	known := map[string]bool{"Inner": true}

	// Test Inner.
	si1 := structInfo{name: "Inner", fields: stInner.Fields.List}
	r1 := gen.generateResetMethod(si1, known)
	if !strings.Contains(r1, `rs.Value = ""`) {
		t.Error("Inner.Reset: missing Value reset")
	}

	// Test Outer.
	si2 := structInfo{name: "Outer", fields: stOuter.Fields.List}
	r2 := gen.generateResetMethod(si2, known)
	if !strings.Contains(r2, "rs.Inner.Reset()") {
		t.Errorf("Outer.Reset: expected Reset() call on Inner field, got:\n%s", r2)
	}
}

func TestExprToString(t *testing.T) {
	gen := &Generator{fset: token.NewFileSet()}
	f, err := parser.ParseFile(gen.fset, "", "package p; type S struct { X []int }", 0)
	if err != nil {
		t.Fatal(err)
	}
	ts := f.Decls[0].(*ast.GenDecl).Specs[0].(*ast.TypeSpec)
	st := ts.Type.(*ast.StructType)
	arr := st.Fields.List[0].Type.(*ast.ArrayType)

	got := gen.exprToString(arr.Elt)
	if got != "int" {
		t.Errorf("exprToString = %q, want %q", got, "int")
	}
}

func TestIndent(t *testing.T) {
	gen := &Generator{}
	result := gen.indent("hello\nworld", "\t")
	want := "\thello\n\tworld"
	if result != want {
		t.Errorf("indent = %q, want %q", result, want)
	}
}

func TestGenerateFile_Basic(t *testing.T) {
	gen := &Generator{fset: token.NewFileSet()}
	src := "package test\ntype Foo struct {\n\tName string\n}"
	f, err := parser.ParseFile(gen.fset, "", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	ts := f.Decls[0].(*ast.GenDecl).Specs[0].(*ast.TypeSpec)
	st := ts.Type.(*ast.StructType)

	structs := []structInfo{{name: "Foo", fields: st.Fields.List}}
	result := gen.generateFile("test", structs, map[string]bool{})

	if !strings.Contains(result, "// Code generated by reset generator. DO NOT EDIT.") {
		t.Error("missing generated header")
	}
	if !strings.Contains(result, "package test") {
		t.Error("missing package declaration")
	}
	if !strings.Contains(result, "func (rs *Foo) Reset()") {
		t.Error("missing Reset method")
	}
}

func TestProcessDir(t *testing.T) {
	dir := t.TempDir()

	// Create a Go source file with an annotated struct.
	src := `package mypkg

// generate:reset
type MyStruct struct {
	Name  string
	Count int
}
`
	if err := os.WriteFile(filepath.Join(dir, "mystruct.go"), []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	gen := &Generator{fset: token.NewFileSet()}
	if err := gen.processDir(dir); err != nil {
		t.Fatal(err)
	}

	// Check that reset.gen.go was created.
	genPath := filepath.Join(dir, "reset.gen.go")
	if _, err := os.Stat(genPath); os.IsNotExist(err) {
		t.Fatal("reset.gen.go was not generated")
	}

	content, err := os.ReadFile(genPath)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(content), "func (rs *MyStruct) Reset()") {
		t.Error("generated file missing Reset method")
	}
	if !strings.Contains(string(content), `rs.Name = ""`) {
		t.Error("generated file missing Name reset")
	}
	if !strings.Contains(string(content), "rs.Count = 0") {
		t.Error("generated file missing Count reset")
	}
}

func TestProcessDir_NoAnnotation(t *testing.T) {
	dir := t.TempDir()

	src := `package mypkg
type NoReset struct {
	X int
}
`
	if err := os.WriteFile(filepath.Join(dir, "noreset.go"), []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	gen := &Generator{fset: token.NewFileSet()}
	if err := gen.processDir(dir); err != nil {
		t.Fatal(err)
	}

	// Should NOT create reset.gen.go (no annotated structs).
	genPath := filepath.Join(dir, "reset.gen.go")
	if _, err := os.Stat(genPath); !os.IsNotExist(err) {
		t.Error("reset.gen.go should not be created when no structs are annotated")
	}
}

func TestProcessDir_MultipleStructs(t *testing.T) {
	dir := t.TempDir()

	src := `package mypkg

// generate:reset
type A struct {
	Name string
}

// generate:reset
type B struct {
	Value float64
}
`
	if err := os.WriteFile(filepath.Join(dir, "types.go"), []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	gen := &Generator{fset: token.NewFileSet()}
	if err := gen.processDir(dir); err != nil {
		t.Fatal(err)
	}

	genPath := filepath.Join(dir, "reset.gen.go")
	content, err := os.ReadFile(genPath)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(content), "func (rs *A) Reset()") {
		t.Error("missing Reset for A")
	}
	if !strings.Contains(string(content), "func (rs *B) Reset()") {
		t.Error("missing Reset for B")
	}
}

func TestFindResetStructs(t *testing.T) {
	src := `package p

// generate:reset
type Annotated struct {
	X string
}

type Plain struct {
	Y int
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}

	structs, known := findResetStructs([]*ast.File{f})

	if len(structs) != 1 {
		t.Fatalf("expected 1 annotated struct, got %d", len(structs))
	}
	if structs[0].name != "Annotated" {
		t.Errorf("expected Annotated, got %s", structs[0].name)
	}
	if !known["Annotated"] {
		t.Error("Annotated should be in knownStructs")
	}
	if !known["Plain"] {
		t.Error("Plain should be in knownStructs")
	}
}

func TestProcessDir_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	gen := &Generator{fset: token.NewFileSet()}

	// Empty directory should not error.
	if err := gen.processDir(dir); err != nil {
		t.Errorf("processDir on empty dir should not error: %v", err)
	}
}

func TestProcessDir_SkipsTestAndGenFiles(t *testing.T) {
	dir := t.TempDir()

	// This file has an annotation but is _test.go — should be skipped.
	src := `package mypkg

// generate:reset
type TestStruct struct {
	X string
}
`
	if err := os.WriteFile(filepath.Join(dir, "mypkg_test.go"), []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	// This file has an annotation but is .gen.go — should be skipped.
	if err := os.WriteFile(filepath.Join(dir, "other.gen.go"), []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	gen := &Generator{fset: token.NewFileSet()}
	if err := gen.processDir(dir); err != nil {
		t.Fatal(err)
	}

	// Should NOT create reset.gen.go since the only annotated files were skipped.
	genPath := filepath.Join(dir, "reset.gen.go")
	if _, err := os.Stat(genPath); !os.IsNotExist(err) {
		t.Error("reset.gen.go should not be created when all annotated files are test/gen files")
	}
}
