![蓝鲸服务配置中心.png](./docs/logo/bscp_zh.png)

---

[![license](https://img.shields.io/badge/license-mit-brightgreen.svg?style=flat)](https://github.com/TencentBlueKing/bk-bscp/blob/master/LICENSE) [![Release Version](https://img.shields.io/github/v/release/TencentBlueKing/bk-bscp?sort=semver)](https://github.com/TencentBlueKing/bk-bscp/releases) ![BK Pipelines Status](https://api.bkdevops.qq.com/process/api/external/pipelines/projects/bscp/p-9ba3c5bdcc874723b0c71cc5a9e3a0cd/badge?X-DEVOPS-PROJECT-ID=bscp) [![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](https://github.com/TencentBlueKing/bk-bscp/pulls)

[EnglishDocs](./README_EN.md)

蓝鲸服务配置中心（BlueKing Service Configuration Provider）是为业务提供服务配置管理功能，平台与蓝鲸其它产品打通，为业务提供一站式解决方案。
平台以业务作为顶层设计，隔离各个业务的服务配置，使用蓝鲸权限中心做服务级别的权限管控，支持业务传统与容器模式的业务配置托管，目前支持文件型配置与键值（KV）型配置。

## Overview

使用BSCP能做什么？

- 业务进程与配置文件分离
- 配置版本管理和热更新
- 全时灰度控制
- 模板化管理配置
- 支持 文件、KV、表格类型配置
- 提供容器、主机、SDK、API、流水线插件等多种消费方式

了解BSCP更详细功能，请参考蓝鲸服务配置中心[白皮书](https://bk.tencent.com/docs/markdown/BSCP/UserGuide/Introduction/product_introduction.md)

## Getting Started

- [下载与编译](docs/install/source_compile.md)
- [安装部署](docs/install/deploy-guide.md)
- [API使用说明](./docs/apidoc/api.md)

## Contributing

对于项目感兴趣，想一起贡献并完善项目请参阅[contributing](./CONTRIBUTING.md)。

[腾讯开源激励计划](https://opensource.tencent.com/contribution) 鼓励开发者的参与和贡献，期待你的加入。

## Support

- 阅读 [源码](https://github.com/TencentBlueKing/bk-bscp)
- 参考 [bk-bscp产品文档](https://bk.tencent.com/docs/markdown/BSCP/UserGuide/Introduction/product_introduction.md)
- 了解蓝鲸社区相关信息：蓝鲸社区版交流QQ群 495299374
- 直接反馈 [issue](https://github.com/TencentBlueKing/bk-bscp/issues)，我们会定期查看与答复
- 蓝鲸社区 [问答反馈](https://bk.tencent.com/s-mart/community)

## FAQ

- [FAQ](https://bk.tencent.com/docs/markdown/ZH/BSCP/UserGuide/FAQ/faq.md)
- [蓝鲸文档中心](https://bk.tencent.com/docs/)

## BlueKing Community

- [BK-BCS](https://github.com/TencentBlueKing/bk-bcs)：蓝鲸容器管理平台是以容器技术为基础，为微服务业务提供编排管理的基础服务平台。
- [BK-CMDB](https://github.com/TencentBlueKing/bk-cmdb)：蓝鲸配置平台（蓝鲸CMDB）是一个面向资产及应用的企业级配置管理平台。
- [BK-CI](https://github.com/TencentBlueKing/bk-ci)：蓝鲸持续集成平台是一个开源的持续集成和持续交付系统，可以轻松将你的研发流程呈现到你面前。
- [BK-PaaS](https://github.com/TencentBlueKing/bk-PaaS)：蓝鲸PaaS平台是一个开放式的开发平台，让开发者可以方便快捷地创建、开发、部署和管理SaaS应用。
- [BK-JOB](https://github.com/TencentBlueKing/bk-job)：蓝鲸作业平台(Job)是一套运维脚本管理系统，具备海量任务并发处理能力。
- [BK-SOPS](https://github.com/TencentBlueKing/bk-sops)：标准运维（SOPS）是通过可视化的图形界面进行任务流程编排和执行的系统，是蓝鲸体系中一款轻量级的调度编排类SaaS产品

## License

bk-bscp是基于MIT协议， 详细请参考[LICENSE](./LICENSE.txt)。
