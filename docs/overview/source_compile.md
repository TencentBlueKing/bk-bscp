# BSCP编译文档

## 1. 编译环境

- golang >= 1.16
- protoc 3.20.0
- protoc-gen-go v1.28.0
- protoc-gen-go-grpc v1.2
- protoc-gen-grpc-gateway v1.16.0
- node >= 18
- npm >10

**注：** BSCP源码文件中的 <u>pkg/protocol/README.md</u> 包含了 protoc 相关依赖的安装教程。

**将go mod设置为on**

```shell
go env -w GO111MODULE="on"
```



## 2. 源码下载

```shell
cd $GOPATH/src
git clone https://github.com/Tencent/bk-bscp.git github.com/TencentBlueKing/bk-bscp
```



## 3. 编译

**进入源码根目录：**

```shell
cd $GOPATH/src/github.com/TencentBlueKing/bk-bscp
```

> 可以参考根目录下Makefile中的指令

### 全部编译
```
make all
```
### 只编译后端服务
```
make build_bscp
```

最终生成的编译后的二进制会在`build/bk-bscp/[对应服务]` 目录中
