#!/usr/bin/env python3
# -*- coding: utf-8 -*-

import json
import sys


DEFAULT_EXTENSIONS = {
    "isPublic": True,
    "allowApplyPermission": True,
    "authConfig": {
        "appVerifiedRequired": True,
        "userVerifiedRequired": True,
        "resourcePermissionRequired": True
    }
}

# 平台级运维接口，业务方不应在权限中心看到或申请；实际调用走对应的 /inner/ 资源。
PLATFORM_ONLY_PATHS = {
    "/api/v1/config/manage_config_kv",
}


def build_platform_only_extensions():
    """平台级运维接口公开路径的网关配置：隐藏文档并禁止申请权限。"""
    return {
        "isPublic": False,
        "allowApplyPermission": False,
        "authConfig": {
            "appVerifiedRequired": True,
            "userVerifiedRequired": False,
            "resourcePermissionRequired": True
        }
    }


def build_inner_extensions(method, path):
    """为 /inner/ 接口动态构建蓝鲸网关扩展配置，通过 backend.path 将网关路径映射到实际后端路径。"""
    backend_path = path.replace("/inner/", "/", 1)
    return {
        "isPublic": False,
        "allowApplyPermission": True,
        "backend": {
            "type": "HTTP",
            "method": method,
            "path": backend_path,
            "matchSubpath": False,
            "timeout": 0,
            "upstreams": {},
            "transformHeaders": {}
        },
        "authConfig": {
            "appVerifiedRequired": True,
            "userVerifiedRequired": False,
            "resourcePermissionRequired": True
        }
    }


def inject_bk_gateway_config(file_path):
    try:
        with open(file_path, 'r', encoding='utf-8') as f:
            swagger_data = json.load(f)
    except Exception as e:
        print(f"读取或解析文件失败 [{file_path}]: {e}")
        return

    paths = swagger_data.get("paths", {})
    inner_count = 0
    default_count = 0
    platform_only_count = 0

    for path, path_item in paths.items():
        is_inner = "/inner/" in path
        is_platform_only = path in PLATFORM_ONLY_PATHS
        for method, method_config in path_item.items():
            if not isinstance(method_config, dict):
                continue
            if is_inner:
                method_config["x-bk-apigateway-resource"] = build_inner_extensions(method, path)
                inner_count += 1
            elif is_platform_only:
                method_config["x-bk-apigateway-resource"] = build_platform_only_extensions()
                platform_only_count += 1
            else:
                method_config.setdefault("x-bk-apigateway-resource", DEFAULT_EXTENSIONS)
                default_count += 1

    inject_count = inner_count + default_count + platform_only_count
    if inject_count > 0:
        try:
            with open(file_path, 'w', encoding='utf-8') as f:
                json.dump(swagger_data, f, indent=2, ensure_ascii=False)
            print(f"成功, 已为 {file_path} 注入蓝鲸网关配置: "
                  f"inner={inner_count}, default={default_count}, platform_only={platform_only_count}。")
        except Exception as e:
            print(f"写入文件失败 [{file_path}]: {e}")
    else:
        print(f"未在 {file_path} 中检测到接口，跳过注入。")

if __name__ == "__main__":
    # 支持从命令行传入多个文件路径
    if len(sys.argv) < 2:
        print("使用方法: python3 inject_bk_gateway.py <file1.json> <file2.json> ...")
        sys.exit(1)
        
    for target_file in sys.argv[1:]:
        inject_bk_gateway_config(target_file)