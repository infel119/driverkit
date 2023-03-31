package cmd

import (
	"fmt"
	"github.com/falcosecurity/driverkit/pkg/kernelrelease"
	"io"
	"os"
	"runtime"
	"sort"
	"strings"

	"github.com/falcosecurity/driverkit/pkg/driverbuilder/builder"
	"github.com/falcosecurity/driverkit/pkg/version"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	homedir "github.com/mitchellh/go-homedir"
	logger "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

func persistentValidateFunc(rootCommand *RootCmd, rootOpts *RootOptions) func(c *cobra.Command, args []string) error {
	return func(c *cobra.Command, args []string) error {
		// Early exit if detect some error into config flags
		if configOptions.configErrors {
			return fmt.Errorf("exiting for validation errors")
		}
		// Merge environment variables or config file values into the RootOptions instance
		skip := map[string]bool{ // do not merge these
			"config":   true,
			"timeout":  true,
			//"loglevel": true,
			"dryrun":   true,
			"proxy":    true,
		}
		nested := map[string]string{ // handle nested options in config file
			"output-module": "output.module",
			"output-probe":  "output.probe",
		}
		rootCommand.c.Flags().VisitAll(func(f *pflag.Flag) {
			if name := f.Name; !skip[name] {
				if name == "kernelurls" {
					// Slice types need special treatment when used as flags. If we call 'Set(name, value)',
					// rather than replace, it appends. Since viper will already have the cli options set
					// if supplied, we only need this step if rootCommand doesn't already have them e.g.
					// not set on CLI so read from config.
					if cli_urls, err := rootCommand.c.Flags().GetStringSlice(name); err == nil && len(cli_urls) != 0 {
						return
					}
					value := viper.GetStringSlice(name)
					if len(value) != 0 {
						strValue := strings.Join(value, ",")
						rootCommand.c.Flags().Set(name, strValue)
					}
				} else {
					value := viper.GetString(name)
					if value == "" {
						// fallback to nested options in config file, if any
						if nestedName, ok := nested[name]; ok {
							value = viper.GetString(nestedName)
						}
					}
					// set the value, if any, otherwise let the default
					if value != "" {
						rootCommand.c.Flags().Set(name, value)
					}
				}
			}
		})

		//config配置文件中的loglevel也生效 (注:当命令行若设置了--loglevel, 则配置文件的不会生效)
		if logger.GetLevel().String() != configOptions.LogLevel {
			lvl, err := logger.ParseLevel(configOptions.LogLevel)
			if err != nil {
				logger.WithField("--loglevel", configOptions.LogLevel).Errorln("not a valid log level")
				return fmt.Errorf("exiting for validation errors")
			}
			logger.SetLevel(lvl)
		}

		builder.SetOnlineMode(configOptions.OnlineMode)

		// Avoid sensitive info into default values help line
		rootCommand.StripSensitive()

		// We just use ubuntu internally
		if strings.HasPrefix(rootOpts.Target, "ubuntu") {
			rootOpts.Target = "ubuntu"
		}

		// Do not block root or help command to exec disregarding the root flags validity
		if c.Root() != c && c.Name() != "help" && c.Name() != "__complete" && c.Name() != "__completeNoDesc" && c.Name() != "completion" {
			if errs := rootOpts.Validate(); errs != nil {
				for _, err := range errs {
					logger.WithError(err).Error("error validating build options")
				}
				return fmt.Errorf("exiting for validation errors")
			}
			rootOpts.Log()
		}

		return nil
	}
}

// RootCmd wraps the main cobra.Command.
type RootCmd struct {
	c *cobra.Command
}

// NewRootCmd instantiates the root command.
func NewRootCmd() *RootCmd {
	configOptions = NewConfigOptions()  	//初始化配置参数（driverkit的持久化的参数，或者说和实际编译工作无关的一些辅助参数）(的默认值): --config, --loglevel, --timeout, --dryrun, --proxy
	rootOpts := NewRootOptions() 			//初始化命令行参数(的默认值)
	rootCmd := &cobra.Command{   			//根命令
		Use:                   "driverkit",
		Short:                 "A command line tool to build kernel modules and eBPF probes.",
		ValidArgs:             validProcessors,  	 	//driverkit 子命令有效参数: "docker", "kubernetes", "kubernetes-in-cluster"
		ArgAliases:            aliasProcessors,   		//上面三个的别名："docker", "k8s", "k8s-ic"
		Args:                  cobra.OnlyValidArgs, 	//设置子命令有效性检查: 如果driverkit的一级子命令中没在 ValidProcessors 里，就会报错
		DisableFlagsInUseLine: true,
		DisableAutoGenTag:     true,
		Version:               version.String(),
		SilenceUsage: 		   true,
		Run: func(c *cobra.Command, args []string) {
			if len(args) == 0 {
				logger.WithField("processors", validProcessors).Info("specify a valid processor")
			}
			// Fallback to help
			c.Help()
		},
	}
	ret := &RootCmd{
		c: rootCmd,
	}

	rootCmd.PersistentPreRunE = persistentValidateFunc(ret, rootOpts) //注册一个 PreRun, 此函数在 rootCmd.Run 前运行

	flags := rootCmd.Flags()

	targets := builder.BuilderByTarget.Targets()  //调用pkg/driverbuilder/builder包的时候，init()就已经初始化了 BuilderByTarget["ubuntu"] = &ubuntu{} ...
	sort.Strings(targets)


	flags.StringVarP(&configOptions.ConfigFile, "config", "c", configOptions.ConfigFile, "config file path (default $HOME/.driverkit.yaml if exists)")
	flags.StringVarP(&configOptions.LogLevel, "loglevel", "l", configOptions.LogLevel, "log level: [info, debug]")
	flags.IntVar(&configOptions.Timeout, "timeout", configOptions.Timeout, "timeout in seconds")
	flags.BoolVar(&configOptions.DryRun, "dryrun", configOptions.DryRun, "do not actually perform the action")
	flags.StringVar(&configOptions.ProxyURL, "proxy", configOptions.ProxyURL, "the proxy to use to download data")
	flags.BoolVar(&configOptions.OnlineMode, "onlinemode", configOptions.OnlineMode, "get image and kernel header from remote with internet access")

	flags.StringVar(&rootOpts.Output.Module, "output-module", rootOpts.Output.Module, "filepath where to save the resulting kernel module")
	flags.StringVar(&rootOpts.Output.Probe, "output-probe", rootOpts.Output.Probe, "filepath where to save the resulting eBPF probe")
	flags.StringVar(&rootOpts.Architecture, "architecture", runtime.GOARCH, "target architecture for the built driver, one of "+kernelrelease.SupportedArchs.String())
	flags.StringVar(&rootOpts.ModuleFilePath, "modulefilepath", rootOpts.ModuleFilePath, "the tar.gz filepath of kernel module source code")
	flags.StringVar(&rootOpts.KernelVersion, "kernelversion", rootOpts.KernelVersion, "kernel version to build the module for, it's the numeric value after the hash when you execute 'uname -v'")
	flags.StringVar(&rootOpts.KernelRelease, "kernelrelease", rootOpts.KernelRelease, "kernel release to build the module for, it can be found by executing 'uname -v'")
	flags.StringVarP(&rootOpts.Target, "target", "t", rootOpts.Target, "the system to target the build for, one of ["+strings.Join(targets, ",")+"]")
	flags.StringVar(&rootOpts.KernelConfigData, "kernelconfigdata", rootOpts.KernelConfigData, "base64 encoded kernel config data: in some systems it can be found under the /boot directory, in other it is gzip compressed under /proc")
	flags.StringVar(&rootOpts.ModuleDeviceName, "moduledevicename", rootOpts.ModuleDeviceName, "kernel module device name")
	flags.StringVar(&rootOpts.ModuleDriverName, "moduledrivername", rootOpts.ModuleDriverName, "kernel module driver name, i.e. the name you see when you check installed modules via lsmod")
	flags.StringVar(&rootOpts.BuilderImage, "builderimage", rootOpts.BuilderImage, "docker image to be used to build the kernel module and eBPF probe. If not provided, an automatically selected image will be used.")
	flags.StringSliceVar(&rootOpts.BuilderRepos, "builderrepo", rootOpts.BuilderRepos, "list of docker repositories in descending priority order, used to search for builder images. Default falcosecurity/driverkit will always be enforced as lowest priority repo. eg: --builderrepo myorg/driverkit --builderrepo falcosecurity/driverkit")
	flags.StringVar(&rootOpts.GCCVersion, "gccversion", rootOpts.GCCVersion, "enforce a specific gcc version for the build")

	flags.StringSliceVar(&rootOpts.KernelUrls, "kernelurls", nil, "list of kernel header urls (e.g. --kernelurls <URL1> --kernelurls <URL2> --kernelurls \"<URL3>,<URL4>\")")

	flags.StringVar(&rootOpts.Repo.Org, "repo-org", rootOpts.Repo.Org, "repository github organization")
	flags.StringVar(&rootOpts.Repo.Name, "repo-name", rootOpts.Repo.Name, "repository github name")

	flags.StringVar(&rootOpts.LocalKernelDir, "localkerneldir", rootOpts.LocalKernelDir, "get kernel file from local directory")

	viper.BindPFlags(flags)

	// Flag annotations and custom completions
	rootCmd.MarkFlagFilename("config", viper.SupportedExts...) // 参数--config 补全文件的后缀格式
	rootCmd.RegisterFlagCompletionFunc("target", func(c *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return targets, cobra.ShellCompDirectiveDefault
	})
	rootCmd.RegisterFlagCompletionFunc("architecture", func(c *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return kernelrelease.SupportedArchs.Strings(), cobra.ShellCompDirectiveDefault
	})
	rootCmd.RegisterFlagCompletionFunc("loglevel", func(c *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return validLogLevel, cobra.ShellCompDirectiveDefault
	})

	// Subcommands
	rootCmd.AddCommand(NewKubernetesCmd(rootOpts, flags))
	rootCmd.AddCommand(NewKubernetesInClusterCmd(rootOpts, flags))
	rootCmd.AddCommand(NewDockerCmd(rootOpts, flags))
	rootCmd.AddCommand(NewImagesCmd(rootOpts, flags))
	rootCmd.AddCommand(NewCompletionCmd())

	ret.StripSensitive()

	return ret
}

// Sensitive is a list of sensitive environment variable to replace into the help outputs.
var Sensitive = []string{
	"HOME",
}

// StripSensitive removes sensistive info from default values printed into the help messages.
func (r *RootCmd) StripSensitive() {
	for _, s := range Sensitive {
		homeDir := os.Getenv(s)
		for _, childCommand := range r.c.Commands() {
			childCommand.Flags().VisitAll(func(f *pflag.Flag) {
				f.DefValue = strings.ReplaceAll(f.DefValue, homeDir, fmt.Sprintf("$%s", s))
			})
		}
	}
}

// Command returns the underlying cobra.Command.
func (r *RootCmd) Command() *cobra.Command {
	return r.c
}

// SetOutput sets the main command output writer.
func (r *RootCmd) SetOutput(w io.Writer) {
	r.c.SetOut(w)
	r.c.SetErr(w)
	logger.SetOutput(w)
}

// SetArgs proxies the arguments to the underlying cobra.Command.
func (r *RootCmd) SetArgs(args []string) {
	r.c.SetArgs(args)
}

// Execute proxies the cobra.Command execution.
func (r *RootCmd) Execute() error {
	return r.c.Execute()
}

// Start creates the root command and runs it.
//执行顺序：init()里除了cobra.OnInitialize()的语句 -> Start() -> init()中的cobra.OnInitialize(initConfig)
// -> Start()中的rootCmd.PersistentPreRunE = persistentValidateFunc(ret, rootOpts) -> rootCmd.Run()
func Start() {
	root := NewRootCmd()
	if err := root.Execute(); err != nil {
		logger.WithError(err).Fatal("error executing driverkit")
	}
}

func init() {
	logger.SetFormatter(&logger.TextFormatter{
		ForceColors:            true,
		DisableLevelTruncation: false,
		DisableTimestamp:       true,
	})

	//注册preRun函数, 在root.Execute()函数里执行
	cobra.OnInitialize(initConfig)
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {				//读取我们设置的 --config 文件
	if errs := configOptions.Validate(); errs != nil {
		for _, err := range errs {
			logger.WithError(err).Error("error validating config options")
		}
		// configOptions.configErrors should be true here
	}
	if configOptions.ConfigFile != "" {
		viper.SetConfigFile(configOptions.ConfigFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			logger.WithError(err).Debug("error getting the home directory")
			// not setting configOptions.configErrors = true because we fallback to `$HOME/.driverkit.yaml` and try with it
		}

		viper.AddConfigPath(home)
		viper.SetConfigName(".driverkit")
	}

	viper.AutomaticEnv()
	viper.SetEnvPrefix("driverkit")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		logger.WithField("file", viper.ConfigFileUsed()).Info("using config file")
	} else {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found, ignore ...
			logger.Debug("running without a configuration file")
		} else {
			// Config file was found but another error was produced
			logger.WithField("file", viper.ConfigFileUsed()).WithError(err).Debug("error running with config file")
			configOptions.configErrors = true
		}
	}
}
