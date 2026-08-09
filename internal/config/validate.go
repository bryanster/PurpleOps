package config

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// pathDirPerm is owner+group only: evidence and content trees hold customer and
// upstream data this process creates, rather than directories an operator might
// have thought about umask for.
const pathDirPerm fs.FileMode = 0o750

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

	// The whole reason there are two keys is that they have different blast
	// radii: rotating the session secret signs everybody out, and rotating the
	// encryption key makes every enrolled authenticator unreadable. An operator
	// who pastes one value into both has the second consequence attached to the
	// first lever and no way to find that out except by pulling it.
	if !c.Session.Secret.IsZero() && !c.Encryption.Key.IsZero() &&
		bytes.Equal(c.Session.Secret.Reveal(), c.Encryption.Key.Reveal()) {
		errs = append(errs, &FieldError{
			Name: envEncryptionKey,
			Msg: fmt.Sprintf("must not be the same value as %s: rotating the session secret is how "+
				"you sign everybody out, and sharing it here would make that also destroy every "+
				"enrolled authenticator", envSessionSecret),
		})
	}

	// An idle timeout longer than the lifetime is not a configuration with a
	// generous idle policy — it is one with no idle policy at all, because the
	// absolute expiry always arrives first. An operator who wrote that meant
	// something, and it is not what they would get.
	if c.Session.IdleTimeout > c.Session.Lifetime {
		errs = append(errs, &FieldError{
			Name:  envSessionIdle,
			Value: c.Session.IdleTimeout.String(),
			Msg: fmt.Sprintf("must not be longer than %s (%s), which would end the session first",
				envSessionLifetime, c.Session.Lifetime),
		})
	}

	if c.Events.MaxSubscribers < 1 {
		errs = append(errs, &FieldError{
			Name:  envEventsMaxSubscribers,
			Value: strconv.Itoa(c.Events.MaxSubscribers),
			Msg:   "must be at least 1",
		})
	}
	if c.Events.Buffer < 1 {
		errs = append(errs, &FieldError{
			Name:  envEventsBuffer,
			Value: strconv.Itoa(c.Events.Buffer),
			Msg:   "must be at least 1",
		})
	}
	if c.Events.Heartbeat <= 0 {
		errs = append(errs, &FieldError{
			Name:  envEventsHeartbeat,
			Value: c.Events.Heartbeat.String(),
			Msg:   "must be a positive duration",
		})
	}
	if c.Events.MaxReplayEvents < 0 {
		errs = append(errs, &FieldError{
			Name:  envEventsMaxReplay,
			Value: strconv.Itoa(c.Events.MaxReplayEvents),
			Msg:   "must be at least 0",
		})
	}

	errs = append(errs, c.validateOIDC()...)
	errs = append(errs, c.validateSAML()...)

	if c.Evidence.MaxUploadBytes < 1 {
		errs = append(errs, &FieldError{
			Name: envEvidenceMaxUpload, Value: c.Evidence.MaxUploadBytes.String(),
			Msg: "must be at least 1 byte",
		})
	}
	if c.Evidence.MaxEngagementBytes < 1 {
		errs = append(errs, &FieldError{
			Name: envEvidenceMaxEngagement, Value: c.Evidence.MaxEngagementBytes.String(),
			Msg: "must be at least 1 byte",
		})
	}
	if c.Evidence.MIMEAllowlist != "" {
		for _, m := range strings.Split(c.Evidence.MIMEAllowlist, ",") {
			m = strings.TrimSpace(m)
			if m == "" {
				continue
			}
			if !strings.Contains(m, "/") || strings.HasPrefix(m, "/") || strings.HasSuffix(m, "/") {
				errs = append(errs, &FieldError{
					Name: envEvidenceMIMEAllowlist, Value: c.Evidence.MIMEAllowlist,
					Msg: fmt.Sprintf("invalid MIME type %q: must be type/subtype", m),
				})
			}
		}
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

// validateOIDC checks the single sign-on section as a whole (M1-009), which is
// all-or-nothing: either it is absent, or it is complete enough to attempt a
// login with.
//
// The half-configured case is what this is really for. A deployment with a
// client ID and no issuer looks configured to whoever set it up and offers no
// SSO at all, and the symptom — a login page with no button on it — says
// nothing about which of seven variables is missing. It is a startup error
// naming the variable instead.
//
// What is *not* checked here is whether the provider answers. That is a
// question about somebody else's server, it can change while this one runs, and
// treating it as a startup condition would mean a provider having a bad morning
// stops this deployment from starting at all — with local login, the thing that
// would have got everybody back in, behind the same failed boot. See
// internal/authn/oidc, which discovers lazily and keeps the failure to itself.
func (c *Config) validateOIDC() []error {
	var errs []error

	if !c.OIDC.Enabled() {
		// Named individually rather than as "some OIDC variable is set": the
		// operator is looking for the one they have to add, not for a list of the
		// ones they already wrote.
		for _, set := range []struct {
			name  string
			isSet bool
		}{
			{envOIDCClientID, c.OIDC.ClientID != ""},
			{envOIDCSecret, !c.OIDC.ClientSecret.IsZero()},
			{envOIDCRoleMap, !c.OIDC.RoleMap.IsZero()},
			{envOIDCProvision, c.OIDC.AutoProvision},
		} {
			if set.isSet {
				errs = append(errs, &FieldError{
					Name: set.name,
					Msg: fmt.Sprintf("is set, but %s is not, so single sign-on is off and this value does "+
						"nothing. Set the issuer, or remove this", envOIDCIssuer),
				})
			}
		}
		return errs
	}

	if c.OIDC.ClientID == "" {
		errs = append(errs, &FieldError{
			Name: envOIDCClientID,
			Msg:  fmt.Sprintf("must be set when %s is: the provider has to know who is asking", envOIDCIssuer),
		})
	}
	if c.OIDC.GroupsClaim == "" {
		// Unreachable through Load — the binding has a default and an empty
		// value reads as unset — but reachable through a hand-built Config, and
		// an empty claim name would silently map nobody to anything.
		errs = append(errs, &FieldError{
			Name: envOIDCGroupsClaim,
			Msg:  "must name the claim the provider puts group memberships in",
		})
	}

	// The same rule the base URL is held to, for the same reason and one step
	// further: an authorization code and an ID token cross this connection, so
	// plain http to anything but a loopback development provider hands both to
	// whoever is on the path.
	if c.Env == EnvProduction && c.OIDC.Issuer.Scheme == "http" && !isLoopback(c.OIDC.Issuer.Hostname()) {
		errs = append(errs, &FieldError{
			Name:  envOIDCIssuer,
			Value: c.OIDC.Issuer.String(),
			Msg: fmt.Sprintf("must use https when %s=%s: the token exchange carries an authorization "+
				"code and an ID token (loopback hosts are exempt)", envEnv, EnvProduction),
		})
	}

	return errs
}

// maxClockSkew bounds [SAML.ClockSkew]. Five minutes is already more than any
// correctly configured pair of machines needs, and every minute of it is a
// minute an assertion stays usable after the identity provider said it should
// not be. An operator who needs more than this has a broken clock, and the fix
// for a broken clock is NTP rather than a wider window here.
const maxClockSkew = 5 * time.Minute

// validateSAML checks the SAML section as a whole (M1-010), on the same
// all-or-nothing principle as [Config.validateOIDC] and for the same reason: a
// half-configured section is a login page with a button missing and nothing
// anywhere saying which of eleven variables is at fault.
//
// As with OIDC, nothing here asks whether the identity provider answers, or
// whether it will accept us. Those are facts about somebody else's server and
// about a registration a human has to perform, and neither belongs behind a
// process that will not start.
func (c *Config) validateSAML() []error {
	var errs []error

	if !c.SAML.Enabled() {
		for _, set := range []struct {
			name  string
			isSet bool
		}{
			{envSAMLEntityID, c.SAML.EntityID != ""},
			{envSAMLCertFile, c.SAML.CertFile != ""},
			{envSAMLKeyFile, c.SAML.KeyFile != ""},
			{envSAMLRoleMap, !c.SAML.RoleMap.IsZero()},
			{envSAMLProvision, c.SAML.AutoProvision},
		} {
			if set.isSet {
				errs = append(errs, &FieldError{
					Name: set.name,
					Msg: fmt.Sprintf("is set, but neither %s nor %s is, so SAML is off and this value "+
						"does nothing. Point at the identity provider's metadata, or remove this",
						envSAMLMetaURL, envSAMLMetaFile),
				})
			}
		}
		return errs
	}

	if !c.SAML.MetadataURL.IsZero() && c.SAML.MetadataFile != "" {
		// Not resolved by preferring one: the two can describe different
		// identity providers, and picking silently would mean this deployment
		// trusts a signing certificate the operator believes it does not.
		errs = append(errs, &FieldError{
			Name:  envSAMLMetaFile,
			Value: c.SAML.MetadataFile,
			Msg: fmt.Sprintf("is set and so is %s; there is one identity provider, so set one of "+
				"them and remove the other", envSAMLMetaURL),
		})
	}
	if c.SAML.MetadataFile != "" {
		if err := checkFileReadable(c.SAML.MetadataFile); err != nil {
			errs = append(errs, &FieldError{
				Name: envSAMLMetaFile, Value: c.SAML.MetadataFile, Msg: err.Error(),
			})
		}
	}

	// The key and the certificate are a pair and are checked as one. A service
	// provider with neither cannot sign an authentication request and cannot
	// publish a certificate for the identity provider to check one against, and
	// every commercial identity provider refuses a registration without it.
	for _, file := range []struct {
		name  string
		value string
		what  string
	}{
		{envSAMLCertFile, c.SAML.CertFile, "the certificate the identity provider checks our signatures against"},
		{envSAMLKeyFile, c.SAML.KeyFile, "the key that signs authentication requests"},
	} {
		if file.value == "" {
			errs = append(errs, &FieldError{
				Name: file.name,
				Msg: fmt.Sprintf("must be set when SAML is configured: it is the PEM file holding %s",
					file.what),
			})
			continue
		}
		if err := checkFileReadable(file.value); err != nil {
			errs = append(errs, &FieldError{Name: file.name, Value: file.value, Msg: err.Error()})
		}
	}

	// The private key is the one file in this configuration whose contents are
	// a credential, so it is also the one whose permissions are worth having an
	// opinion about. A warning would be ignored; this is an error naming the
	// chmod that fixes it.
	if c.SAML.KeyFile != "" {
		if err := checkFilePrivate(c.SAML.KeyFile); err != nil {
			errs = append(errs, &FieldError{
				Name: envSAMLKeyFile, Value: c.SAML.KeyFile, Msg: err.Error(),
			})
		}
	}

	if c.SAML.ClockSkew < 0 || c.SAML.ClockSkew > maxClockSkew {
		errs = append(errs, &FieldError{
			Name:  envSAMLClockSkew,
			Value: c.SAML.ClockSkew.String(),
			Msg: fmt.Sprintf("must be between 0 and %s: it widens every validity window in an "+
				"assertion, so a generous value is a replay window held open after the identity "+
				"provider closed it", maxClockSkew),
		})
	}

	// The same rule the base URL and the OIDC issuer are held to: the metadata
	// carries the certificate every assertion is checked against, so fetching it
	// over plain http means whoever is on the path chooses who may sign in here.
	if c.Env == EnvProduction && !c.SAML.MetadataURL.IsZero() &&
		c.SAML.MetadataURL.Scheme == "http" && !isLoopback(c.SAML.MetadataURL.Hostname()) {
		errs = append(errs, &FieldError{
			Name:  envSAMLMetaURL,
			Value: c.SAML.MetadataURL.String(),
			Msg: fmt.Sprintf("must use https when %s=%s: this document carries the certificate every "+
				"assertion is verified against (loopback hosts are exempt)", envEnv, EnvProduction),
		})
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

	for _, dir := range []struct {
		name string
		path string
	}{
		{envEvidenceDir, c.Evidence.Dir},
		{envContentDir, c.Content.Dir},
		{envBrandingDir, c.Report.BrandingDir},
	} {
		if err := os.MkdirAll(dir.path, pathDirPerm); err != nil {
			errs = append(errs, &FieldError{
				Name: dir.name, Value: dir.path, Msg: "could not be created: " + err.Error(),
			})
		} else if err := checkDirWritable(dir.path); err != nil {
			errs = append(errs, &FieldError{
				Name: dir.name, Value: dir.path, Msg: err.Error(),
			})
		}
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

	probe, err := os.CreateTemp(dir, ".blacklight-write-probe-*")
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

// checkFileReadable returns an error phrased to follow a variable name and
// value. It opens the file rather than stat-ing it, because a stat says nothing
// about whether this process may read what is there.
func checkFileReadable(path string) error {
	info, err := os.Stat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return errors.New("must be a readable file, but nothing exists at that path")
	case err != nil:
		return fmt.Errorf("could not be read: %w", err)
	case info.IsDir():
		return errors.New("must be a file, not a directory")
	}

	file, err := os.Open(path)
	if err != nil {
		return errors.New("exists but could not be opened for reading by this process")
	}
	return file.Close()
}

// keyFilePerm is the widest permission a private key file may carry: owner
// only, and not even the group.
const keyFilePerm fs.FileMode = 0o600

// checkFilePrivate refuses a private key that anybody but its owner can read.
//
// This is the check that would normally be a warning nobody reads. It is an
// error because the failure it prevents is silent and permanent: a key
// world-readable in an image layer or a mounted config directory stays that way
// until somebody uses it to mint an authentication request in this deployment's
// name, and nothing in a log would have said so.
func checkFilePrivate(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		// Reported by checkFileReadable, which every caller runs first; saying
		// it twice would be two errors about one missing file.
		return nil //nolint:nilerr // the readable check owns this failure
	}
	if perm := info.Mode().Perm(); perm&^keyFilePerm != 0 {
		return fmt.Errorf("is a private key readable by more than its owner (mode %#o); "+
			"run chmod %#o on it", perm, keyFilePerm)
	}
	return nil
}
