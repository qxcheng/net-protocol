// +build amd64

package sleep

// See commit_noasm.go for a description of commitSleep.
func commitSleep(g uintptr, waitingG *uintptr) bool
