#!/bin/bash
set -xeuo pipefail

rm -Rf {{ .DriverBuildDir }}
mkdir {{ .DriverBuildDir }}
rm -Rf /tmp/module-download
mkdir -p /tmp/module-download

tar -xzf kernel-module.tar.gz -C /tmp/module-download
mv /tmp/module-download/*/* {{ .DriverBuildDir }}/

#cp /driverkit/module-Makefile {{ .DriverBuildDir }}/Makefile
#bash /driverkit/fill-driver-config.sh {{ .DriverBuildDir }}

# Fetch the kernel
mkdir /tmp/kernel-download
cd /tmp/kernel-download
if [[ "${MODE}" == "online" ]];then
  curl --silent -o kernel.rpm -SL {{ .KernelDownloadURL }}
else
  mv /kernel0 kernel.rpm
fi

rpm2cpio kernel.rpm | cpio --extract --make-directories
rm -Rf /tmp/kernel
mkdir -p /tmp/kernel
mv usr/src/kernels/*/* /tmp/kernel

{{ if .BuildModule }}
# Build the module
cd {{ .DriverBuildDir }}
make CC=/usr/bin/gcc-{{ .GCCVersion }} KERNEL_DIR=/tmp/kernel MODULE_DIR={{ .DriverBuildDir }}
mv *.ko {{ .ModuleFullPath }}
strip -g {{ .ModuleFullPath }}
# Print results
modinfo {{ .ModuleFullPath }}
{{ end }}

{{ if .BuildProbe }}
# Build the eBPF probe
cd {{ .DriverBuildDir }}/bpf
make KERNELDIR=/tmp/kernel
ls -l probe.o
{{ end }}