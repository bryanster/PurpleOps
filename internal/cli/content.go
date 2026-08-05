package cli

import (
	"context"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/bryanster/blacklight/internal/authn"
	"github.com/bryanster/blacklight/internal/content"
	"github.com/bryanster/blacklight/internal/events"
	"github.com/bryanster/blacklight/internal/store"
	"github.com/bryanster/blacklight/internal/store/activity"
	storecontent "github.com/bryanster/blacklight/internal/store/content"
)

// blctl content sources | enable | disable | sync
//
// sources/enable/disable are M2-002. sync stays a stub until M2-003's job
// runner lands. The host is the access control: these open the database file
// directly, the same way `user create` does.

func newContentCommand(a *app) *cobra.Command {
	return group("content", "Manage content sources (M2)",
		newContentSourcesCommand(a),
		newContentEnableCommand(a),
		newContentDisableCommand(a),
		newContentSyncCommand(),
	)
}

func newContentSyncCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "Install or refresh a content source (M2)",
		Long: "Installs or refreshes a content source — ATT&CK, Atomic Red Team, Sigma, CTID —\n" +
			"from the command line, for a deployment that cannot reach them from the\n" +
			"browser, or for seeding one before anybody signs in.",
		Args: noArgs,
		RunE: notImplemented("M2-003", "which builds the adapter interface and job runner"),
	}
}

func newContentSourcesCommand(a *app) *cobra.Command {
	var (
		showID  string
		kind    string
		enabled string
	)

	cmd := &cobra.Command{
		Use:   "sources",
		Short: "List content sources, or show one",
		Long: "Lists every content source on this installation (kind, enabled, status,\n" +
			"item count), or shows one source in full when --id is given.\n\n" +
			"Filter with --kind and/or --enabled=true|false. Disabled sources stay in\n" +
			"the list — an operator needs to see them to turn them back on.",
		Args: noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withStore(cmd.Context(), func(ctx context.Context, db *store.DB) error {
				if err := a.requireMigrated(ctx, db); err != nil {
					return err
				}
				reg, err := contentRegistry(db)
				if err != nil {
					return err
				}

				if showID != "" {
					detail, err := reg.GetSource(ctx, strings.TrimSpace(showID))
					if err != nil {
						return err
					}
					return a.printContentSourceDetail(detail)
				}

				filter := content.SourceFilter{Kind: strings.TrimSpace(kind)}
				if enabled != "" {
					switch strings.ToLower(enabled) {
					case "true", "1", "yes":
						v := true
						filter.Enabled = &v
					case "false", "0", "no":
						v := false
						filter.Enabled = &v
					default:
						return usagef(cmd, "--enabled must be true or false")
					}
				}

				items, err := reg.ListSources(ctx, filter)
				if err != nil {
					return err
				}
				return a.printContentSources(items)
			})
		},
	}

	cmd.Flags().StringVar(&showID, "id", "", "show one source by identifier")
	cmd.Flags().StringVar(&kind, "kind", "", "filter by kind (attack|atomic|sigma|ctid|custom)")
	cmd.Flags().StringVar(&enabled, "enabled", "", "filter by enabled state (true|false)")
	return cmd
}

func newContentEnableCommand(a *app) *cobra.Command {
	return newContentEnablementCommand(a, "enable", true)
}

func newContentDisableCommand(a *app) *cobra.Command {
	return newContentEnablementCommand(a, "disable", false)
}

func newContentEnablementCommand(a *app, use string, enable bool) *cobra.Command {
	var id string
	verb := "Enables"
	if !enable {
		verb = "Disables"
	}

	cmd := &cobra.Command{
		Use:   use,
		Short: strings.TrimSuffix(verb, "s") + " a content source",
		Long: verb + " a content source by identifier. Idempotent.\n\n" +
			"Disabling keeps rows on disk but blocks new references; enabling makes\n" +
			"them eligible again. Prefer disable over delete for builtin upstream seeds.",
		Args: noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			id = strings.TrimSpace(id)
			if id == "" {
				return usagef(cmd, "--id is required")
			}
			return a.withStore(cmd.Context(), func(ctx context.Context, db *store.DB) error {
				if err := a.requireMigrated(ctx, db); err != nil {
					return err
				}
				reg, err := contentRegistry(db)
				if err != nil {
					return err
				}
				// blctl acts as the host operator: no HTTP session, so the
				// activity actor is empty. The row still records the verb.
				actor := authn.Subject{}
				var src storecontent.Source
				if enable {
					src, err = reg.EnableSource(ctx, actor, id)
				} else {
					src, err = reg.DisableSource(ctx, actor, id)
				}
				if err != nil {
					return err
				}
				return a.printContentSource(src)
			})
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "source identifier")
	return cmd
}

