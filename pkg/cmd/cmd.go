package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

var (
	cmdExample = `
	# login to aad
	%[1]s aad login
`

	errNoContext = fmt.Errorf("no context is currently set, use %q to select a new one", "kubectl config use-context <context>")
	errNoUser    = fmt.Errorf("no user")
)

// Options provides information required to update
// the current context on a user's KUBECONFIG
type Options struct {
	configFlags *genericclioptions.ConfigFlags

	resultingAuthInfo *api.AuthInfo

	authInfo string

	rawConfig           api.Config
	args                []string
	useServicePrincipal bool
	useUserPrincipal    bool
	forceRefresh        bool

	genericclioptions.IOStreams
}

func stringptr(str string) *string { return &str }

// NewOptions provides an instance of Options with default values
func NewOptions(streams genericclioptions.IOStreams) *Options {
	configFlags := &genericclioptions.ConfigFlags{
		KubeConfig: stringptr(""),
	}
	return &Options{
		configFlags: configFlags,

		IOStreams: streams,
	}
}

// NewCmd provides a cobra command wrapping Options
func NewCmd(streams genericclioptions.IOStreams) *cobra.Command {
	o := NewOptions(streams)

	cmd := &cobra.Command{
		Use:          "aad login [flags]",
		Short:        "login to azure active directory and populate kubeconfig with AAD tokens",
		Example:      fmt.Sprintf(cmdExample, "kubectl"),
		SilenceUsage: true,
		RunE: func(c *cobra.Command, args []string) error {
			if err := o.Complete(c, args); err != nil {
				return err
			}
			if err := o.Validate(); err != nil {
				return err
			}
			if err := o.Run(); err != nil {
				return err
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&o.useServicePrincipal, "service-principal", o.useServicePrincipal,
		fmt.Sprintf("if true, it will use service principal to login using %s and %s envrionment variables",
			envServicePrincipalClientID, envServicePrincipalClientSecret))
	cmd.Flags().BoolVar(&o.useUserPrincipal, "user-principal", o.useUserPrincipal,
		fmt.Sprintf("if true, it will use user principal to login using %s and %s envrionment variables",
			envROPCUsername, envROPCPassword))
	cmd.Flags().BoolVar(&o.forceRefresh, "force", o.forceRefresh, "if true, it will always login and disregard the access token in kubeconfig")
	o.configFlags.AddFlags(cmd.Flags())

	return cmd
}

// Complete sets all information required for updating the current context
func (o *Options) Complete(cmd *cobra.Command, args []string) error {
	o.args = args

	clientConfig := o.configFlags.ToRawKubeConfigLoader()
	var err error
	o.rawConfig, err = clientConfig.RawConfig()
	if err != nil {
		return err
	}

	currentContext, exists := o.rawConfig.Contexts[o.rawConfig.CurrentContext]
	if !exists {
		return errNoContext
	}
	currentAuthInfo, exists := o.rawConfig.AuthInfos[currentContext.AuthInfo]
	if !exists {
		return errNoUser
	}

	o.authInfo = currentContext.AuthInfo
	o.resultingAuthInfo = currentAuthInfo.DeepCopy()

	return nil
}

// Validate ensures that all required arguments and flag values are provided
func (o *Options) Validate() error {
	if len(o.args) > 0 {
		return errors.New("no argument is allowed")
	}
	if o.useServicePrincipal && o.useUserPrincipal {
		return errors.New("service principal and user principal cannot be used at the same time")
	}
	return nil
}

// Run runs
func (o *Options) Run() error {
	if o.resultingAuthInfo.AuthProvider == nil {
		return nil
	}

	if o.resultingAuthInfo.AuthProvider.Name != azureAuthProvider {
		// not azure auth provider. skip
		return nil
	}
	ts, err := newTokenRefresher(o.resultingAuthInfo.AuthProvider.Config, o.useServicePrincipal, o.useUserPrincipal, o.forceRefresh)
	if err != nil {
		return err
	}
	if err := ts.Refresh(); err != nil {
		return err
	}

	o.resultingAuthInfo.AuthProvider.Config = ts.ToCfg()
	o.rawConfig.AuthInfos[o.authInfo] = o.resultingAuthInfo
	return clientcmd.ModifyConfig(clientcmd.NewDefaultPathOptions(), o.rawConfig, true)
}
