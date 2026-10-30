package tui

import "os/exec"

// detach: pigeon on Windows never starts a serve of its own here
// (keepGateway is for Linux).
func detach(*exec.Cmd) {}
