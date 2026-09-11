package reconcilers

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// printfVerb matches a printf-style verb (%s, %d, %v, etc.).
var printfVerb = regexp.MustCompile(`%[a-zA-Z]`)

// TestNoPrintfStyleLogrCalls guards against the class of bug in issue #1525:
// a printf-style format string passed straight to a logr.Logger Info/Error
// call. logr treats trailing args as key/value pairs, so the format verbs
// are never substituted and, if the remaining args form an odd count, zapr
// emits a DPanic that becomes a real panic under dev-mode logging (used by
// this repo's own test suites).
//
// It statically scans every non-test .go file in the module for Info/Error
// calls whose message argument is a raw string literal containing a printf
// verb. A message built via fmt.Sprintf(...) is a *ast.CallExpr rather than
// a *ast.BasicLit, so it is not flagged.
func TestNoPrintfStyleLogrCalls(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not determine test file location")
	}
	moduleRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")

	var violations []string
	fset := token.NewFileSet()

	err := filepath.Walk(moduleRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}

		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}

			var msgArg ast.Expr
			switch sel.Sel.Name {
			case "Info":
				if len(call.Args) >= 1 {
					msgArg = call.Args[0]
				}
			case "Error":
				if len(call.Args) >= 2 {
					msgArg = call.Args[1]
				}
			default:
				return true
			}

			lit, ok := msgArg.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			if printfVerb.MatchString(lit.Value) {
				pos := fset.Position(lit.Pos())
				violations = append(violations, fmt.Sprintf("%s:%d: printf-style verb in logr message %s", pos.Filename, pos.Line, lit.Value))
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("failed to scan for printf-style logr calls: %v", err)
	}

	if len(violations) > 0 {
		t.Fatalf("found printf-style format strings passed directly to logr Info/Error calls (use structured key/value args instead):\n%s", strings.Join(violations, "\n"))
	}
}
