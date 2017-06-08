package main


import (
	// pkgPod "github.com/rkt/rkt/pkg/pod"
	"github.com/spf13/cobra"
)

var (
	cmdRestApi = &cobra.Command{
		Use:   "api",
		Short: "restful api",
		Long:  `Start rkt Restful API.`,
		Run:   ensureSuperuser(runWrapper(runAPI)),
	}
	flagListen string
)

func init() {
	cmdRkt.AddCommand(cmdRestApi)
	cmdRestApi.Flags().StringVar(&flagListen, "--listen", "", "start rkt restful API")
}
func runAPI(cmd *cobra.Command, args []string) (exit int) {
	stdout.Printf("stated API")
	return 254
}