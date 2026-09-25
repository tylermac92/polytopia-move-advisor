// Command advisor loads the rules file and starts the advisor server.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/tylermac92/polytopia-move-advisor/internal/rules"
)

func main() {
	rulesPath := flag.String("rules", "rules/v1.yaml", "path to the pinned rules file")
	flag.Parse()

	// The rules are validated before anything else starts, so a bad rules
	// file stops the advisor with every problem listed instead of surfacing
	// as wrong advice later.
	r, err := rules.Load(*rulesPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "advisor: invalid rules file %s\n", err)
		os.Exit(1)
	}
	fmt.Printf("advisor: loaded rules %s for %s (%d units, %d techs)\n",
		r.Version, r.GameBuild, len(r.Units), len(r.Techs))

	// The server is not implemented yet.
}
