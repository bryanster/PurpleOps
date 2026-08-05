package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/bryanster/blacklight/internal/authn"
	"github.com/bryanster/blacklight/internal/content"
	"github.com/bryanster/blacklight/internal/content/atomic"
	"github.com/bryanster/blacklight/internal/content/attack"
	"github.com/bryanster/blacklight/internal/content/ctid"
	"github.com/bryanster/blacklight/internal/content/sigma"
	"github.com/bryanster/blacklight/internal/events"
	"github.com/bryanster/blacklight/internal/store"
	"github.com/bryanster/blacklight/internal/store/activity"
	storecontent "github.com/bryanster/blacklight/internal/store/content"
)

// blctl content sources | enable | disable | sync | import-bundle | reprocess
//
// The host is the access control: these open the database file directly, the
// same way `user create` does.

func newContentCommand(a *app) *cobra.Command {
	return group("content", "Manage content sources (M2)",
		newContentSourcesCommand(a),
		newContentEnableCommand(a),
		newContentDisableCommand(a),
		newContentSyncCommand(a),
		newContentImportBundleCommand(a),
		newContentReprocessCommand(a),
		newContentExportCustomCommand(a),
		newContentImportCommand(a),
	)
}

func newContentSyncCommand(a *app) *cobra.Command {
	var (
		source  string
		version string
		wait    bool
	)
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Install or refresh a content source",
		Long: "Enqueues a content sync job for a source identified by id or kind.\n" +
			"At most one content job runs installation-wide; a second start fails.\n" +
			"Pass --wait to block until the job reaches a terminal status.",
		Args: noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if source == "" {
				return fmt.Errorf("--source is required (source id or kind)")
			}
			return a.withContentRunner(cmd.Context(), nil, func(ctx context.Context, runner *content.Runner, src storecontent.Source) error {
				job, err := runner.StartSync(ctx, authn.Subject{}, content.StartSyncRequest{
					SourceID: src.ID,
					Version:  version,
				})
				if err != nil {
					return err
				}
				return a.finishContentJob(ctx, runner, job, wait)
			}, source)
		},
	}
	cmd.Flags().StringVar(&source, "source", "", "source id or kind (attack|atomic|sigma|ctid)")
	cmd.Flags().StringVar(&version, "version", "", "version pin (ATT&CK release label); omit for latest")
	cmd.Flags().BoolVar(&wait, "wait", false, "block until the job finishes")
	return cmd
}

func newContentImportBundleCommand(a *app) *cobra.Command {
	var (
		source  string
		file    string
		version string
		wait    bool
	)
	cmd := &cobra.Command{
		Use:   "import-bundle",
		Short: "Install a content source from an offline bundle file",
		Long: "Uploads a release archive from disk and enqueues a bundle_import job.\n" +
			"The archive shape must match what the adapter's HTTPS fetch would have\n" +
			"produced — see docs/content-bundles.md. No network I/O is performed.\n" +
			"Pass --wait to block until the job reaches a terminal status.",
		Args: noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if source == "" {
				return fmt.Errorf("--source is required (source id or kind)")
			}
			if file == "" {
				return fmt.Errorf("--file is required")
			}
			return a.withContentRunner(cmd.Context(), nil, func(ctx context.Context, runner *content.Runner, src storecontent.Source) error {
				f, err := os.Open(file)
				if err != nil {
					return fmt.Errorf("open bundle: %w", err)
				}
				defer f.Close()
				path, sha, _, err := runner.SpoolUpload(ctx, f, file)
				if err != nil {
					return err
				}
				job, err := runner.StartBundleImport(ctx, authn.Subject{}, content.StartBundleImportRequest{
					SourceID:     src.ID,
					Version:      version,
					BundlePath:   path,
					BundleSHA256: sha,
				})
				if err != nil {
					return err
				}
				return a.finishContentJob(ctx, runner, job, wait)
			}, source)
		},
	}
	cmd.Flags().StringVar(&source, "source", "", "source id or kind (attack|atomic|sigma|ctid)")
	cmd.Flags().StringVar(&file, "file", "", "path to the release archive (zip/tar.gz)")
	cmd.Flags().StringVar(&version, "version", "", "version pin (ATT&CK release label); omit for latest/embedded")
	cmd.Flags().BoolVar(&wait, "wait", false, "block until the job finishes")
	return cmd
}

