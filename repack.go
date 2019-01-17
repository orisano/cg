package main

import (
	"flag"
	"fmt"
	"go/types"
	"sort"
	"strings"

	"github.com/orisano/subflag"
	"github.com/pkg/errors"
	"golang.org/x/tools/go/loader"
)

type RepackCommand struct {
	Source      string
	Destination string
	Ignores string
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
		return subflag.ErrInvalidArguments
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

	src := walkFields(nil, srcType, "y")
	dst := walkFields(nil, dstType, "x")

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

	for _, d := range dst {
		if len(src) == 0 {
			break
		}
		mini := computeEditDistance(d, src[0])
		for i := 1; i < len(src); i++ {
			v := computeEditDistance(d, src[i])
			if mini > v {
				src[0], src[i] = src[i], src[0]
				mini = v
			}
		}
		fmt.Println(d, "=", src[0])
		src = src[1:]
	}

	return nil
}

func walkFields(a []string, s *types.Struct, prefix string) []string {
	for i := 0; i < s.NumFields(); i++ {
		f := s.Field(i)
		name := prefix + "." + f.Name()

		t := f.Type()
		ptr, ok := t.(*types.Pointer)
		if ok {
			t = ptr.Elem()
		}
		named, ok := t.(*types.Named)
		if ok && strings.IndexByte(strings.Split(named.Obj().Pkg().Path(), "/")[0], '.') >= 0 {
			t = t.Underlying()
		}
		st, ok := t.(*types.Struct)
		if ok {
			a = walkFields(a, st, name)
		} else {
			a = append(a, name)
		}
	}
	return a
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
			replaceCost := 3
			if x[i-1] == y[j-1] {
				replaceCost = 0
			}
			d[i][j] = mini(d[i][j-1]+1, mini(d[i-1][j]+1, d[i-1][j-1]+replaceCost))
		}
	}
	return d[N][M]
}

func mini(a, b int) int {
	if a < b {
		return a
	} else {
		return b
	}
}

func filterStrings(s []string, cond func(string)bool) []string {
	x := s[:0]
	for _, a := range s {
		if cond(a) {
			x = append(x, a)
		}
	}
	return x
}