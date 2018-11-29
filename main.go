package main

import (
	"log"
	"os"

	"github.com/orisano/subflag"
)

func main() {
	log.SetPrefix("cg: ")
	log.SetFlags(0)

	err := subflag.SubCommand(os.Args[1:], []subflag.Command{
		&InterfacerCommand{},
		&ModelCommand{},
	})
	if err != nil {
		log.Fatal(err)
	}
}
