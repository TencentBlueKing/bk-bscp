# BSCP部署文档
> 这里展示在单台机器上直接通过二进制方式直接启动，并且你已经通过编译得到了二进制

## 1. 依赖

### 1.1 依赖第三方系统

- 蓝鲸配置平台
- 蓝鲸制品库
- 蓝鲸权限中心
- 蓝鲸 API 网关

### 1.2 依赖第三方组件

- Mysql >= 8.0.17
- Etcd >= 3.2.0
- Redis-Cluster >= 4.0

## 2. BSCP 微服务进程

| 微服务名称           | 描述                             |
| -------------------- | -------------------------------- |
| bk-bscp-apiserver    | 网关微服务，是管理端接口的网关   |
| bk-bscp-authserver   | 鉴权微服务，提供资源操作鉴权功能 |
| bk-bscp-cacheservice | 缓存微服务，提供缓存管理功能     |
| bk-bscp-configserver | 配置微服务，提供各类资源管理功能 |
| bk-bscp-dataservice  | 数据微服务，提供数据管理功能     |
| bk-bscp-feedserver   | 配置拉取微服务，提供拉取配置功能 |
| vault                | 键值加密存储系统               |



## 3. 前置准备

### 3.1 部署Mysql

[参考官方安装教程] https://dev.mysql.com/doc/mysql-installation-excerpt/8.0/en/linux-installation.html

### 3.2 部署Etcd

[参考官方安装教程] https://etcd.io/docs/v3.2/op-guide/

### 3.3 部署Redis-Cluster

[参考官方安装教程] https://redis.io/docs/manual/scaling/

### 3.4 BSCP应用创建

在蓝鲸开发者中心中创建BSCP应用，应用ID为 bk-bscp。如果使用其他应用id，会导致BSCP在权限中心注册权限模型失败，这是因为权限中心某些版本注册权限模型，会校验 SystemID 和 AppCode是否相同导致。

### 3.5 蓝鲸配置平台

BSCP业务列表来自于蓝鲸配置平台。调用蓝鲸配置平台需要 BSCP appCode、appSecret（appCode、appSecret可以在蓝鲸开发者中心中获取），以及一个有权限拉取蓝鲸配置平台业务列表的用户账号。

### 3.6 蓝鲸制品库

BSCP配置文件内容存放于蓝鲸制品库。需要从制品库获取BSCP平台认证Token，并且通过该Token，在制品库创建一个BSCP项目，以及该项目管理员用户账号。

### 3.7 蓝鲸权限中心

BSCP鉴权操作依赖于蓝鲸权限中心。调用蓝鲸权限中心需要 BSCP appCode、appSecret（appCode、appSecret可以在蓝鲸开发者中心中获取），需要在蓝鲸权限中心添加BSCP应用的白名单。

### 3.8 蓝鲸 API 网关

BSCP接口是通过蓝鲸 API 网关对外提供服务。Release包中的api目录下存放了 apiserver 和 feedserver 网关的资源配置、资源文档，需要将其导入对应的网关，并进行版本发布。此外，还需要获取 apiserver 和 feedserver 网关的API公钥(指纹)。

### 3.9 初始化DB

**登陆数据库**

```shell
mysql -uroot -p
```

**BSCP DB初始化**

```bash
# 使用data-service的migrate子命令进行DB初始化，配置文件路径根据实际情况进行调整
./bk-bscp-dataservice migrate up -c ./etc/data_service.yaml
```


## 4. 修改微服务配置文件
参考配置：[config/example](../config/example/) 

前置准备已经获取到了BSCP配置文件中需要的全部必填配置参数，部分 mysql 或者 redis 等配置参数可按需配置，如果不配置则使用默认值，配置文件中有详细说明。apiserver_api_gw_public.key 与 feedserver_api_gw_public.key 文件分别替换为 apiserver 和 feedserver 网关的API公钥(指纹)。

配置文件主要有三份：
```
- vault/vault.hcl : vault 服务启动配置
- vault/root-key.yaml : vault-sidecar vault 解密以及插件注册
- bk-bscp-feed.yaml : feedserver 启动配置
- bk-bscp.yaml : apiserver,authserver,cacheservice,configserver,dataservice 启动配置
- bk-bscp-ui.yaml: ui 前端服务启动配置
```


### 5. 启动服务

#### 5.1 启动 vault 以及初始化
1、先启动 vault
```
./vault server -config=./config/vault/vault_barrier.hcl 
```

2、配置 VAULT_ADDR 环境变量：假设vault监听的是127.0.0.1:8200 
```
export VAULT_ADDR=http://127.0.0.1:8200
```

3、支持init
执行
```
vault-sidecar init
```

获取5个`unsealKeys`和一个`token`，填入vault/root-key.yaml配置中

```
# 设置 VAULT_TOKEN 环境变量，启动dataserver从这里读取
export VAULT_TOKEN=xxxx  
```

4、解密

```
vault-sidecar server -c ./config/vault/root-key.yaml
```

#### 5.2 依次启动其他服务

```shell
bk-bscp-authserver -c ./config/bk-bscp.yaml
bk-bscp-dataservice -c ./config/bk-bscp.yaml
bk-bscp-configserver -c ./config/bk-bscp.yaml
bk-bscp-apiserver -c ./config/bk-bscp.yaml --port 8081
bk-bscp-cachserver -c ./config/bk-bscp.yaml
bk-bscp-feedserver  -c ./config/bk-bscp-feed.yaml
bk-bscp-ui --config ./config/bk-bscp-ui.yaml --bind-address=[替换为机器IP]
```
