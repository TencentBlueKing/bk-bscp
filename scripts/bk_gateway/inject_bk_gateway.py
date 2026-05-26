#!/usr/bin/env python3
# -*- coding: utf-8 -*-

import json
import sys

def inject_bk_gateway_config(file_path):
    try:
        with open(file_path, 'r', encoding='utf-8') as f:
            swagger_data = json.load(f)
    except Exception as e:
        print(f"读取或解析文件失败 [{file_path}]: {e}")
        return

    # 蓝鲸网关专属配置模板
    bk_extensions = {
        "isPublic": False,  # 默认非公开，需管理员授权后才可见
        "allowApplyPermission": True,  # 允许用户申请权限
        "authConfig": {
            "appVerifiedRequired": True,  # 需要应用认证
            "userVerifiedRequired": False,  # 内部调用免用户校验
            "resourcePermissionRequired": True  # 需要资源权限校验
        }
    }

    paths = swagger_data.get("paths", {})
    inject_count = 0

    # 遍历所有路径，精准注入
    for path, path_item in paths.items():
        # 核心判断：只要路由中包含 /inner/
        if "/inner/" in path:
            for method, method_config in path_item.items():
                if isinstance(method_config, dict):
                    # 注入扩展字段，且不影响该路径下的其他原有属性
                    method_config["x-bk-apigateway-resource"] = bk_extensions
                    inject_count += 1

    if inject_count > 0:
        try:
            with open(file_path, 'w', encoding='utf-8') as f:
                json.dump(swagger_data, f, indent=2, ensure_ascii=False)
            print(f"成功, 已为 {file_path} 中的 {inject_count} 个 inner 接口注入蓝鲸网关配置。")
        except Exception as e:
            print(f"写入文件失败 [{file_path}]: {e}")
    else:
        print(f"未在 {file_path} 中检测到 /inner/ 接口，跳过注入。")

if __name__ == "__main__":
    # 支持从命令行传入多个文件路径
    if len(sys.argv) < 2:
        print("使用方法: python3 inject_bk_gateway.py <file1.json> <file2.json> ...")
        sys.exit(1)
        
    for target_file in sys.argv[1:]:
        inject_bk_gateway_config(target_file)