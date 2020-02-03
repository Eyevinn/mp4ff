package cmd

import (
	"fmt"
	"os"

	"github.com/edgeware/gomp4/mp4"
	"github.com/spf13/cobra"
)

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Prints info about a file",
	Long:  "Prints details aboute an ISO/MP4/CMAF file.",
	Run: func(cmd *cobra.Command, args []string) {
		file := cmd.Flag("file")
		fileName := file.Value.String()
		if fileName == "" {
			fmt.Println("Missing filename")
			os.Exit(1)
		}
		fd, err := os.Open(fileName)
		defer fd.Close()
		v, err := mp4.DecodeFile(fd)
		if err != nil {
			fmt.Println(err)
		}
		v.Dump(fd)
	},
}

func init() {
	rootCmd.AddCommand(infoCmd)
	infoCmd.Flags().StringP("file", "f", "", "File to analyze")
}
