#!/bin/bash

images=(
driverkit-builder-any-x86_64_gcc8.0.0_gcc6.0.0_gcc5.0.0_gcc4.9.0_gcc4.8.0
driverkit-builder-any-x86_64_gcc10.0.0_gcc9.0.0
driverkit-builder-any-x86_64_gcc12.0.0_gcc11.0.0
driverkit-builder-centos-x86_64_gcc4.8.5
driverkit-builder-any-aarch64_gcc8.0.0_gcc6.0.0_gcc5.0.0_gcc4.9.0_gcc4.8.0
driverkit-builder-any-aarch64_gcc10.0.0_gcc9.0.0
driverkit-builder-any-aarch64_gcc12.0.0_gcc11.0.0
qemu-user-static)

#if get images from docker hub:
#docker pull falcosecurity/driverkit-builder-any-x86_64_gcc12.0.0_gcc11.0.0
#docker pull multiarch/qemu-user-static

if [[ "${1}" == "-h" || "${1}" == "--help" ]];then
  echo -e "Load docker images for driverkit to build kernel module in container.\nUsage:\n\timages_load.sh\n\timages_load.sh \$imagesDirectory\t\tfor example: images_load.sh build/images/\n"
  exit 0
fi

if [ $EUID -ne 0 ];then
  echo "error: need root privilege, please run it with sudo"
  exit 1
fi

if [[ $(dirname $0 | grep -E "^/") != "" ]];then
  imagesDir=$(dirname $0)/images
else
  imagesDir=`pwd`/$(dirname $0)/images
fi

if [[ "${1}" != "" ]];then
  if [ ! -d ${1} ];then
    echo "${1}: No such directory or it is a file"
    exit 1
  fi
  imagesDir=${1}
fi

allImages=$(sudo docker images)

for image in ${images[*]}
do
  if [[ $(echo "${allImages}" | grep -E "^(falcosecurity|multiarch)/${image}")  == "" ]];then
    echo -e "\e[0;32mloading image:\e[0m ${image}..."
    sudo docker load -i ${imagesDir}/${image}.tar
  else
    echo -e "\e[0;32mskipping image:\e[0m ${image} exists"
  fi
done
