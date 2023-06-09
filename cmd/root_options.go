package cmd

import (
	"fmt"

	"github.com/creasty/defaults"
	"github.com/falcosecurity/driverkit/pkg/driverbuilder/builder"
	"github.com/falcosecurity/driverkit/pkg/kernelrelease"
	"github.com/falcosecurity/driverkit/validate"
	"github.com/go-playground/validator/v10"
	logger "github.com/sirupsen/logrus"
)

// OutputOptions wraps the two drivers that driverkit builds.
type OutputOptions struct {
	Module string `validate:"required_without=Probe,filepath,omitempty,endswith=.ko" name:"--output-module"`
	Probe  string `validate:"required_without=Module,filepath,omitempty,endswith=.o" name:"--output-probe"`
}

type RepoOptions struct {
	Org  string `name:"--repo-org"`
	Name string `name:"--repo-name"`
}

// RootOptions ...
type RootOptions struct {
	Architecture     	  string   `validate:"required,architecture" name:"--architecture"`
	ModuleFilePath    	  string   `validate:"isExistFilePath" name:"--modulefilepath"`
	KernelVersion    	  string   `default:"1" validate:"omitempty" name:"--kernelversion"`
	ModuleDriverName 	  string   `validate:"max=60" name:"--moduledrivername"`
	ModuleDeviceName 	  string   `validate:"excludes=/,max=255" name:"--moduledevicename"`
	KernelRelease    	  string   `validate:"required,ascii" name:"--kernelrelease"`
	Target           	  string   `validate:"required,target" name:"--target"`
	KernelConfigData 	  string   `validate:"omitempty,base64" name:"--kernelconfigdata"` // fixme > tag "name" does not seem to work when used at struct level, but works when used at inner level
	BuilderImage     	  string   `validate:"omitempty,imagename" name:"--builderimage"`
	BuilderRepos     	  []string `validate:"omitempty" name:"--builderrepo"`
	GCCVersion       	  string   `validate:"omitempty,semvertolerant" name:"--gccversion"`
	KernelUrls       	  []string `name:"--kernelurls"`
	Repo             	  RepoOptions
	Output           	  OutputOptions

	LocalKernelDir		  string	`validate:"omitempty,isExistDirPath" name:"--localkerneldir"`
}

func init() {
	validate.V.RegisterStructValidation(RootOptionsLevelValidation, RootOptions{})
}

// NewRootOptions ...
func NewRootOptions() *RootOptions {
	rootOpts := &RootOptions{}
	if err := defaults.Set(rootOpts); err != nil {
		logger.WithError(err).WithField("options", "RootOptions").Fatal("error setting driverkit options defaults")
	}
	return rootOpts
}

// Validate validates the RootOptions fields.
func (ro *RootOptions) Validate() []error {
	if err := validate.V.Struct(ro); err != nil {
		errors := err.(validator.ValidationErrors)
		errArr := []error{}
		for _, e := range errors {
			// Translate each error one at a time
			errArr = append(errArr, fmt.Errorf(e.Translate(validate.T)))
		}
		return errArr
	}

	// check that the kernel versions supports at least one of probe and module
	kr := kernelrelease.FromString(ro.KernelRelease)
	kr.Architecture = kernelrelease.Architecture(ro.Architecture)
	if !kr.SupportsModule() && !kr.SupportsProbe() {
		return []error{fmt.Errorf("both module and probe are not supported by given options")}
	}
	return nil
}

