#!/bin/bash
set -eo pipefail

set -x

# 使用方法提示
usage() {
    echo "用法: $0 -i <镜像名称> [-n <节点列表>] [-d <输出目录>] [-c]"
    echo "选项:"
    echo "  -i  要处理的镜像名称（必需）"
    echo "  -n  远程节点列表，逗号分隔（可选）"
    echo "  -d  输出目录（默认：./images）"
    echo "  -c  处理完成后清理临时文件"
    echo "示例:"
    echo "  $0 -i nginx:latest -n node1,node2"
    exit 1
}

# 检查命令是否存在
check_command() {
    if ! command -v "$1" &> /dev/null; then
        echo "错误：未找到命令 $1"
        exit 1
    fi
}

# 主处理函数
process_image() {
    local image=$1
    local output_dir=$2
    local image_tar
    
    # 创建输出目录
    mkdir -p "$output_dir"
    
    # 处理文件名（替换特殊字符）
    local filename=$(echo "${image//\//_}" | sed 's/:/_/g').tar
    image_tar="${output_dir}/${filename}"
    
    echo "步骤1: 正在拉取镜像 ${image}"
    if ! docker pull "$image"; then
        echo "错误：镜像拉取失败"
        exit 1
    fi
    
    echo "步骤2: 正在导出镜像到 ${image_tar}"
    if ! docker save -o "$image_tar" "$image"; then
        echo "错误：镜像导出失败"
        exit 1
    fi
    
    echo "步骤3: 正在导入镜像到本地 containerd"
    if ! ctr --namespace=k8s.io image import "$image_tar"; then
        echo "错误：本地镜像导入失败"
        exit 1
    fi
}

# 远程分发函数
remote_import() {
    local image_tar=$1
    local nodes=(${2//,/ })
    local filename=$(basename "$image_tar")
    
    for node in "${nodes[@]}"; do
        echo "正在处理节点 ${node}:"
        
        # 传输文件
        echo "步骤4: 正在传输镜像到 ${node}"
        if ! scp "$image_tar" "${node}:/tmp/${filename}"; then
            echo "错误：文件传输失败到 ${node}"
            continue
        fi
        
        # 远程导入
        echo "步骤5: 正在在 ${node} 上导入镜像"
        ssh "$node" "ctr --namespace=k8s.io image import /tmp/${filename} && rm -f /tmp/${filename}"
        
        if [ $? -eq 0 ]; then
            echo "成功：${node} 镜像导入完成"
        else
            echo "错误：${node} 镜像导入失败"
        fi
    done
}

# 初始化变量
IMAGE=""
NODES=""
OUTPUT_DIR="./images"
CLEANUP=false

# 解析参数
while getopts ":i:n:d:c" opt; do
    case $opt in
        i) IMAGE=$OPTARG ;;
        n) NODES=$OPTARG ;;
        d) OUTPUT_DIR=$OPTARG ;;
        c) CLEANUP=true ;;
        \?) usage ;;
    esac
done

# 检查必需参数
if [ -z "$IMAGE" ]; then
    usage
fi

# 检查依赖命令
check_command docker
check_command ctr
check_command scp
check_command ssh

# 主流程
process_image "$IMAGE" "$OUTPUT_DIR"

# 获取生成的实际文件路径
filename=$(echo "${IMAGE//\//_}" | sed 's/:/_/g').tar
IMAGE_TAR="${OUTPUT_DIR}/${filename}"

# 远程节点处理
if [ -n "$NODES" ]; then
    remote_import "$IMAGE_TAR" "$NODES"
fi

# 清理文件
if [ "$CLEANUP" = true ]; then
    echo "正在清理临时文件..."
    rm -f "$IMAGE_TAR"
fi

echo "所有操作已完成！"