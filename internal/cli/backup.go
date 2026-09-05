package cli

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/bryanster/blacklight/internal/config"
	"github.com/bryanster/blacklight/internal/store"
)

// backupResult is the JSON shape for the command's output. Every field means the
// same thing in both formats.
type backupResult struct {
	Archive   string   `json:"archive"`
	SizeBytes int64    `json:"sizeBytes"`
	Members   []string `json:"members"`
}

func newBackupCommand(a *app) *cobra.Command {
	var outputPath string

	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Take a consistent backup of the database and evidence",
		Long: "Creates a single archive of the database file, the evidence directory, and\n" +
			"any entrypoint-generated secrets (session.secret, encryption.key) that live\n" +
			"beside the database on the data volume.\n\n" +
			"The server must be stopped before running this — DuckDB gives the database\n" +
			"file to one process at a time, and this command opens it first to prove\n" +
			"nobody else holds it. If the server is still running, the open fails with an\n" +
			"error that says which process to stop.\n\n" +
			"A backup that has never been restored is a hypothesis. Test yours.",
		Args:    noArgs,
		Example: "  blctl backup\n  blctl backup -o /backup/blacklight-$(date -u +%Y%m%d).tar.gz",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := a.settings()
			if err != nil {
				return err
			}

			// Verify no other process holds the database. Open and
			// immediately close — the archive must happen with the DB
			// closed so the WAL checkpoint is flushed.
			db, err := store.Open(cmd.Context(), cfg.Database)
			if err != nil {
				return openFailure(cfg.Database.Path, err)
			}
			if err := db.Close(); err != nil {
				return fmt.Errorf("closing database before backup: %w", err)
			}

			result, err := runBackup(cfg, outputPath)
			if err != nil {
				return err
			}
			return a.print(result, func(w *tabwriter.Writer) {
				fmt.Fprintf(w, "archive\t%s\n", result.Archive)
				fmt.Fprintf(w, "size\t%s\n", humanBytes(result.SizeBytes))
				fmt.Fprintf(w, "members\t%d\n", len(result.Members))
			})
		},
	}

	cmd.Flags().StringVarP(&outputPath, "output", "o", "",
		"archive path (default: blacklight-<timestamp>.backup.tar.gz)")
	return cmd
}

// runBackup creates the archive and returns what it wrote. It assumes the
// database is not held by another process; the caller must have verified that.
func runBackup(cfg config.Tool, outputPath string) (backupResult, error) {
	if outputPath == "" {
		outputPath = fmt.Sprintf("blacklight-%s.backup.tar.gz",
			time.Now().UTC().Format("20060102T150405Z"))
	}

	dbPath := cfg.Database.Path
	dataDir := filepath.Dir(dbPath)
	dbName := filepath.Base(dbPath)
	evDir := cfg.Evidence.Dir

	f, err := os.Create(outputPath)
	if err != nil {
		return backupResult{}, fmt.Errorf("creating archive %q: %w", outputPath, err)
	}
	closed := false
	defer func() {
		if !closed {
			f.Close()
		}
	}()

	gz := gzip.NewWriter(f)
	defer gz.Close()

	tw := tar.NewWriter(gz)
	defer tw.Close()

	var members []string

	// Database file.
	if err := addFileToArchive(tw, dbPath, dbName); err != nil {
		return backupResult{}, err
	}
	members = append(members, dbName)

	// Evidence directory, if it exists.
	if info, err := os.Stat(evDir); err == nil && info.IsDir() {
		prefix := filepath.Base(evDir)
		if err := addDirToArchive(tw, evDir, prefix); err != nil {
			return backupResult{}, fmt.Errorf("archiving evidence: %w", err)
		}
		members = append(members, prefix+"/")
	}

	// Entrypoint-generated secrets beside the database file. These only exist
	// when the operator did not supply the corresponding environment variable.
	for _, name := range []string{"session.secret", "encryption.key"} {
		secPath := filepath.Join(dataDir, name)
		if _, err := os.Stat(secPath); err == nil {
			if err := addFileToArchive(tw, secPath, name); err != nil {
				return backupResult{}, fmt.Errorf("archiving %s: %w", name, err)
			}
			members = append(members, name)
		}
	}

	// Close in order: tar, gzip, file. Each must be closed before the size is
	// meaningful.
	if err := tw.Close(); err != nil {
		return backupResult{}, err
	}
	if err := gz.Close(); err != nil {
		return backupResult{}, err
	}
	if err := f.Close(); err != nil {
		return backupResult{}, err
	}
	closed = true

	info, err := os.Stat(outputPath)
	if err != nil {
		return backupResult{}, fmt.Errorf("statting archive: %w", err)
	}

	return backupResult{
		Archive:   outputPath,
		SizeBytes: info.Size(),
		Members:   members,
	}, nil
}

// addFileToArchive adds a single file to a tar writer under the given name.
func addFileToArchive(tw *tar.Writer, path, name string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("statting %s: %w", path, err)
	}
	hdr, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}
	hdr.Name = name
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}

	//nolint:gosec // G122: operator-owned backup sources, no attacker path into
	// the walk; the stat above and this open are the same trusted file.
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(tw, f); err != nil {
		return err
	}
	return nil
}

// addDirToArchive walks dir and adds every file and subdirectory under prefix.
func addDirToArchive(tw *tar.Writer, dir, prefix string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		name := filepath.Join(prefix, rel)

		// Directories are bare headers; files are written whole by
		// addFileToArchive, so a header and its body always describe the
		// same stat.
		if info.IsDir() {
			hdr, err := tar.FileInfoHeader(info, "")
			if err != nil {
				return err
			}
			hdr.Name = name
			if err := tw.WriteHeader(hdr); err != nil {
				return err
			}
			return nil
		}

		//nolint:gosec // G122: operator-owned backup sources, no attacker path
		// into the walk; stat and open happen on the same trusted path.
		return addFileToArchive(tw, path, name)
	})
}
