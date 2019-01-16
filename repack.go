package main

import (
	"flag"
	"fmt"
	"go/types"
	"strings"

	"github.com/agnivade/levenshtein"
	"github.com/orisano/subflag"
	"github.com/pkg/errors"
	"golang.org/x/tools/go/loader"
)

type RepackCommand struct {
	Source      string
	Destination string
}

func (c *RepackCommand) FlagSet() *flag.FlagSet {
	fs := flag.NewFlagSet("repack", flag.ExitOnError)
	fs.StringVar(&c.Source, "src", "", "source type (required)")
	fs.StringVar(&c.Destination, "dst", "", "destination type (required)")
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

	src := walkFields(nil, srcType, "x")
	dst := walkFields(nil, dstType, "y")

	for _, s := range src {
		if len(dst) == 0 {
			break
		}
		mini := levenshtein.ComputeDistance(s, dst[0])
		for i := 1; i < len(dst); i++ {
			v := levenshtein.ComputeDistance(s, dst[i])
			if mini > v {
				dst[0], dst[i] = dst[i], dst[0]
				mini = v
			}
		}
		fmt.Println(s, "=", dst[0])
		dst = dst[1:]
	}

	return nil
}

func walkFields(a []string, s *types.Struct, prefix string) []string {
	for i := 0; i < s.NumFields(); i++ {
		f := s.Field(i)
		name := prefix + "." + f.Name()
		t, ok := f.Type().Underlying().(*types.Struct)
		if ok {
			a = walkFields(a, t, name)
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