func newContentReprocessCommand(a *app) *cobra.Command {
	var (
		source  string
		version string
		wait    bool
	)
	cmd := &cobra.Command{
		Use:   "reprocess",
		Short: "Re-parse a source from its last raw snapshot",
		Long: "Enqueues a reprocess job that opens the last successful raw snapshot\n" +
			"for the source/version and runs Parse → Normalize → Apply with no network.\n" +
			"Fails if no raw snapshot exists. ATT&CK requires --version.\n" +
			"Pass --wait to block until the job reaches a terminal status.",
		Args: noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if source == "" {
				return fmt.Errorf("--source is required (source id or kind)")
			}
			return a.withContentRunner(cmd.Context(), nil, func(ctx context.Context, runner *content.Runner, src storecontent.Source) error {
				job, err := runner.StartReprocess(ctx, authn.Subject{}, content.StartReprocessRequest{
					SourceID: src.ID,
					Version:  version,
				})
				if err != nil {
					return err
				}
				return a.finishContentJob(ctx, runner, job, wait)
			}, source)
		},
	}
	cmd.Flags().StringVar(&source, "source", "", "source id or kind (attack|atomic|sigma|ctid)")
	cmd.Flags().StringVar(&version, "version", "", "version pin (required for ATT&CK; default current for rolling sources)")
	cmd.Flags().BoolVar(&wait, "wait", false, "block until the job finishes")
	return cmd
}

// withContentRunner opens the store, builds a content Runner with the process
// config, Boots + Starts it, resolves source, runs fn, then Stops.
//
// adapters may be nil (production blctl has no upstream adapters until the
// process that embeds them registers them — same as the server). Tests that
// need a fixture inject via a non-nil map.
func (a *app) withContentRunner(
	ctx context.Context,
	adapters map[storecontent.Kind]content.Adapter,
	fn func(context.Context, *content.Runner, storecontent.Source) error,
	source string,
) error {
	return a.withStore(ctx, func(ctx context.Context, db *store.DB) error {
		cfg, err := a.settings()
		if err != nil {
			return err
		}
		paths := storecontent.NewPaths(cfg.Content.Dir)
		sources := storecontent.NewSources(db)
		versions := storecontent.NewVersions(db, paths)
		jobs := storecontent.NewJobs(db)

		src, err := resolveContentSource(ctx, sources, source)
		if err != nil {
			return err
		}

		if adapters == nil {
			adapters = map[storecontent.Kind]content.Adapter{}
		}
		if _, ok := adapters[storecontent.KindAttack]; !ok {
			adapters[storecontent.KindAttack] = attack.New()
		}
		if _, ok := adapters[storecontent.KindAtomic]; !ok {
			adapters[storecontent.KindAtomic] = atomic.New()
		}
		if _, ok := adapters[storecontent.KindSigma]; !ok {
			adapters[storecontent.KindSigma] = sigma.New()
		}
		if _, ok := adapters[storecontent.KindCTID]; !ok {
			adapters[storecontent.KindCTID] = ctid.New()
		}
		customSvc, err := contentCustom(db)
		if err != nil {
			return err
		}
		runner, err := content.NewRunner(content.RunnerDeps{
			DB:         db,
			Sources:    sources,
			Versions:   versions,
			Jobs:       jobs,
			Paths:      paths,
			Activity:   events.New(activity.New(db)),
			Custom:     customSvc,
			Adapters:   adapters,
			MaxBytes:   cfg.Content.MaxBytes.Int64(),
			JobTimeout: cfg.Content.JobTimeout,
			WriteBatch: cfg.Content.WriteBatch,
			Log:        a.logger(cfg),
		})
		if err != nil {
			return err
		}
		// Boot reconciles leftover running rows from a previous process.
		if err := runner.Boot(ctx); err != nil {
			return err
		}
		runner.Start(ctx)
		defer runner.Stop()
		return fn(ctx, runner, src)
	})
}

