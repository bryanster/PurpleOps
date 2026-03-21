package handler

import "os"

// Standard file permission constants used throughout the handler package.
const (
	DirPerm  os.FileMode = 0o750
	FilePerm os.FileMode = 0o600
)
