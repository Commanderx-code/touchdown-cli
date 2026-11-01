package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// detectHost names the innermost multiplexer or terminal around this shell
// that can open a pane from the command line, or "" when none can.
func detectHost(getenv func(string) string) string {
	switch {
	case getenv("TMUX") != "":
		return "tmux"
	case getenv("ZELLIJ") != "":
		return "zellij"
	case getenv("HERDR_ENV") == "1":
		return "herdr"
	case getenv("KITTY_PID") != "":
		return "kitty"
	case getenv("WEZTERM_PANE") != "":
		return "wezterm"
	}
	return ""
}

// shellQuote joins args into one POSIX shell command line, single-quoting
// each argument.
func shellQuote(args []string) string {
	quoted := make([]string, len(args))
	for i, a := range args {
		quoted[i] = "'" + strings.ReplaceAll(a, "'", `'\''`) + "'"
	}
	return strings.Join(quoted, " ")
}

var realRunHostCommand = func(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).Output()
}

// runHostCommand runs a multiplexer command and returns its stdout. Tests
// replace it.
var runHostCommand = realRunHostCommand

// openSplit opens the ticker in a new pane of the given host. It returns the
// process exit code: 0 when a pane was opened, 1 with instructions on errOut
// otherwise.
func openSplit(host, self string, forwarded []string, errOut io.Writer) int {
	child := append([]string{self, "-status", "-watch"}, forwarded...)
	var err error
	switch host {
	case "tmux":
		err = hostRun("tmux", append([]string{"split-window", "-h", "-l", "36", "-d"}, child...)...)
	case "zellij":
		err = hostRun("zellij", append([]string{"run", "-d", "right", "-n", "NFL", "--"}, child...)...)
	case "kitty":
		err = hostRun("kitten", append([]string{"@", "launch", "--location=vsplit", "--keep-focus"}, child...)...)
	case "wezterm":
		err = hostRun("wezterm", append([]string{"cli", "split-pane", "--right", "--percent", "35", "--"}, child...)...)
	case "herdr":
		err = herdrSplit(child)
	default:
		printManual(errOut, child)
		return 1
	}
	if err != nil {
		fmt.Fprintf(errOut, "%s failed: %v\n", host, err)
		printManual(errOut, child)
		return 1
	}
	return 0
}

func hostRun(name string, args ...string) error {
	_, err := runHostCommand(name, args...)
	return describeExit(err)
}

// herdrSplit opens a pane with herdr's CLI, then types the command into it.
func herdrSplit(child []string) error {
	out, err := runHostCommand("herdr", "pane", "split", "--current", "--direction", "right", "--ratio", "0.35", "--no-focus")
	if err != nil {
		return describeExit(err)
	}
	var reply struct {
		Result struct {
			Pane struct {
				ID string `json:"pane_id"`
			} `json:"pane"`
		} `json:"result"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(out, &reply); err != nil {
		return fmt.Errorf("unexpected reply from herdr pane split: %w", err)
	}
	if reply.Result.Pane.ID == "" {
		if reply.Error != "" {
			return errors.New(reply.Error)
		}
		return errors.New("herdr pane split returned no pane id")
	}
	_, err = runHostCommand("herdr", "pane", "run", reply.Result.Pane.ID, shellQuote(child))
	return describeExit(err)
}

// describeExit folds a command's stderr into its error so the user sees
// what the multiplexer complained about.
func describeExit(err error) error {
	var exit *exec.ExitError
	if errors.As(err, &exit) && len(exit.Stderr) > 0 {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(exit.Stderr)))
	}
	return err
}

func printManual(errOut io.Writer, child []string) {
	fmt.Fprintln(errOut, "Run this in a new pane:")
	fmt.Fprintln(errOut, "  "+shellQuote(child))
}