func (a *app) finishContentJob(ctx context.Context, runner *content.Runner, job storecontent.Job, wait bool) error {
	if !wait {
		return a.printContentJob(job)
	}
	// Bound wait by job timeout + slack so a hung adapter cannot pin the CLI
	// forever when --wait is set without a parent deadline.
	wctx := ctx
	cancel := func() {}
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		cfg, err := a.settings()
		if err != nil {
			return err
		}
		timeout := cfg.Content.JobTimeout
		if timeout <= 0 {
			timeout = 30 * time.Minute
		}
		wctx, cancel = context.WithTimeout(ctx, timeout+time.Minute)
	}
	defer cancel()
	job, err := runner.Wait(wctx, job.ID)
	if err != nil {
		return err
	}
	if err := a.printContentJob(job); err != nil {
		return err
	}
	switch job.Status {
	case storecontent.JobStatusSucceeded:
		return nil
	case storecontent.JobStatusCancelled:
		return fmt.Errorf("job %s cancelled", job.ID)
	default:
		if job.Error != "" {
			return fmt.Errorf("job %s %s: %s", job.ID, job.Status, job.Error)
		}
		return fmt.Errorf("job %s ended %s", job.ID, job.Status)
	}
}

func resolveContentSource(ctx context.Context, sources *storecontent.Sources, idOrKind string) (storecontent.Source, error) {
	// Prefer id lookup when it looks like a UUID; otherwise treat as kind.
	if strings.Contains(idOrKind, "-") && len(idOrKind) >= 32 {
		return sources.ByID(ctx, idOrKind)
	}
	switch strings.ToLower(idOrKind) {
	case "attack", "atomic", "sigma", "ctid", "custom":
		return sources.ByKind(ctx, storecontent.Kind(strings.ToLower(idOrKind)))
	default:
		// Fall back to id — user may have passed a compact id form.
		if src, err := sources.ByID(ctx, idOrKind); err == nil {
			return src, nil
		}
		return storecontent.Source{}, fmt.Errorf("unknown source %q (want id or kind attack|atomic|sigma|ctid)", idOrKind)
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

func contentCustom(db *store.DB) (*content.Custom, error) {
	return content.NewCustom(content.CustomDeps{
		Sources:    storecontent.NewSources(db),
		Procedures: storecontent.NewProcedures(db),
		Detections: storecontent.NewDetections(db),
		Notes:      storecontent.NewNotes(db),
		Activity:   events.New(activity.New(db)),
	})
}

func newContentExportCustomCommand(a *app) *cobra.Command {
	var (
		format string
		out    string
		typ    string
	)
	cmd := &cobra.Command{
		Use:   "export-custom",
		Short: "Export custom content as YAML or JSON",
		Long: "Writes every custom procedure template, detection rule, and note\n" +
			"to stdout or --output. Format is yaml (default) or json. Pass\n" +
			"--type procedure_templates|detection_rules|notes to narrow the export.\n" +
			"The document is the same shape GET /content/custom/export returns.",
		Args: noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			format = strings.ToLower(strings.TrimSpace(format))
			if format == "" {
				format = "yaml"
			}
			switch format {
			case "yaml", "json":
			default:
				return usagef(cmd, "--format must be yaml or json")
			}
			exportType := content.ExportAll
			if t := strings.TrimSpace(typ); t != "" {
				switch t {
				case "procedure_templates", "detection_rules", "notes":
					exportType = content.ExportType(t)
				default:
					return usagef(cmd, "--type must be procedure_templates, detection_rules, or notes")
				}
			}
			return a.withStore(cmd.Context(), func(ctx context.Context, db *store.DB) error {
				if err := a.requireMigrated(ctx, db); err != nil {
					return err
				}
				svc, err := contentCustom(db)
				if err != nil {
					return err
				}
				doc, err := svc.Export(ctx, exportType)
				if err != nil {
					return err
				}
				payload, err := encodeCustomExport(doc, format)
				if err != nil {
					return err
				}
				if out == "" {
					_, err = cmd.OutOrStdout().Write(payload)
					return err
				}
				return os.WriteFile(out, payload, 0o600)
			})
		},
	}
	cmd.Flags().StringVar(&format, "format", "yaml", "output format (yaml|json)")
	cmd.Flags().StringVarP(&out, "output", "o", "", "write to file instead of stdout")
	cmd.Flags().StringVar(&typ, "type", "", "restrict to procedure_templates|detection_rules|notes")
	return cmd
}

func newContentImportCommand(a *app) *cobra.Command {
	var (
		format   string
		path     string
		dryRun   bool
		failFast bool
	)
	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import v1 testcases.json / knowledgebase YAML into custom content",
		Long: "Parses PurpleOps/Blacklight v1 custom content files and upserts them\n" +
			"under the singleton custom source. Accepts a file, a directory (the\n" +
			"layouts the v1 seeder expected), or a zip. Formats: auto (default),\n" +
			"testcases_json, testcases_yaml, knowledgebase_yaml.\n\n" +
			"Re-import is idempotent: external ids are derived from v1 ids/names\n" +
			"(see docs/content-v1-import.md). Pass --dry-run to parse without writing.",
		Args: noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if path == "" {
				return fmt.Errorf("--path is required (file, directory, or zip)")
			}
			format = strings.TrimSpace(format)
			if format == "" {
				format = "auto"
			}
			return a.withStore(cmd.Context(), func(ctx context.Context, db *store.DB) error {
				if err := a.requireMigrated(ctx, db); err != nil {
					return err
				}
				svc, err := contentCustom(db)
				if err != nil {
					return err
				}
				report, err := svc.Import(ctx, authn.Subject{}, content.ImportRequest{
					Format:   format,
					DryRun:   dryRun,
					FailFast: failFast,
					Path:     path,
				})
				if err != nil {
					return err
				}
				_, err = fmt.Fprintln(cmd.OutOrStdout(), report.Summary())
				if err != nil {
					return err
				}
				for _, w := range report.Warnings {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s: %s\n", w.Path, w.Message)
				}
				for _, e := range report.Errors {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "error: %s: %s\n", e.Path, e.Message)
				}
				if len(report.Errors) > 0 && failFast {
					return fmt.Errorf("import stopped with %d error(s)", len(report.Errors))
				}
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&format, "format", "auto", "auto|testcases_json|testcases_yaml|knowledgebase_yaml")
	cmd.Flags().StringVar(&path, "path", "", "file, directory, or zip to import")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "parse and count without writing")
	cmd.Flags().BoolVar(&failFast, "fail-fast", false, "stop at the first per-file error")
	return cmd
}

