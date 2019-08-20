package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	cliflag "k8s.io/component-base/cli/flag"
)

/*
TODO:
1. child command
**/

//SecureServingOptions list secure port
type SecureServingOptions struct {
	BindPort string
}

// AddFlags add secure port options to specific flag set.
func (s *SecureServingOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&s.BindPort, "secure-port", s.BindPort, "The port on which to serve HTTPS with authentication and authorization.")
}

//Options has all the params needed to run a Scheduler
type Options struct {
	ComponentConfig string

	SecureServing           *SecureServingOptions
	CombinedInsecureServing string
	Authentication          string
	Authorization           string
	Deprecated              string

	// ConfigFile is the location of the scheduler server's configuration file.
	ConfigFile string

	// WriteConfigTo is the path where the default configuration will be written.
	WriteConfigTo string

	Master string
}

// NewOptions returns default scheduler app options.
func NewOptions() (*Options, error) {
	s := &SecureServingOptions{
		BindPort: "443",
	}

	o := &Options{
		ComponentConfig:         "componentconfig",
		SecureServing:           s,
		CombinedInsecureServing: "insecure",
		Authentication:          "authentication",
		Authorization:           "authorization",
		Deprecated:              "deprecated",

		ConfigFile:    "",
		WriteConfigTo: "",
		Master:        "",
	}

	return o, nil
}

// Flags returns flags for a specific scheduler by section name
func (o *Options) Flags() (nfs cliflag.NamedFlagSets) {
	fs := nfs.FlagSet("misc")
	fs.StringVar(&o.ConfigFile, "configfile", o.ConfigFile, "The path to the configuration file. Flags override values in this file.")
	fs.StringVar(&o.WriteConfigTo, "writeconfigto", o.WriteConfigTo, "If set, write the configuration values to this file and exit.")
	fs.StringVar(&o.Master, "master", o.Master, "The address of the Kubernetes API server (overrides any value in kubeconfig)")

	o.SecureServing.AddFlags(nfs.FlagSet("secure serving"))

	return nfs

}

func runCommand(cmd *cobra.Command, args []string, opts *Options) error {

	return nil
}

//Newcommand create a *cobra.Command object
func Newcommand() *cobra.Command {
	opts, err := NewOptions()
	if err != nil {
		os.Exit(1)
	}

	cmd := &cobra.Command{
		Use:     "kubesched",
		Long:    "kubesched is copied by kube-scheduler for learning.",
		Version: "v0.0.1.0",
		Run: func(cmd *cobra.Command, args []string) {
			if err := runCommand(cmd, args, opts); err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}
		},
	}

	//cf := cmd.Flags()
	//of := opts.Flags()

	return cmd
}

func main() {
	command := Newcommand()
	command.Execute()
}