func contentRegistry(db *store.DB) (*content.Registry, error) {
	// Paths are unused by the registry surface in M2-002 (no raw writes), but
	// Versions requires one. An empty root is fine: SetRaw is not called here.
	return content.New(content.Deps{
		Sources:  storecontent.NewSources(db),
		Versions: storecontent.NewVersions(db, storecontent.Paths{}),
		Jobs:     storecontent.NewJobs(db),
		Activity: events.New(activity.New(db)),
	})
}

type contentSourceResult struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Enabled   bool   `json:"enabled"`
	Status    string `json:"status"`
	ItemCount int64  `json:"itemCount"`
	URL       string `json:"url,omitempty"`
	Ref       string `json:"ref,omitempty"`
	Error     string `json:"error,omitempty"`
}

func toContentSourceResult(s storecontent.Source) contentSourceResult {
	return contentSourceResult{
		ID:        s.ID,
		Kind:      string(s.Kind),
		Name:      s.Name,
		Enabled:   s.Enabled,
		Status:    string(s.Status),
		ItemCount: s.ItemCount,
		URL:       s.URL,
		Ref:       s.Ref,
		Error:     s.Error,
	}
}

func (a *app) printContentSources(items []storecontent.Source) error {
	out := make([]contentSourceResult, 0, len(items))
	for _, s := range items {
		out = append(out, toContentSourceResult(s))
	}
	return a.print(out, func(w *tabwriter.Writer) {
		fmt.Fprintln(w, "ID\tKIND\tNAME\tENABLED\tSTATUS\tITEMS")
		for _, s := range out {
			fmt.Fprintf(w, "%s\t%s\t%s\t%t\t%s\t%d\n",
				s.ID, s.Kind, s.Name, s.Enabled, s.Status, s.ItemCount)
		}
	})
}

func (a *app) printContentSource(s storecontent.Source) error {
	out := toContentSourceResult(s)
	return a.print(out, func(w *tabwriter.Writer) {
		fmt.Fprintf(w, "ID\t%s\n", out.ID)
		fmt.Fprintf(w, "Kind\t%s\n", out.Kind)
		fmt.Fprintf(w, "Name\t%s\n", out.Name)
		fmt.Fprintf(w, "Enabled\t%t\n", out.Enabled)
		fmt.Fprintf(w, "Status\t%s\n", out.Status)
		fmt.Fprintf(w, "Items\t%d\n", out.ItemCount)
		if out.URL != "" {
			fmt.Fprintf(w, "URL\t%s\n", out.URL)
		}
		if out.Ref != "" {
			fmt.Fprintf(w, "Ref\t%s\n", out.Ref)
		}
		if out.Error != "" {
			fmt.Fprintf(w, "Error\t%s\n", out.Error)
		}
	})
}

func (a *app) printContentSourceDetail(d content.SourceDetail) error {
	type jobSummary struct {
		ID     string `json:"id"`
		Kind   string `json:"kind"`
		Status string `json:"status"`
	}
	type detail struct {
		contentSourceResult
		LastJob *jobSummary `json:"lastJob,omitempty"`
	}
	out := detail{contentSourceResult: toContentSourceResult(d.Source)}
	if d.LastJob != nil {
		out.LastJob = &jobSummary{
			ID:     d.LastJob.ID,
			Kind:   string(d.LastJob.Kind),
			Status: string(d.LastJob.Status),
		}
	}
	return a.print(out, func(w *tabwriter.Writer) {
		fmt.Fprintf(w, "ID\t%s\n", out.ID)
		fmt.Fprintf(w, "Kind\t%s\n", out.Kind)
		fmt.Fprintf(w, "Name\t%s\n", out.Name)
		fmt.Fprintf(w, "Enabled\t%t\n", out.Enabled)
		fmt.Fprintf(w, "Status\t%s\n", out.Status)
		fmt.Fprintf(w, "Items\t%d\n", out.ItemCount)
		if out.URL != "" {
			fmt.Fprintf(w, "URL\t%s\n", out.URL)
		}
		if out.Ref != "" {
			fmt.Fprintf(w, "Ref\t%s\n", out.Ref)
		}
		if out.Error != "" {
			fmt.Fprintf(w, "Error\t%s\n", out.Error)
		}
		if out.LastJob != nil {
			fmt.Fprintf(w, "Last job\t%s (%s, %s)\n",
				out.LastJob.ID, out.LastJob.Kind, out.LastJob.Status)
		}
	})
}
