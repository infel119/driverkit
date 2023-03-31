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
  {{range $url := .KernelDownloadURLS}}
  curl --silent -o kernel.deb -SL {{ $url }}
  ar x kernel.deb
  tar -xf data.tar.*
  {{end}}
else
  mv /kernel0 kernel0.deb
  mv /kernel1 kernel1.deb
  ar x kernel0.deb
  tar -xf data.tar.*
  ar x kernel1.deb
  tar -xf data.tar.*
fi

cd /tmp/kernel-download/usr/src/
ls -altr
sourcedir=$(find . -type d -name "{{ .KernelHeadersPattern }}" | head -n 1 | xargs readlink -f)

{{ if .BuildModule }}
# Build the module
cd {{ .DriverBuildDir }}
make CC=/usr/bin/gcc-{{ .GCCVersion }} KERNEL_DIR=$sourcedir MODULE_DIR={{ .DriverBuildDir }}
mv *.ko {{ .ModuleFullPath }}
strip -g {{ .ModuleFullPath }}
# Print results
modinfo {{ .ModuleFullPath }}
{{ end }}

{{ if .BuildProbe }}
# Build the eBPF probe
cd {{ .DriverBuildDir }}/bpf
make KERNELDIR=$sourcedir
ls -l probe.o
{{ end }}
