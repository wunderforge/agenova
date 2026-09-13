// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Command contractgen handles only Agenova's v0 JSON types and fixture storyboard.
// It is build tooling, not an API, policy engine, or general schema generator.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
)

type shape struct {
	Kind   string           `json:"kind"`
	Ref    string           `json:"ref,omitempty"`
	Values []string         `json:"values,omitempty"`
	Fields map[string]field `json:"fields,omitempty"`
	Item   *shape           `json:"item,omitempty"`
}
type field struct {
	Shape    shape `json:"shape"`
	Optional bool  `json:"optional,omitempty"`
}

// Reflection owns fields/tags; the Go AST owns enum constants. No copied enum list.
func enums(root string) (map[string][]string, error) {
	result := map[string][]string{}
	for _, file := range []string{"types.go", "sandbox_claim.go"} {
		f, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, "api/v1alpha1", file), nil, 0)
		if err != nil {
			return nil, err
		}
		ast.Inspect(f, func(n ast.Node) bool {
			v, ok := n.(*ast.ValueSpec)
			if !ok {
				return true
			}
			t, ok := v.Type.(*ast.Ident)
			if !ok || (t.Name != "ClaimPhase" && t.Name != "DecisionResult") {
				return true
			}
			for _, value := range v.Values {
				lit, ok := value.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					panic("unsupported v0 enum expression")
				}
				s, err := strconv.Unquote(lit.Value)
				if err != nil {
					panic(err)
				}
				result[t.Name] = append(result[t.Name], s)
			}
			return true
		})
	}
	return result, nil
}

func bindings(root string) ([]byte, error) {
	values, err := enums(root)
	if err != nil {
		return nil, err
	}
	defs := map[string]shape{}
	marshaler := reflect.TypeFor[json.Marshaler]()
	var visit func(reflect.Type) shape
	visit = func(t reflect.Type) shape {
		if t == reflect.TypeFor[v0.Duration]() {
			return shape{Kind: "string"}
		}
		if t.Kind() == reflect.Pointer {
			s := visit(t.Elem())
			return shape{Kind: "nullable", Item: &s}
		}
		if t.Implements(marshaler) || reflect.PointerTo(t).Implements(marshaler) {
			panic("unsupported custom JSON serializer: " + t.String())
		}
		if t.Name() != "" && t.PkgPath() != "" {
			if t.PkgPath() != reflect.TypeFor[v0.ClaimRequest]().PkgPath() {
				panic("non-v0 type: " + t.String())
			}
			name := t.Name()
			if _, ok := defs[name]; !ok {
				defs[name] = shape{}
				if t.Kind() == reflect.String {
					if len(values[name]) == 0 {
						panic("unhandled named string: " + name)
					}
					defs[name] = shape{Kind: "enum", Values: values[name]}
				} else if t.Kind() == reflect.Struct {
					fields := map[string]field{}
					for i := 0; i < t.NumField(); i++ {
						f := t.Field(i)
						tag := strings.Split(f.Tag.Get("json"), ",")
						if f.Anonymous || f.PkgPath != "" || tag[0] == "" || tag[0] == "-" || len(tag) > 2 || (len(tag) == 2 && tag[1] != "omitempty") {
							panic("unsupported v0 field: " + name + "." + f.Name)
						}
						fields[tag[0]] = field{Shape: visit(f.Type), Optional: len(tag) == 2}
					}
					defs[name] = shape{Kind: "object", Fields: fields}
				} else {
					panic("unsupported named v0 type: " + name)
				}
			}
			return shape{Kind: "ref", Ref: name}
		}
		switch t.Kind() {
		case reflect.String:
			return shape{Kind: "string"}
		case reflect.Slice:
			s := visit(t.Elem())
			a := shape{Kind: "array", Item: &s}
			return shape{Kind: "nullable", Item: &a}
		case reflect.Map:
			if t.Key().Kind() != reflect.String || t.Elem().Kind() != reflect.Interface {
				panic("unsupported map")
			}
			return shape{Kind: "json-map"}
		default:
			panic("unsupported v0 shape: " + t.String())
		}
	}
	for _, t := range []reflect.Type{reflect.TypeFor[v0.ClaimRequest](), reflect.TypeFor[v0.IssuedState]()} {
		visit(t)
	}
	var tsType func(shape) string
	tsType = func(s shape) string {
		switch s.Kind {
		case "string":
			return "string"
		case "ref":
			return s.Ref
		case "nullable":
			return "(" + tsType(*s.Item) + " | null)"
		case "array":
			return "Array<" + tsType(*s.Item) + ">"
		case "json-map":
			return "Record<string, JsonValue> | null"
		case "enum":
			q := []string{}
			for _, v := range s.Values {
				q = append(q, strconv.Quote(v))
			}
			return strings.Join(q, " | ")
		default:
			panic("unsupported TS shape")
		}
	}
	var out bytes.Buffer
	out.WriteString("// Copyright 2026 Dapeng Zhang and Agenova contributors.\n// SPDX-License-Identifier: Apache-2.0\n// Generated from api/v1alpha1 by ui/contractgen. Do not edit.\n\nexport type JsonValue = null | boolean | number | string | JsonValue[] | { [key: string]: JsonValue };\n")
	names := []string{}
	for name := range defs {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		s := defs[name]
		if s.Kind != "object" {
			fmt.Fprintf(&out, "export type %s = %s;\n", name, tsType(s))
			continue
		}
		fmt.Fprintf(&out, "export interface %s {\n", name)
		keys := []string{}
		for k := range s.Fields {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			f := s.Fields[k]
			optional := ""
			if f.Optional {
				optional = "?"
			}
			fmt.Fprintf(&out, "  %s%s: %s;\n", k, optional, tsType(f.Shape))
		}
		out.WriteString("}\n")
	}
	encoded, _ := json.MarshalIndent(defs, "", "  ")
	fmt.Fprintf(&out, "\nexport const shapes = %s as const;\n", encoded)
	return out.Bytes(), nil
}

func main() {
	if len(os.Args) != 3 && !(len(os.Args) == 4 && os.Args[1] == "check") {
		panic("usage: contractgen generate|check|fixtures REPO_ROOT [CHECK_CANDIDATE]")
	}
	root, err := filepath.Abs(os.Args[2])
	if err != nil {
		panic(err)
	}
	if os.Args[1] == "fixtures" {
		rows, err := fixtures(root)
		if err != nil {
			panic(err)
		}
		if err := json.NewEncoder(os.Stdout).Encode(rows); err != nil {
			panic(err)
		}
		return
	}
	generated, err := bindings(root)
	if err != nil {
		panic(err)
	}
	path := filepath.Join(root, "ui/src/contracts.generated.ts")
	if len(os.Args) == 4 {
		path = os.Args[3]
	}
	switch os.Args[1] {
	case "generate":
		if err := os.WriteFile(path, generated, 0644); err != nil {
			panic(err)
		}
	case "check":
		actual, err := os.ReadFile(path)
		if err != nil {
			panic(err)
		}
		if !bytes.Equal(bytes.ReplaceAll(actual, []byte("\r\n"), []byte("\n")), generated) {
			panic("contract drift: run npm --prefix ui run contracts:generate")
		}
	default:
		panic("unknown mode")
	}
}
