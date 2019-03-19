package main

import (
	"flag"
	"fmt"
	"go/types"
	"sort"
	"strings"

	"github.com/pkg/errors"
	"golang.org/x/tools/go/loader"
)

type RepackCommand struct {
	Source      string
	Destination string
	Ignores     string
}

func (c *RepackCommand) FlagSet() *flag.FlagSet {
	fs := flag.NewFlagSet("repack", flag.ExitOnError)
	fs.StringVar(&c.Source, "src", "", "source type (required)")
	fs.StringVar(&c.Destination, "dst", "", "destination type (required)")
	fs.StringVar(&c.Ignores, "i", "", "ignore fields pattern (comma separated)")
	return fs
}

func (c *RepackCommand) Run(args []string) error {
	if c.Source == "" || c.Destination == "" {
		return flag.ErrHelp
	}
	srcImportPath, srcTypeName, err := parseType(c.Source)
	if err != nil {
		return errors.Wrap(err, "failed to parse source type")
	}
	dstImportPath, dstTypeName, err := parseType(c.Destination)
	if err != nil {
		return errors.Wrap(err, "failed to parse destination type")
	}

	var conf loader.Config
	conf.Import(srcImportPath)
	conf.Import(dstImportPath)
	prog, err := conf.Load()
	if err != nil {
		return errors.Wrap(err, "failed to load program")
	}
	srcType := prog.Package(srcImportPath).Pkg.Scope().Lookup(srcTypeName).Type().Underlying().(*types.Struct)
	dstType := prog.Package(dstImportPath).Pkg.Scope().Lookup(dstTypeName).Type().Underlying().(*types.Struct)

	src := enumerateFields(srcType, "y")
	dst := enumerateFields(dstType, "x")

	ignores := strings.Split(c.Ignores, ",")
	cond := func(s string) bool {
		fields := strings.Split(s, ".")
		for _, field := range fields {
			for _, x := range ignores {
				if field == x {
					return false
				}
			}
		}
		return true
	}

	src = filterStrings(src, cond)
	dst = filterStrings(dst, cond)

	sort.SliceStable(dst, func(i, j int) bool {
		return len(dst[j]) < len(dst[i])
	})

	n := maxInt(len(src), len(dst))
	a := make([][]int, n)
	for i := 0; i < n; i++ {
		a[i] = make([]int, n)
		for j := 0; j < n; j++ {
			if i >= len(dst) || j >= len(src) {
				a[i][j] = 0
			} else {
				a[i][j] = -computeEditDistance(dst[i], src[j]) + -computeEditDistance(suffix(dst[i], 5), suffix(src[j], 5))
			}
		}
	}

	x := hungarian(a)
	for i, d := range dst {
		if 0 <= x[i] && x[i] < len(src) {
			fmt.Println(d, "=", src[x[i]])
		}
	}
	return nil
}

func suffix(s string, n int) string {
	if len(s) > n {
		return s[len(s)-n:]
	}
	return s
}

func isNoStdPackage(p *types.Package) bool {
	importPath := p.Path()
	host := strings.Split(importPath, "/")[0]
	return strings.Index(host, ".") >= 0
}

func toPath(prefix string, stack []*types.Var) string {
	s := []string{prefix}
	for _, f := range stack {
		s = append(s, f.Name())
	}
	return strings.Join(s, ".")
}

func enumerateFields(s *types.Struct, prefix string) []string {
	var fields []string
	WalkFields(s, func(s *types.Struct, stack []*types.Var) {
		fields = append(fields, toPath(prefix, stack))
	})
	return fields
}

func WalkFields(root *types.Struct, fn func(s *types.Struct, stack []*types.Var)) {
	walkFields(root, fn, nil)
}

func walkFields(s *types.Struct, fn func(*types.Struct, []*types.Var), stack []*types.Var) {
	for i := 0; i < s.NumFields(); i++ {
		f := s.Field(i)
		t := f.Type()
		ptr, ok := t.(*types.Pointer)
		if ok {
			t = ptr.Elem()
		}
		named, ok := t.(*types.Named)
		if ok && isNoStdPackage(named.Obj().Pkg()) {
			t = t.Underlying()
		}
		st, ok := t.(*types.Struct)
		if ok {
			walkFields(st, fn, append(stack, f))
		} else {
			fn(s, append(stack, f))
		}
	}
}

func parseType(s string) (importPath, typeName string, err error) {
	tokens := strings.Split(s, "#")
	if len(tokens) != 2 {
		return "", "", errors.Errorf("invalid format %q", s)
	}
	return tokens[0], tokens[1], nil
}

func computeEditDistance(a, b string) int {
	x := []rune(a)
	y := []rune(b)

	N := len(x)
	M := len(y)

	d := make([][]int, N+1)
	for i := range d {
		d[i] = make([]int, M+1)
	}
	d[0][0] = 0
	for i := 1; i <= N; i++ {
		d[i][0] = i
	}
	for i := 1; i <= M; i++ {
		d[0][i] = i
	}

	for i := 1; i <= N; i++ {
		for j := 1; j <= M; j++ {
			replaceCost := 2
			if x[i-1] == y[j-1] {
				replaceCost = 0
			}
			d[i][j] = minInt(d[i][j-1]+1, minInt(d[i-1][j]+1, d[i-1][j-1]+replaceCost))
		}
	}
	return d[N][M]
}

func minInt(a, b int) int {
	if a < b {
		return a
	} else {
		return b
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	} else {
		return b
	}
}

func filterStrings(s []string, cond func(string) bool) []string {
	x := s[:0]
	for _, a := range s {
		if cond(a) {
			x = append(x, a)
		}
	}
	return x
}

func hungarian(a [][]int) []int {
	const inf = 1<<29
	n := len(a)
	fx := fillInt(make([]int, n), inf)
	fy := make([]int, n)
	x := fillInt(make([]int, n), -1)
	y := fillInt(make([]int, n), -1)

	p := 0
	q := 0
	for i := range a {
		for j := range a[i] {
			fx[i] = maxInt(fx[i], a[i][j])
		}
	}
	t := make([]int, n)
	s := make([]int, n+1)
	for i := 0; i < n; {
		t = fillInt(t, -1)
		s = fillInt(s, i)
		for p, q = 0, 0; p <= q && x[i] < 0; p++ {
			for k, j := s[p], 0; j < n && x[i] < 0; j++ {
				if fx[k]+fy[j] == a[k][j] && t[j] < 0 {
					q++
					s[q] = y[j]
					t[j] = k
					if s[q] < 0 {
						for p = j; p >= 0; j = p {
							y[j], k = t[j], t[j]
							p = x[k]
							x[k] = j
						}
					}
				}
			}
		}
		if x[i] < 0 {
			d := inf
			for k := 0; k <= q; k++ {
				for j := 0; j < n; j++ {
					if t[j] < 0 {
						d = minInt(d, fx[s[k]]+fy[j]-a[s[k]][j])
					}
				}
			}
			for j := 0; j < n; j++ {
				if t[j] >= 0 {
					fy[j] += d
				}
			}
			for k := 0; k <= q; k++ {
				fx[s[k]] -= d
			}
		} else {
			i++
		}
	}
	return x
}

func fillInt(a []int, v int) []int {
	for i := range a {
		a[i] = v
	}
	return a
}
