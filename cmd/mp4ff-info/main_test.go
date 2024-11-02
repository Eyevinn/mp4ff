package main

import (
	"testing"
)

func TestOptions(t *testing.T) {
	cases := []struct {
		desc string
		args []string
		err  bool
	}{
		{desc: "no args", args: []string{appName}, err: true},
		{desc: "unknown args", args: []string{appName, "-x"}, err: true},
		{desc: "non-existing file", args: []string{appName, "infile.mp4"}, err: true},
		{desc: "bad file", args: []string{appName, "main.go"}, err: true},
		{desc: "good file", args: []string{appName, "../../mp4/testdata/init.mp4"}, err: false},
		{desc: "good with details", args: []string{appName, "-l", "all:1", "../../mp4/testdata/init.mp4"}, err: false},
		{desc: "version", args: []string{appName, "-version"}, err: false},
		{desc: "help", args: []string{appName, "-h"}, err: false},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			err := run(c.args)
			if c.err && err == nil {
				t.Error("expected error but got nil")
			}
			if !c.err && err != nil {
				t.Errorf("unexpected error: %s", err)
			}
		})
	}
}
