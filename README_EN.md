![BlueKing Service Configuration Provider.png](./docs/logo/bscp_zh.png)

---

[![license](https://img.shields.io/badge/license-mit-brightgreen.svg?style=flat)](https://github.com/TencentBlueKing/bk-bscp/blob/master/LICENSE) [![Release Version](https://img.shields.io/github/v/release/TencentBlueKing/bk-bscp?sort=semver)](https://github.com/TencentBlueKing/bk-bscp/releases) ![BK Pipelines Status](https://api.bkdevops.qq.com/process/api/external/pipelines/projects/bscp/p-9ba3c5bdcc874723b0c71cc5a9e3a0cd/badge?X-DEVOPS-PROJECT-ID=bscp) [![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](https://github.com/TencentBlueKing/bk-bscp/pulls)

[Chinese Documentation](./README.md)

BlueKing Service Configuration Provider (BSCP) is a service configuration management platform for businesses. It integrates with other BlueKing products to provide a one-stop solution for businesses.
The platform is designed around businesses, isolating service configurations for each business. It uses BlueKing Permission Center for service-level access control, supports traditional and containerized business configuration hosting, and currently supports file-based configurations and key-value (KV) configurations.

## Overview

What can you do with BSCP?

- Separate business processes from configuration files
- Manage configuration versions and hot updates
- Full-time grayscale control
- Template-based configuration management
- Support for configurations in file, kv, and table types
- Provide multiple consumption methods such as containers, hosts, SDKs, APIs, and pipeline plugins

For more detailed information about BSCP, please refer to the BlueKing Container Management Platform [White Paper](https://bk.tencent.com/docs/markdown/BSCP/UserGuide/Introduction/product_introduction.md)

## Getting Started

- [Download and Compile](docs/install/source_compile.md)
- [Installation and Deployment](docs/install/deploy-guide.md)
- [API Usage Instructions](./docs/apidoc/api.md)

## Contributing

If you are interested in the project and would like to contribute and improve it, please refer to the [contributing](./CONTRIBUTING.md) guidelines.

[Tencent Open Source Incentive Program](https://opensource.tencent.com/contribution) encourages developers to participate and contribute. We look forward to your involvement.

## Support

- Read the [source code](https://github.com/TencentBlueKing/bk-bscp)
- Refer to the [bk-bscp product documentation](https://bk.tencent.com/docs/markdown/BSCP/UserGuide/Introduction/product_introduction.md)
- Learn about the BlueKing community: BlueKing Community QQ Group 495299374
- Provide direct feedback through [issues](https://github.com/TencentBlueKing/bk-bscp/issues), we regularly review and respond
- BlueKing Community [Q&A Feedback](https://bk.tencent.com/s-mart/community)

## FAQ

- [FAQ](https://bk.tencent.com/docs/markdown/ZH/BSCP/UserGuide/FAQ/faq.md)
- [BlueKing Documentation Center](https://bk.tencent.com/docs/)

## BlueKing Community

- [BK-BCS](https://github.com/TencentBlueKing/bk-bcs): BlueKing Container Management Platform is a foundational service platform for orchestrating and managing microservices based on container technology.
- [BK-CMDB](https://github.com/TencentBlueKing/bk-cmdb): BlueKing Configuration Platform (BlueKing CMDB) is an enterprise-level configuration management platform for assets and applications.
- [BK-CI](https://github.com/TencentBlueKing/bk-ci): BlueKing Continuous Integration Platform is an open-source continuous integration and continuous delivery system that easily presents your development process to you.
- [BK-PaaS](https://github.com/TencentBlueKing/bk-PaaS): BlueKing PaaS Platform is an open development platform that allows developers to easily create, develop, deploy, and manage SaaS applications.
- [BK-JOB](https://github.com/TencentBlueKing/bk-job): BlueKing Job Platform is an operations script management system with massive task concurrency processing capabilities.
- [BK-SOPS](https://github.com/TencentBlueKing/bk-sops): Standard Operation (SOPS) is a system for orchestrating and executing task flows through a visual graphical interface, a lightweight scheduling and orchestration SaaS product in the BlueKing ecosystem.

## License

bk-bscp is based on the MIT License, for details please refer to the [LICENSE](./LICENSE.txt).
