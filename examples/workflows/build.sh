#!/bin/bash

# PipelineX 工作工作流运行器构建脚本

set -e

# 颜色定义
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}=== PipelineX 工作流运行器构建脚本 ===${NC}"
echo ""

# 获取脚本所在目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "当前目录: $SCRIPT_DIR"
echo ""

# 检查 Go 环境
if ! command -v go &> /dev/null; then
    echo -e "${YELLOW}错误: 未找到 Go 环境，请先安装 Go${NC}"
    exit 1
fi

echo "Go 版本: $(go version)"
echo ""

# 编译 main.go
echo -e "${YELLOW}正在编译 main.go...${NC}"
go build -o runner main.go

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ 编译成功!${NC}"
    echo ""
    echo "可执行文件: $SCRIPT_DIR/runner"
    echo ""
    echo "运行方式:"
    echo "  cd $SCRIPT_DIR"
    echo "  ./runner"
else
    echo -e "${YELLOW}✗ 编译失败${NC}"
    exit 1
fi
