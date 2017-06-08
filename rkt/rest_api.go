package main


import (
	// pkgPod "github.com/rkt/rkt/pkg/pod"
	"github.com/spf13/cobra"
)

var (
	cmdRestApi = &cobra.Command{
		Use:   "rm --uuid-file=FILE | UUID ...",
		Short: "Remove all files and resources associated with an exited pod",
		Long:  `Unlike gc, rm allows users to remove specific pods.`,
		Run:   ensureSuperuser(runWrapper(runAPI)),
	}
	flagApi string
)

func init() {
	cmdRkt.AddCommand(cmdRestApi)
	cmdRestApi.Flags().StringVar(&flagApi, "rest-api", "", "start rkt restful API")
}
func runAPI(cmd *cobra.Command, args []string) (exit int) {
	stdout.Printf("stated API")
	return 254
}