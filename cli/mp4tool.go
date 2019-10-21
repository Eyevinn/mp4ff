package main

import (
	"fmt"
	"os"
	"time"

	cli "github.com/jawher/mow.cli"
	"github.com/jfbus/mp4"
	"github.com/jfbus/mp4/filter"
)

func main() {
	cmd := cli.App("mp4tool", "MP4 command line tool")

	cmd.Command("info", "Displays information about a media", func(cmd *cli.Cmd) {
		file := cmd.StringArg("FILE", "", "the file to display")
		cmd.Action = func() {
			fd, err := os.Open(*file)
			defer fd.Close()
			v, err := mp4.Decode(fd)
			if err != nil {
				fmt.Println(err)
			}
			v.Dump()
		}
	})

	cmd.Command("clip", "Generates a clip", func(cmd *cli.Cmd) {
		start := cmd.IntOpt("s start", 0, "start time (sec)")
		duration := cmd.IntOpt("d duration", 10, "duration (sec)")
		src := cmd.StringArg("SRC", "", "the source file name")
		dst := cmd.StringArg("DST", "", "the destination file name")
		cmd.Action = func() {
			in, err := os.Open(*src)
			if err != nil {
				fmt.Println(err)
			}
			defer in.Close()
			v, err := mp4.Decode(in)
			if err != nil {
				fmt.Println(err)
			}
			out, err := os.Create(*dst)
			if err != nil {
				fmt.Println(err)
			}
			defer out.Close()
			filter.EncodeFiltered(out, v, filter.Clip(time.Duration(*start)*time.Second, time.Duration(*duration)*time.Second))
		}
	})

	cmd.Command("copy", "Decodes a media and reencodes it to another file", func(cmd *cli.Cmd) {
		src := cmd.StringArg("SRC", "", "the source file name")
		dst := cmd.StringArg("DST", "", "the destination file name")
		cmd.Action = func() {
			in, err := os.Open(*src)
			if err != nil {
				fmt.Println(err)
			}
			defer in.Close()
			v, err := mp4.Decode(in)
			if err != nil {
				fmt.Println(err)
			}
			out, err := os.Create(*dst)
			if err != nil {
				fmt.Println(err)
			}
			defer out.Close()
			v.Encode(out)
		}
	})
	cmd.Run(os.Args)
}
