package data

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// categoryFiles maps each ItemData source file (basename without .go) to the
// exact Category value every record in it must carry. The basename IS the
// expected Category. Keep this list in sync with the var X = map[uint32]ItemData
// declarations in this package.
var categoryFiles = []string{
	"arms",
	"arrows_and_bolts",
	"ashes",
	"ashes_of_war",
	"bolstering_materials",
	"chest",
	"crafting_materials",
	"gestures",
	"head",
	"incantations",
	"info",
	"key_items",
	"legs",
	"melee_armaments",
	"ranged_and_catalysts",
	"shields",
	"sorceries",
	"talismans",
	"tools",
}

// isItemDataMap reports whether typeExpr is the type map[uint32]ItemData.
func isItemDataMap(typeExpr ast.Expr) bool {
	mt, ok := typeExpr.(*ast.MapType)
	if !ok {
		return false
	}
	key, ok := mt.Key.(*ast.Ident)
	if !ok || key.Name != "uint32" {
		return false
	}
	val, ok := mt.Value.(*ast.Ident)
	return ok && val.Name == "ItemData"
}

// itemDataCategory extracts the string value of the Category field from an
// ItemData composite literal. found is false when the field is absent; ok is
// false when it is present but not a plain string literal.
func itemDataCategory(lit *ast.CompositeLit) (value string, found, ok bool) {
	for _, elt := range lit.Elts {
		kv, isKV := elt.(*ast.KeyValueExpr)
		if !isKV {
			continue
		}
		ident, isIdent := kv.Key.(*ast.Ident)
		if !isIdent || ident.Name != "Category" {
			continue
		}
		bl, isBasic := kv.Value.(*ast.BasicLit)
		if !isBasic || bl.Kind != token.STRING {
			return "", true, false
		}
		s, err := strconv.Unquote(bl.Value)
		if err != nil {
			return "", true, false
		}
		return s, true, true
	}
	return "", false, false
}

// TestItemCategoryPlacement guards the physical layout of the ItemData database:
// every ItemData record declared in <category>.go must carry
// Category: "<category>". It parses only the canonical category source files via
// the Go AST — never test or helper files — so no record can drift into the
// wrong file. It is deterministic and depends on nothing outside this package's
// source directory (no Git, tmp, or environment state).
func TestItemCategoryPlacement(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve test source directory")
	}
	dir := filepath.Dir(thisFile)

	fset := token.NewFileSet()
	for _, base := range categoryFiles {
		path := filepath.Join(dir, base+".go")
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}

		sawMap := false
		ast.Inspect(file, func(n ast.Node) bool {
			lit, isLit := n.(*ast.CompositeLit)
			if !isLit || !isItemDataMap(lit.Type) {
				return true
			}
			sawMap = true
			for _, elt := range lit.Elts {
				kv, isKV := elt.(*ast.KeyValueExpr)
				if !isKV {
					continue
				}
				item, isItem := kv.Value.(*ast.CompositeLit)
				if !isItem {
					continue
				}
				pos := fset.Position(item.Pos())
				cat, found, valOK := itemDataCategory(item)
				key := exprText(kv.Key)
				switch {
				case !found:
					t.Errorf("%s:%d: ItemData %s has no Category field (expected %q)",
						base+".go", pos.Line, key, base)
				case !valOK:
					t.Errorf("%s:%d: ItemData %s Category is not a string literal (expected %q)",
						base+".go", pos.Line, key, base)
				case cat != base:
					t.Errorf("%s:%d: ItemData %s declared in wrong file: Category=%q but file requires %q",
						base+".go", pos.Line, key, cat, base)
				}
			}
			return false // element ItemData literals already handled above
		})
		if !sawMap {
			t.Errorf("%s.go: no map[uint32]ItemData literal found — category file list is stale", base)
		}
	}
}

// exprText renders a map key (a hex ID) for diagnostics.
func exprText(e ast.Expr) string {
	if bl, ok := e.(*ast.BasicLit); ok {
		return bl.Value
	}
	if id, ok := e.(*ast.Ident); ok {
		return id.Name
	}
	return strings.TrimSpace("<expr>")
}
