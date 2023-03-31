package builder

import (
	"context"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/system"
	"io"
	"os"
	"path/filepath"
)

const Kernel_File_Path_Under_Root = "build/kernel"

func GetDriverkitRootDir() (string, error) {
	exepath, err := os.Executable()   	// exepath = {driverkitRoot}/_output/bin/driverkit
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(exepath), "../.."), nil
}

//{driverkitRoot}/build/kernel
func GetLocalKernelFileDir() (string, error) {
	driverkitRootDir, err := GetDriverkitRootDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(driverkitRootDir, Kernel_File_Path_Under_Root), nil
}

func GetLocalKernelFiles(localKernelDir, osType string, kernelFileNames []string) ([]string, error) {
	var err error

	if localKernelDir == "" {
		localKernelDir, err = GetLocalKernelFileDir()
		if err != nil {
			return nil, err
		}
	}

	var kernelFilesPath []string
	keys := make(map[string]struct{})

	for _, filename := range kernelFileNames {
		filePath := filepath.Join(localKernelDir, osType, filename)
		_, err = os.Stat(filePath)
		if err != nil && os.IsNotExist(err) {
			continue
		}
		if _, ok := keys[filePath]; !ok {
			kernelFilesPath = append(kernelFilesPath, filePath)
			keys[filePath] = struct{}{}
		}
	}

	if len(kernelFilesPath) == 0 {
		return nil, fmt.Errorf("not found local kernel file in %s", localKernelDir)
	}

	return kernelFilesPath, nil
}

func CopyFileToContainer(ctx context.Context, cli *client.Client, ID, srcPath, dstPath string) error {

	dstInfo := archive.CopyInfo{Path: dstPath}
	dstStat, err := cli.ContainerStatPath(ctx, ID, dstPath)

	if err == nil && dstStat.Mode&os.ModeSymlink != 0 {
		linkTarget := dstStat.LinkTarget
		if !system.IsAbs(linkTarget) {
			// Join with the parent directory.
			dstParent, _ := archive.SplitPathDirEntry(dstPath)
			linkTarget = filepath.Join(dstParent, linkTarget)
		}

		dstInfo.Path = linkTarget
		dstStat, err = cli.ContainerStatPath(ctx, ID, linkTarget)
	}

	if err == nil {
		dstInfo.Exists, dstInfo.IsDir = true, dstStat.Mode.IsDir()
	}

	var (
		content         io.Reader
		resolvedDstPath string
	)

	// Prepare source copy info.
	srcInfo, err := archive.CopyInfoSourcePath(srcPath, true)
	if err != nil {
		return err
	}

	srcArchive, err := archive.TarResource(srcInfo)
	if err != nil {
		return err
	}
	defer srcArchive.Close()

	dstDir, preparedArchive, err := archive.PrepareArchiveCopy(srcArchive, srcInfo, dstInfo)
	if err != nil {
		return err
	}
	defer preparedArchive.Close()

	resolvedDstPath = dstDir
	content = preparedArchive

	return cli.CopyToContainer(ctx, ID, resolvedDstPath, content, types.CopyToContainerOptions{})
}
