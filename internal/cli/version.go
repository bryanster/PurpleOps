package cli

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/bryanster/blacklight/internal/version"
)

// newVersionCommand reports the build identity of this binary. It is the one
// command that touches nothing: no configuration, no database — so it also
// answers "is this thing installed and runnable at all?".
func newVersionCommand(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the build identity of this binary",
		Long: "Prints the version, the commit it was built from and when.\n\n" +
			"The server reports the same three fields at GET /api/v1/version, which is how\n" +
			"you check that the CLI you are holding matches the deployment you are pointing\n" +
			"it at.",
		Args: noArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			build := version.Get()
			return a.print(build, func(w *tabwriter.Writer) {
				fmt.Fprintf(w, "%s\n", build)
			})
		},
	}
}
