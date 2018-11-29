package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/orisano/subflag"
	"github.com/pkg/errors"
)

type ModelCommand struct {
	Name string
}

func (c *ModelCommand) FlagSet() *flag.FlagSet {
	fs := flag.NewFlagSet("model", flag.ExitOnError)
	fs.StringVar(&c.Name, "n", "", "gen model name (required)")
	return fs
}

func (c *ModelCommand) Run(args []string) error {
	if c.Name == "" {
		return subflag.ErrInvalidArguments
	}

	if err := renderTemplate("repository", c.Name); err != nil {
		return errors.Wrap(err, "failed to render repository")
	}
	if err := renderTemplate("service", c.Name); err != nil {
		return errors.Wrap(err, "failed to render service")
	}
	return nil
}

func renderTemplate(pkg, name string) error {
	f, err := os.Create(filepath.Join(pkg, name+".go"))
	if err != nil {
		return errors.Wrap(err, "failed to create")
	}
	_, err = fmt.Fprintf(f, `package %s

//go:generate cg interfacer
type %s%s struct{}
`, pkg, name, strings.Title(pkg))
	if err != nil {
		return errors.Wrap(err, "failed to render template")
	}

	return nil
}