func encodeCustomExport(doc content.ExportDoc, format string) ([]byte, error) {
	// Build a JSON-shaped document without dragging gen types into blctl.
	// Intermediate JSON keeps nested arrays/objects simple.
	type exportArg struct {
		Name        string `json:"name" yaml:"name"`
		Description string `json:"description" yaml:"description"`
		Type        string `json:"type" yaml:"type"`
		Default     string `json:"default" yaml:"default"`
	}
	type exportProc struct {
		ID                     string      `json:"id" yaml:"id"`
		SourceID               string      `json:"sourceId" yaml:"sourceId"`
		Version                string      `json:"version" yaml:"version"`
		ExternalID             string      `json:"externalId" yaml:"externalId"`
		Name                   string      `json:"name" yaml:"name"`
		Description            string      `json:"description" yaml:"description"`
		Platforms              []string    `json:"platforms" yaml:"platforms"`
		Executor               string      `json:"executor" yaml:"executor"`
		ElevationRequired      bool        `json:"elevationRequired" yaml:"elevationRequired"`
		Command                string      `json:"command" yaml:"command"`
		Cleanup                string      `json:"cleanup" yaml:"cleanup"`
		InputArgs              []exportArg `json:"inputArgs" yaml:"inputArgs"`
		TechniqueExternalIDs   []string    `json:"techniqueExternalIds" yaml:"techniqueExternalIds"`
		DependencyExecutorName string      `json:"dependencyExecutorName" yaml:"dependencyExecutorName"`
		Dependencies           string      `json:"dependencies" yaml:"dependencies"`
		CreatedAt              string      `json:"createdAt" yaml:"createdAt"`
		UpdatedAt              string      `json:"updatedAt" yaml:"updatedAt"`
	}
	type exportDet struct {
		ID                   string         `json:"id" yaml:"id"`
		SourceID             string         `json:"sourceId" yaml:"sourceId"`
		Version              string         `json:"version" yaml:"version"`
		ExternalID           string         `json:"externalId" yaml:"externalId"`
		Name                 string         `json:"name" yaml:"name"`
		Description          string         `json:"description" yaml:"description"`
		TechniqueExternalIDs []string       `json:"techniqueExternalIds" yaml:"techniqueExternalIds"`
		Level                string         `json:"level" yaml:"level"`
		Status               string         `json:"status" yaml:"status"`
		Logsource            map[string]any `json:"logsource" yaml:"logsource"`
		RuleYAML             string         `json:"ruleYaml" yaml:"ruleYaml"`
		CreatedAt            string         `json:"createdAt" yaml:"createdAt"`
		UpdatedAt            string         `json:"updatedAt" yaml:"updatedAt"`
	}
	type exportNote struct {
		ID                  string   `json:"id" yaml:"id"`
		SourceID            string   `json:"sourceId" yaml:"sourceId"`
		Version             string   `json:"version" yaml:"version"`
		ExternalID          string   `json:"externalId" yaml:"externalId"`
		Title               string   `json:"title" yaml:"title"`
		BodyMarkdown        string   `json:"bodyMarkdown" yaml:"bodyMarkdown"`
		Tags                []string `json:"tags" yaml:"tags"`
		TechniqueExternalID string   `json:"techniqueExternalId" yaml:"techniqueExternalId"`
		CreatedAt           string   `json:"createdAt" yaml:"createdAt"`
		UpdatedAt           string   `json:"updatedAt" yaml:"updatedAt"`
	}
	type exportMeta struct {
		SourceName  string `json:"sourceName" yaml:"sourceName"`
		LicenseSPDX string `json:"licenseSpdx,omitempty" yaml:"licenseSpdx,omitempty"`
		LicenseName string `json:"licenseName,omitempty" yaml:"licenseName,omitempty"`
		LicenseURL  string `json:"licenseUrl,omitempty" yaml:"licenseUrl,omitempty"`
		Attribution string `json:"attribution" yaml:"attribution"`
		ExportedAt  string `json:"exportedAt" yaml:"exportedAt"`
	}
	type exportEnvelope struct {
		Meta               exportMeta   `json:"meta" yaml:"meta"`
		ProcedureTemplates []exportProc `json:"procedureTemplates" yaml:"procedureTemplates"`
		DetectionRules     []exportDet  `json:"detectionRules" yaml:"detectionRules"`
		Notes              []exportNote `json:"notes" yaml:"notes"`
	}

	exportedAt := doc.Meta.ExportedAt.UTC().Format(time.RFC3339)
	out := exportEnvelope{
		Meta: exportMeta{
			SourceName:  doc.Meta.SourceName,
			LicenseSPDX: doc.Meta.LicenseSPDX,
			LicenseName: doc.Meta.LicenseName,
			LicenseURL:  doc.Meta.LicenseURL,
			Attribution: doc.Meta.Attribution,
			ExportedAt:  exportedAt,
		},
		ProcedureTemplates: make([]exportProc, 0, len(doc.ProcedureTemplates)),
		DetectionRules:     make([]exportDet, 0, len(doc.DetectionRules)),
		Notes:              make([]exportNote, 0, len(doc.Notes)),
	}
	for _, p := range doc.ProcedureTemplates {
		var args []exportArg
		if len(p.InputArgs) > 0 {
			if err := json.Unmarshal(p.InputArgs, &args); err != nil {
				// Non-array upstream shapes become empty args in export.
				args = nil
			}
		}
		if args == nil {
			args = []exportArg{}
		}
		out.ProcedureTemplates = append(out.ProcedureTemplates, exportProc{
			ID: p.ID, SourceID: p.SourceID, Version: p.Version, ExternalID: p.ExternalID,
			Name: p.Name, Description: p.Description, Platforms: decodeStringSlice(p.Platforms),
			Executor: p.Executor, ElevationRequired: p.ElevationRequired,
			Command: p.Command, Cleanup: p.Cleanup, InputArgs: args,
			TechniqueExternalIDs:   decodeStringSlice(p.TechniqueExternalIDs),
			DependencyExecutorName: p.DependencyExecutorName, Dependencies: p.Dependencies,
			CreatedAt: p.CreatedAt.UTC().Format(time.RFC3339Nano),
			UpdatedAt: p.UpdatedAt.UTC().Format(time.RFC3339Nano),
		})
	}
	for _, d := range doc.DetectionRules {
		out.DetectionRules = append(out.DetectionRules, exportDet{
			ID: d.ID, SourceID: d.SourceID, Version: d.Version, ExternalID: d.ExternalID,
			Name: d.Name, Description: d.Description,
			TechniqueExternalIDs: decodeStringSlice(d.TechniqueExternalIDs),
			Level:                d.Level, Status: d.RuleStatus, Logsource: decodeObjectMap(d.Logsource),
			RuleYAML:  d.RuleYAML,
			CreatedAt: d.CreatedAt.UTC().Format(time.RFC3339Nano),
			UpdatedAt: d.UpdatedAt.UTC().Format(time.RFC3339Nano),
		})
	}
	for _, n := range doc.Notes {
		out.Notes = append(out.Notes, exportNote{
			ID: n.ID, SourceID: n.SourceID, Version: n.Version, ExternalID: n.ExternalID,
			Title: n.Title, BodyMarkdown: n.BodyMarkdown, Tags: decodeStringSlice(n.Tags),
			TechniqueExternalID: n.TechniqueExternalID,
			CreatedAt:           n.CreatedAt.UTC().Format(time.RFC3339Nano),
			UpdatedAt:           n.UpdatedAt.UTC().Format(time.RFC3339Nano),
		})
	}

	switch format {
	case "json":
		return json.MarshalIndent(out, "", "  ")
	default:
		var buf strings.Builder
		buf.WriteString("# Blacklight custom content export\n")
		buf.WriteString("# Source: " + doc.Meta.SourceName + "\n")
		if doc.Meta.Attribution != "" {
			buf.WriteString("# Attribution: " + doc.Meta.Attribution + "\n")
		}
		buf.WriteString("# Exported-At: " + exportedAt + "\n\n")
		body, err := yaml.Marshal(out)
		if err != nil {
			return nil, err
		}
		buf.Write(body)
		return []byte(buf.String()), nil
	}
}