// Log emits a log line containing the receiving RootOptions for debugging purposes.
//
// Call it only after validation.
func (ro *RootOptions) Log() {
	fields := logger.Fields{}
	if ro.Output.Module != "" {
		fields["output-module"] = ro.Output.Module
	}
	if ro.Output.Probe != "" {
		fields["output-probe"] = ro.Output.Probe

	}
	fields["modulefilepath"] = ro.ModuleFilePath
	if ro.KernelRelease != "" {
		fields["kernelrelease"] = ro.KernelRelease
	}
	if ro.KernelVersion != "" {
		fields["kernelversion"] = ro.KernelVersion
	}
	if ro.Target != "" {
		fields["target"] = ro.Target
	}
	fields["arch"] = ro.Architecture
	if len(ro.KernelUrls) > 0 {
		fields["kernelurls"] = ro.KernelUrls
	}
	if ro.Repo.Org != "" {
		fields["repo-org"] = ro.Repo.Org
	}
	if ro.Repo.Name != "" {
		fields["repo-name"] = ro.Repo.Name
	}

	if ro.LocalKernelDir != "" {
		fields["localkerneldir"] = ro.LocalKernelDir
	}

	logger.WithFields(fields).Debug("running with options")
}

func (ro *RootOptions) toBuild() *builder.Build {
	kernelConfigData := ro.KernelConfigData
	if len(kernelConfigData) == 0 {
		kernelConfigData = "bm8tZGF0YQ==" // no-data
	}

	build := &builder.Build{
		TargetType:       		builder.Type(ro.Target),
		ModuleFilePath:   		ro.ModuleFilePath,
		KernelVersion:    		ro.KernelVersion,
		KernelRelease:    		ro.KernelRelease,
		Architecture:     		ro.Architecture,
		KernelConfigData: 		kernelConfigData,
		ModuleOutPutFilePath:   ro.Output.Module,
		ProbeFilePath:    		ro.Output.Probe,
		ModuleDriverName: 		ro.ModuleDriverName,
		ModuleDeviceName: 		ro.ModuleDeviceName,
		GCCVersion:       		ro.GCCVersion,
		BuilderImage:     		ro.BuilderImage,
		BuilderRepos:     		ro.BuilderRepos,
		KernelUrls:       		ro.KernelUrls,
		RepoOrg:          		ro.Repo.Org,
		RepoName:         		ro.Repo.Name,
		LocalKernelDir: 		ro.LocalKernelDir,
	}

	// Always append falcosecurity repo; Note: this is a prio first slice
	// therefore, default falcosecurity repo has lowest prio.
	build.BuilderRepos = append(build.BuilderRepos, "docker.io/falcosecurity/driverkit")

	// attempt the build in case it comes from an invalid config
	kr := build.KernelReleaseFromBuildConfig()
	if len(build.ModuleOutPutFilePath) > 0 && !kr.SupportsModule() {
		build.ModuleOutPutFilePath = ""
		logger.Warningf("Skipping build attempt of module for unsupported kernel version %s", kr.String())
	}
	if len(build.ProbeFilePath) > 0 && !kr.SupportsProbe() {
		build.ProbeFilePath = ""
		logger.Warningf("Skipping build attempt of probe for unsupported kernel version %s", kr.String())
	}

	return build
}

// RootOptionsLevelValidation validates KernelConfigData and Target at the same time.
//
// It reports an error when `KernelConfigData` is empty and `Target` is `vanilla`.
func RootOptionsLevelValidation(level validator.StructLevel) {
	opts := level.Current().Interface().(RootOptions)

	if opts.Target == builder.TargetTypeVanilla.String() ||
		opts.Target == builder.TargetTypeMinikube.String() ||
		opts.Target == builder.TargetTypeFlatcar.String() {
		if len(opts.KernelConfigData) == 0 {
			level.ReportError(opts.KernelConfigData, "kernelConfigData", "KernelConfigData", "required_kernelconfigdata_with_target_vanilla", "")
		}
	}

	if opts.KernelVersion == "" && (opts.Target == builder.TargetTypeUbuntu.String()) {
		level.ReportError(opts.KernelVersion, "kernelVersion", "KernelVersion", "required_kernelversion_with_target_ubuntu", "")
	}

	// Target redhat requires a valid build image (has to be registered in order to download packages)
	if opts.Target == builder.TargetTypeRedhat.String() && opts.BuilderImage == "" {
		level.ReportError(opts.BuilderImage, "builderimage", "builderimage", "required_builderimage_with_target_redhat", "")
	}
}
