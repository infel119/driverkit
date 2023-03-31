package builder

import (
	"fmt"
	"github.com/falcosecurity/driverkit/pkg/kernelrelease"
)

// Build contains the info about the on-going build.
type Build struct {
	TargetType       		Type
	KernelConfigData 		string
	KernelRelease    		string
	KernelVersion    		string
	ModuleFilePath    		string
	Architecture     		string
	ModuleOutPutFilePath   	string
	ProbeFilePath    		string
	ModuleDriverName 		string			//暂时没用到，驱动名直接由Makefile决定
	ModuleDeviceName		string
	BuilderImage     		string
	BuilderRepos     		[]string
	KernelUrls      		[]string
	GCCVersion       		string
	RepoOrg          		string
	RepoName         		string
	Images           		ImagesMap

	LocalKernelDir			string
}

var onlineMode bool
func SetOnlineMode(mode bool) {
	onlineMode = mode
}
func IsOnlineMode() bool {
	return onlineMode
}

func (b *Build) KernelReleaseFromBuildConfig() kernelrelease.KernelRelease {
	kv := kernelrelease.FromString(b.KernelRelease)
	kv.Architecture = kernelrelease.Architecture(b.Architecture)
	return kv
}

func (b *Build) toGithubRepoArchive() string {
	return fmt.Sprintf("https://github.com/%s/%s", b.RepoOrg, b.RepoName)
}

func (b *Build) ToConfig() Config {
	return Config{
		DriverName:      b.ModuleDriverName,
		DeviceName:      b.ModuleDeviceName,
		DownloadBaseURL: b.toGithubRepoArchive(),
		Build:           b,
	}
}
