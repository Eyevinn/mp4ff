package cmd

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"os"
	"strconv"

	"github.com/edgeware/gomp4/mp4"
	"github.com/edgeware/gomp4/tools"
	"github.com/spf13/cobra"
)

var resegCmd = &cobra.Command{
	Use:   "resegment",
	Short: "Resegment a file with new boundaries",
	Long:  "Resegment a segmented file based on PTS",
	Run: func(cmd *cobra.Command, args []string) {
		infile := cmd.Flag("infile").Value.String()
		outfile := cmd.Flag("outfile").Value.String()
		boundary, err := strconv.ParseUint(cmd.Flag("boundary").Value.String(), 10, 64)
		if err != nil {
			fmt.Println("Bad boundary")
			os.Exit(1)

		}
		if infile == "" || outfile == "" {
			fmt.Println("Must specify infile and outfile")
			os.Exit(1)
		}
		fmt.Println(infile, outfile, boundary)
		ifd, err := os.Open(infile)
		defer ifd.Close()
		if err != nil {
			log.Fatalln(err)
		}
		parsedMp4, err := mp4.DecodeFile(ifd)
		if err != nil {
			log.Fatalln(err)
		}
		newMp4 := tools.Resegment(parsedMp4, boundary)
		if err != nil {
			log.Fatalln(err)
		}
		ofd, err := os.Create(outfile)
		defer ofd.Close()
		if err != nil {
			log.Fatalln(err)
		}
		newMp4.Encode(ofd)
	},
}

func init() {
	rootCmd.AddCommand(resegCmd)
	resegCmd.Flags().StringP("infile", "i", "", "Infile to resegment")
	resegCmd.Flags().StringP("outfile", "o", "", "Outfile")
	resegCmd.Flags().Int64P("boundary", "b", 0, "Resegment timestamp (PTS)")
}
