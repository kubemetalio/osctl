#!/bin/bash

set -ex

function write_os_iso() {
    # 根分区设备，例如: /dev/sda
    device=$1
    iso_path=$2

    # 解绑根分区块设备下所有挂载目录
    cat /proc/mounts | grep ^${device} | awk '{print $2}' | xargs -n1 -i umount {}
    # 删除块设备所有的分区
    sfdisk --delete ${device}
    # 将操作系统镜像写入根分区设备
    qemu-img dd -f qcow2 -O raw bs=16M if=${iso_path} of=${device}
    # 检查并修复根分区设备
    echo Fix | parted ---pretend-input-tty ${device} print
    # 通知内核重新加载指定设备的分区表，无需重启
    partprobe ${device}
}

function init_data_disk() {
    device=$1
    fs_type=$2

    # 删除块设备所有的分区
    sfdisk --delete ${device}
    # 将分区重建为GPT格式
    echo label:gpt | sfdisk ${device}
    # 通知内核重新加载指定设备的分区表，无需重启
    partprobe ${device}

    case ${fs_type} in
    xfs)
         # 格式化为xfs文件系统
         mkfs.xfs -f -n ftype=1 ${device}
        ;;
    ext4)
         # 格式化为ext4文件系统
         mkfs.ext4 -F ${device}
         ;;
    *)
         echo "invalid file system type: ${fs_type}"
        ;;
    esac
}
