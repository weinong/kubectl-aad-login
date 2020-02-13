package main

import (
	"os"

	"github.com/spf13/pflag"
	"github.com/weinong/kubernetes-aad-plugin/pkg/cmd"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

func main() {
	flags := pflag.NewFlagSet("kubectl-aad-login", pflag.ExitOnError)
	pflag.CommandLine = flags

	root := cmd.NewCmd(genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr})
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