func decodeStringSlice(raw []byte) []string {
	if len(raw) == 0 {
		return []string{}
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil || out == nil {
		return []string{}
	}
	return out
}

func decodeObjectMap(raw []byte) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil || out == nil {
		return map[string]any{}
	}
	return out
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

func (a *app) printContentJob(j storecontent.Job) error {
	type jobResult struct {
		ID       string `json:"id"`
		SourceID string `json:"sourceId"`
		Kind     string `json:"kind"`
		Status   string `json:"status"`
		Version  string `json:"version,omitempty"`
		Phase    string `json:"phase,omitempty"`
		Message  string `json:"message,omitempty"`
		Error    string `json:"error,omitempty"`
	}
	out := jobResult{
		ID:       j.ID,
		SourceID: j.SourceID,
		Kind:     string(j.Kind),
		Status:   string(j.Status),
		Version:  j.Version,
		Phase:    j.Phase,
		Message:  j.Message,
		Error:    j.Error,
	}
	return a.print(out, func(w *tabwriter.Writer) {
		fmt.Fprintf(w, "ID\t%s\n", out.ID)
		fmt.Fprintf(w, "Source\t%s\n", out.SourceID)
		fmt.Fprintf(w, "Kind\t%s\n", out.Kind)
		fmt.Fprintf(w, "Status\t%s\n", out.Status)
		if out.Version != "" {
			fmt.Fprintf(w, "Version\t%s\n", out.Version)
		}
		if out.Phase != "" {
			fmt.Fprintf(w, "Phase\t%s\n", out.Phase)
		}
		if out.Message != "" {
			fmt.Fprintf(w, "Message\t%s\n", out.Message)
		}
		if out.Error != "" {
			fmt.Fprintf(w, "Error\t%s\n", out.Error)
		}
	})
}
