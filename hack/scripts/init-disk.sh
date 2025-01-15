#!/bin/bash

set -e

option_type=
device=
root_device_num=
root_size=
swap_size=
fs_type=
iso_path=

function usage() {
    shellname="$(echo ${0##*/})"

    echo -e "usage:
	-h                           显示帮助
	-t [option_type]             操作类别：root(格式化根分区), data(格式化数据盘), iso(iso镜像写入到根分区)
	-d [device]                  根分区或数据分区设备，例如 sda，sdb 或 nvme0n1
	-n [root_device_num]         根分区序号，例如根分区是sda4，则该值为4
	-s [root_size]               根分区大小，rest或者具体数值500，单位G
	-w [swap_size]               swap分区大小，例如 128 （单位G），0表示不做swap分区
	-f [fs_type]                 data分区的文件系统类型：xfs, ext4
	-p [iso_path]                iso镜像的文件路径
"

    echo "example:
# 1、option_type=root: 初始化根分区，假设跟分区设备为 sda4。
# 1) NOSWAP; / rest;
bash init-disk.sh -t root -d sda -n 4 -s rest -w 0

# 2) NOSWAP; / 1024G; /data rest; /data xfs
bash init-disk.sh -t root -d sda -n 4 -s 500 -w 0 -f xfs

# 3) SWAP 128G; / rest;
bash init-disk.sh -t root -d sda -n 4 -s rest -w 128

# 4) SWAP 128G; / 500G; /data rest; /data xfs
bash init-disk.sh -t root -d sda -n 4 -s 500 -w 128 -f xfs

# 2、option_type=data: 格式化data盘
bash init-disk.sh -t data -d sdb -f xfs

# 3、option_type=iso: iso镜像写入到根分区
bash init-disk.sh -t iso -d sda -p iso.qcow2
"
    exit
}

while getopts "ht:d:n:s:w:f:p:" opt; do
    case $opt in
    t)
        option_type=$OPTARG
        echo "option_type VALUE: $OPTARG"
        ;;
    d)
        device=$OPTARG
        echo "device VALUE: $OPTARG"
        ;;
    n)
        root_device_num=$OPTARG
        echo "root_device_num VALUE: $OPTARG"
        ;;
    s)
        root_size=$OPTARG
        echo "root_size VALUE: $OPTARG"
        ;;
    w)
        swap_size=$OPTARG
        echo "swap_size VALUE: $OPTARG"
        ;;
    f)
        fs_type=$OPTARG
        echo "fs_type VALUE: $OPTARG"
        ;;
    p)
        iso_path=$OPTARG
        echo "iso_path VALUE: $OPTARG"
        ;;
    h) usage ;;
    ?) usage ;;
    esac
done

# 将系统镜像写入根分区设备
function write_os_iso() {
    # 根分区设备，例如: sda
    local root_device=$1
    local iso_path=$2
    device="/dev/$root_device"

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

# 格式化磁盘为xfs或ext4的文件系统
function format_disk() {
    local device=$1
    local fs_type=$2

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

# 通过传入root_device的名字，例如（sda,nvme0n1）拼接成设备全名
# nvme0n1 => nvme0n1p1
# sda => sda1
function root_part_by_num() {
    local disk="$1"   # 第一个参数：磁盘名称（如 nvme0n1）
    local num="$2"    # 第二个参数：分区编号（如 1）

    # 判断磁盘名称是否以 nvme 或 nbd 开头
    if [[ "$disk" == nvme* || "$disk" == nbd* ]]; then
        echo "${disk}p${num}"  # 添加 'p' 分隔符
    else
        echo "${disk}${num}"  # 直接拼接分区编号
    fi
}

# 获取指定磁盘设备上最后一段空闲空间的起始位置（单位为 MiB）以便新建分区。
function free_end() {
    # 例如 sda 或 nvme0n1
    local root_device=$1
    free_end=$(parted /dev/${root_device} unit MiB print free | grep 'Free Space'|tail -1 | awk '{print $1}' | sed 's/MiB//')
    echo $free_end
}

# 将剩余的磁盘空间创建一个新的磁盘分区(根分区或data分区)
function create_free_partition() {
    local disk_device=$1

    # 获取空闲空间的起始位置
    disk_start=$(parted /dev/${disk_device} unit MiB print free | grep 'Free Space'|tail -1 | awk '{print $1}' | sed 's/MiB//')
    # 将剩余空间做一个新分区
    echo Ignore | parted ---pretend-input-tty /dev/${disk_device} -- mkpart primary ${disk_start}MiB 100%
}

# 创建指定大小的磁盘分区(根分区或data分区)
function create_partition_by_size() {
    # 例如 sda 或 nvme0n1
    local disk_device=$1
    # 分区大小，单位MiB
    local disk_size=$2

    # 获取分区的起始位置和终止位置
    disk_start=$(parted /dev/${disk_device} unit MiB print free | grep 'Free Space'|tail -1 | awk '{print $1}' | sed 's/MiB//')
    disk_end=$((1+ root_start + ${disk_size}))
    # 创建指定大小的分区
    echo Ignore | parted ---pretend-input-tty /dev/${disk_device} -- mkpart primary ${disk_start}MiB ${disk_end}MiB
}

# 调整指定分区的大小
function resize_root_partition() {
    # 例如 sda 或 nvme0n1
    local root_device=$1
    # sda4 => 4
    local root_device_num=$2
    # 1. rest; 2. 500G
    # 转换为MiB单位
    local root_size=$3

    case ${root_size} in
    rest)
        # 类型1： / rest;
        # root_size=rest, 表示把剩余的磁盘大小全部作为root盘的大小。
        # 通过 parted 工具将指定分区的大小调整为磁盘的 100% (占用剩余未分配空间), 假设磁盘为 /dev/sda，分区编号为 1。
        # 命令为：echo Yes | parted ---pretend-input-tty /dev/sda -- resizepart 1 100%
        echo Yes | parted ---pretend-input-tty /dev/${root_device} -- resizepart ${root_device_num} 100%
        ;;
    *)
        # 类型2: / 500G; /data rest
        # 如果指定了root盘的大小，例如500G
        # 获取根分区的起始位置和终止位置
        root_start=$(parted /dev/${root_device} unit MiB print | awk '/./{end=$2} END{print end}' | sed 's/MiB//')
        root_end=$((1+ root_start + ${root_size}))
        # 调整根分区大小
        echo Yes | parted ---pretend-input-tty /dev/${root_device} -- resizepart ${root_device_num} ${root_end}MiB
        ;;
    esac
}

