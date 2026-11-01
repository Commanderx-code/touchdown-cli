//go:build !windows

package main

import "syscall"

// detachAttr puts the refresh child in its own session so it survives the
// prompt process exiting and never holds the terminal.
func detachAttr() *syscall.SysProcAttr { return &syscall.SysProcAttr{Setsid: true} }
