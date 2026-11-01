//go:build windows

package main

import "syscall"

const detachedProcess = 0x00000008

// detachAttr starts the refresh child without a console and outside the
// parent's process group so it survives the prompt process exiting.
func detachAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | detachedProcess}
}