# 创建swap分区
function create_swap_partition() {
    # 例如 sda 或 nvme0n1
    local root_device=$1
    # sda4 => 4
    local root_device_num=$2
    # 128G
    local swap_size=$3

    # 删除root分区
    echo Ignore | parted ---pretend-input-tty /dev/${root_device} rm ${root_device_num}
    # 创建swap分区
    create_partition_by_size ${root_device} ${swap_size}
    # 格式化分区为 Swap 类型
    swap_device=$(root_part_by_num ${root_device} ${root_device_num})
    mkswap /dev/${swap_device}
}

# 创建data分区
function create_data_partition() {
    # 例如 sda 或 nvme0n1
    local data_device=$1
    # sda4 => 4
    local data_device_num=$2
    # xfs或ext4
    local fs_type=$3

    # 将剩余空间创建data分区
    create_free_partition ${data_device}
    # 格式化data分区, 例如：/dev/sda5
    full_data_device=$(root_part_by_num ${data_device} ${data_device_num})
    format_disk ${full_data_device} ${fs_type}
}

# root disk config
# 1. NOSWAP; / rest; /boot 2G;
# 2. NOSWAP; / 1024G; /boot 2G; /data rest
# 3. SWAP 128G; / rest; /boot 2G;
# 4. SWAP 128G; / 500G; /boot 2G; /data rest
function create_root_partition() {
    # 例如 sda 或 nvme0n1
    local root_device=$1
    # sda4 => 4
    local root_device_num=$2
    # 1. rest; 2. 500G 3. 转换为MiB单位
    local root_size=$3
    # 1. 0 (不需要创建swap分区) ; 2. 128G (swap分区大小)
    local swap_size=$4
    # ext4, xfs
    local fs_type=$5

    case ${swap_size} in
    0) # 不创建swap分区
        # 创建root分区，示例1. NOSWAP; / rest;
        echo "resize root partition"
        resize_root_partition $root_device $root_device_num $root_size
        # 如果需要创建data分区，示例2. NOSWAP; / 1024G; /data rest
        if [ "$root_size" != "rest" ]; then
            echo "create data partition"
            data_device_num=$(($root_device_num + 1))
            create_data_partition $root_device $data_device_num $fs_type
        fi
        ;;
    *) # 创建swap分区
        echo "create swap partition"
        create_swap_partition ${root_device} ${root_device_num} ${swap_size}
        if [ "$root_size" = "rest" ]; then
            # 将剩余空间创建root分区。示例3. SWAP 128G; / rest
            echo "create root partition"
            create_free_partition $root_device
        else
            # 创建指定大小的root分区，并且剩余空间创建data分区。示例4. SWAP 128G; / 500G; /data rest
            echo "create root partition by size"
            # 基于剩余空间创建下一个分区（swap分区后的第二个分区）
            create_partition_by_size $root_device $root_size

            echo "create data partition"
            # 将从swap分区开始的第三个分区做data分区
            data_device_num=$(($root_device_num + 2))
            create_data_partition $root_device $data_device_num $fs_type
        fi
        ;;
    esac

    # 如果文件系统检查成功，则调整文件系统大小（通常是与磁盘分区大小匹配）。并忽略报错
    e2fsck -y -f /dev/$root_device && resize2fs /dev/$root_device || true
}

# data disk config
# 1. /data
# 2. /data1 /data2 /data3
# 格式化数据盘
function init_data_disk() {
    # 设备名，例如 sdb
    local data_device=$1
    local fs_type=$2
    device="/dev/$data_device"

    # 删除块设备所有的分区
    sfdisk --delete ${device}
    # 将分区重建为GPT格式
    echo label:gpt | sfdisk ${device}
    # 通知内核重新加载指定设备的分区表，无需重启
    partprobe ${device}
    # 格式化文件系统
    format_disk ${device} ${fs_type}
}

# main函数
main() {
    case ${option_type} in
    root)
        echo "format root disk"
        # 将单位G换算成MiB
        root_size_by_mb=$(($root_size * 1024))
        swap_size_by_mb=$(($swap_size * 1024))
        # 格式化根分区，swap分区，data分区
        create_root_partition $device $root_device_num $root_size_by_mb $swap_size_by_mb $fs_type
        ;;
    data)
        echo "format data disk"
        init_data_disk $device $fs_type
        ;;
    iso)
        echo "write iso image to disk"
        write_os_iso $device $iso_path
        ;;
    esac
}

main
