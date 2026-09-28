package core

import "os"

// Windows has no PTY sessions (see Session), so no socket directory to check.
func ownedByMe(os.FileInfo) bool { return true }
