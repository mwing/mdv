package main

import (
	"bytes"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
)

func pagerArgs() []string {
	cmdline := os.Getenv("MDV_PAGER")
	if cmdline == "" {
		cmdline = os.Getenv("PAGER")
	}
	args := strings.Fields(cmdline)
	if len(args) == 0 {
		args = []string{"less"}
	}
	if filepath.Base(args[0]) == "less" {
		args = append(args, "-R")
	}
	return args
}

// less treats the placeholder character as binary unless LESSUTFCHARDEF
// (less 632+) says otherwise.
func pagerShowsImages(args []string) bool {
	if filepath.Base(args[0]) != "less" {
		return false
	}
	out, err := exec.Command(args[0], "--version").Output()
	if err != nil {
		return false
	}
	f := strings.Fields(string(out))
	if len(f) < 2 {
		return false
	}
	v, _ := strconv.Atoi(strings.SplitN(f[1], ".", 2)[0])
	return v >= 632
}

func page(args []string, data []byte, images bool) error {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = bytes.NewReader(data)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	cmd.Env = os.Environ()
	if os.Getenv("LESSCHARSET") == "" {
		cmd.Env = append(cmd.Env, "LESSCHARSET=utf-8")
	}
	if images {
		def := "10EEEE:p"
		if v := os.Getenv("LESSUTFCHARDEF"); v != "" {
			def = v + "," + def
		}
		cmd.Env = append(cmd.Env, "LESSUTFCHARDEF="+def)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	// ^C belongs to the pager; dying here would orphan it on the terminal.
	signal.Ignore(os.Interrupt)
	cmd.Wait()
	return nil
}

func edit(name string) error {
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	args := strings.Fields(editor)
	if len(args) == 0 {
		args = []string{"vi"}
	}
	cmd := exec.Command(args[0], append(args[1:], name)...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}
