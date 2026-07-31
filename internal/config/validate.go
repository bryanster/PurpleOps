package config

import (
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// evidenceDirPerm is owner+group only: evidence is customer data, and the
// directory is created by this process rather than by an operator who might
// have thought about umask.
const evidenceDirPerm fs.FileMode = 0o750

// validate checks what a single value cannot check for itself: values that
// only make sense in combination, and paths that only need reading. It has no
// side effects — see ensurePaths for the half that writes.
func (c *Config) validate() []error {
	var errs []error

	if err := validateListenAddr(c.Server.Addr); err != nil {
		errs = append(errs, &FieldError{Name: envAddr, Value: c.Server.Addr, Msg: err.Error()})
	}

	// Session cookies are Secure, and a browser does not send a Secure cookie
	// over plain http — so a production deployment on http:// cannot log
	// anybody in. Loopback is exempt: browsers treat it as a secure context,
	// which is what makes a local production-mode smoke test possible.
	if c.Env == EnvProduction && !c.Server.BaseURL.IsZero() &&
		c.Server.BaseURL.Scheme == "http" && !isLoopback(c.Server.BaseURL.Hostname()) {
		errs = append(errs, &FieldError{
			Name:  envBaseURL,
			Value: c.Server.BaseURL.String(),
			Msg: fmt.Sprintf("must use https when %s=%s, because browsers do not send "+
				"secure session cookies over plain http (loopback hosts are exempt)",
				envEnv, EnvProduction),
		})
	}

	if c.Report.ChromePath != "" {
		if err := checkExecutable(c.Report.ChromePath); err != nil {
			errs = append(errs, &FieldError{
				Name: envChromePath, Value: c.Report.ChromePath, Msg: err.Error(),
			})
		}
	}

	return errs
}

// ensurePaths is the only part of loading that changes the machine: it creates
// the evidence directory and proves both writable locations really are
// writable, by writing. A stat-based check lies about read-only mounts, about
// full filesystems, and about a process running as root.
func (c *Config) ensurePaths() []error {
	var errs []error

	// DuckDB creates the database file itself, so the requirement is on the
	// directory that will hold it and its WAL.
	dbDir := filepath.Dir(c.Database.Path)
	if err := checkDirWritable(dbDir); err != nil {
		errs = append(errs, &FieldError{
			Name:  envDBPath,
			Value: c.Database.Path,
			Msg:   fmt.Sprintf("needs a writable parent directory, but %s %s", strconv.Quote(dbDir), err),
		})
	}

	if err := os.MkdirAll(c.Evidence.Dir, evidenceDirPerm); err != nil {
		errs = append(errs, &FieldError{
			Name: envEvidenceDir, Value: c.Evidence.Dir, Msg: "could not be created: " + err.Error(),
		})
	} else if err := checkDirWritable(c.Evidence.Dir); err != nil {
		errs = append(errs, &FieldError{
			Name: envEvidenceDir, Value: c.Evidence.Dir, Msg: err.Error(),
		})
	}

	return errs
}

// validateListenAddr accepts what net.Listen accepts: a host:port, where the
// host may be empty (every interface) and the port may be 0 (any free port).
func validateListenAddr(addr string) error {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return errors.New(`must be a host:port listen address, such as ":8080" or "127.0.0.1:8080"`)
	}
	n, err := strconv.Atoi(port)
	if err != nil {
		return errors.New("must have a numeric port")
	}
	if n < 0 || n > 65535 {
		return errors.New("must have a port between 0 and 65535")
	}
	return nil
}

// isLoopback reports whether host is one a browser treats as a secure context
// even over plain http.
func isLoopback(host string) bool {
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return host == "localhost" || strings.HasSuffix(host, ".localhost")
}

// checkDirWritable returns an error phrased to follow a directory name:
// `"/data" does not exist`.
func checkDirWritable(dir string) error {
	info, err := os.Stat(dir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return errors.New("does not exist")
	case err != nil:
		return fmt.Errorf("could not be read: %w", err)
	case !info.IsDir():
		return errors.New("is not a directory")
	}

	probe, err := os.CreateTemp(dir, ".purpleops-write-probe-*")
	if err != nil {
		return errors.New("is not writable by this process")
	}
	if err := probe.Close(); err != nil {
		return fmt.Errorf("is not writable by this process: %w", err)
	}
	if err := os.Remove(probe.Name()); err != nil {
		return fmt.Errorf("is writable, but the probe file %s could not be removed: %w", probe.Name(), err)
	}
	return nil
}

// checkExecutable returns an error phrased to follow a variable name and value.
func checkExecutable(path string) error {
	info, err := os.Stat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return errors.New("must be an executable file, but nothing exists at that path")
	case err != nil:
		return fmt.Errorf("could not be read: %w", err)
	case info.IsDir():
		return errors.New("must be an executable file, not a directory")
	case info.Mode().Perm()&0o111 == 0:
		return errors.New("must be executable")
	}
	return nil
}
