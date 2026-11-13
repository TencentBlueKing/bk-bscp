#!/usr/bin/env python3
# -*- coding: utf-8 -*-

import json
import re
import sys


# 与蓝鲸网关 apigateway/apis/web/resource/constants.py 的 PATH_PATTERN 保持一致。
# 网关对前端路径和 backend_config.path 都用该正则校验，冒号等字符会导致导入报
# “校验失败: backend_config.path: 输入值不匹配要求的模式”。
GATEWAY_PATH_PATTERN = re.compile(r"^/[\w{}/.-]*$")

# protoc-gen-openapiv2 为 additional_bindings 生成的 operationId 是 “主绑定名 + 序号”，
# 序号随绑定顺序变化，且看不出路由差异。网关资源名、MCP 工具名都取自 operationId，
# 因此按路径特征改成稳定的名字，主绑定的名字保持不变。
GENERATED_SUFFIX_PATTERN = re.compile(r"^(.+?)(\d+)$")
ENV_SCOPE_PATTERN = re.compile(r"/projects/\{\w+\}/envs/\{\w+\}")
PROJECT_SCOPE_PATTERN = re.compile(r"/projects/\{\w+\}")
INNER_TOKEN = "Inner"
ENV_TOKEN = "Env"
PROJECT_TOKEN = "Project"

# 蓝鲸网关要求 “MCP Server 名称 + 工具名称” 不超过 64 字符。MCP Server 名称形如
# bk-bscp-prod-file-config（24 字符），工具名取自 operationId，因此 operationId 不能超过 40 字符。
# /inner/ 接口仅供内部系统以应用身份调用，不注册为 MCP 工具，不受此限制。
MAX_OPERATION_ID_LENGTH = 40
# 改名后仍超长时，按顺序缩写 rpc 部分，直到满足长度要求。
RPC_ABBREVIATIONS = (("Revisions", "Revs"),)

DEFAULT_EXTENSIONS = {
    "isPublic": True,
    "allowApplyPermission": True,
    "authConfig": {
        "appVerifiedRequired": True,
        "userVerifiedRequired": True,
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


def iter_operations(paths):
    for path, path_item in paths.items():
        for method, method_config in path_item.items():
            if isinstance(method_config, dict):
                yield path, method, method_config


def build_binding_operation_id(base_id, path):
    """按路径特征生成 additional_bindings 的 operationId，如 Config_Inner_Env_Publish。"""
    service, _, rpc = base_id.partition("_")
    is_inner = "/inner/" in path
    scopes = [INNER_TOKEN] if is_inner else []
    if ENV_SCOPE_PATTERN.search(path):
        scopes.append(ENV_TOKEN)
    elif PROJECT_SCOPE_PATTERN.search(path):
        scopes.append(PROJECT_TOKEN)

    operation_id = "_".join([service, *scopes, rpc])
    if is_inner:
        return operation_id
    for word, abbreviation in RPC_ABBREVIATIONS:
        if len(operation_id) <= MAX_OPERATION_ID_LENGTH:
            break
        rpc = rpc.replace(word, abbreviation)
        operation_id = "_".join([service, *scopes, rpc])
    return operation_id


def normalize_operation_ids(paths):
    """把带序号的 operationId 改为稳定名字，返回改名数量与校验错误（重复、超长）。"""
    operation_ids = {config.get("operationId") for _, _, config in iter_operations(paths)}
    renamed = 0
    for path, _, config in iter_operations(paths):
        match = GENERATED_SUFFIX_PATTERN.match(config.get("operationId") or "")
        if not match or match.group(1) not in operation_ids:
            continue
        config["operationId"] = build_binding_operation_id(match.group(1), path)
        renamed += 1

    owners = {}
    for path, method, config in iter_operations(paths):
        owners.setdefault(config.get("operationId"), []).append(f"{method.upper()} {path}")
    errors = [f"operationId 重复: {op_id} <- {', '.join(routes)}"
              for op_id, routes in owners.items() if len(routes) > 1]
    errors += [f"operationId 超过 {MAX_OPERATION_ID_LENGTH} 字符: {op_id}（{len(op_id)}）<- {', '.join(routes)}"
               for op_id, routes in owners.items()
               if op_id and len(op_id) > MAX_OPERATION_ID_LENGTH and any("/inner/" not in r for r in routes)]
    return renamed, errors


def collect_path_errors(paths):
    """校验注入结果中的路径能否被蓝鲸网关接受。未显式配置 backend 时，网关以前端路径作为后端路径。"""
    errors = []
    for path, path_item in paths.items():
        if not GATEWAY_PATH_PATTERN.match(path):
            errors.append(f"前端请求路径不合法: {path}")
        for method, method_config in path_item.items():
            if not isinstance(method_config, dict):
                continue
            extensions = method_config.get("x-bk-apigateway-resource") or {}
            backend_path = (extensions.get("backend") or {}).get("path") or path
            if not GATEWAY_PATH_PATTERN.match(backend_path):
                errors.append(f"后端请求路径不合法: {method.upper()} {path} -> {backend_path}")
    return errors


def inject_bk_gateway_config(file_path):
    try:
        with open(file_path, 'r', encoding='utf-8') as f:
            swagger_data = json.load(f)
    except Exception as e:
        print(f"读取或解析文件失败 [{file_path}]: {e}")
        return False

    paths = swagger_data.get("paths", {})
    inner_count = 0
    default_count = 0

    for path, path_item in paths.items():
        is_inner = "/inner/" in path
        for method, method_config in path_item.items():
            if not isinstance(method_config, dict):
                continue
            if is_inner:
                method_config["x-bk-apigateway-resource"] = build_inner_extensions(method, path)
                inner_count += 1
            else:
                method_config.setdefault("x-bk-apigateway-resource", DEFAULT_EXTENSIONS)
                default_count += 1

    inject_count = inner_count + default_count
    if inject_count == 0:
        print(f"未在 {file_path} 中检测到接口，跳过注入。")
        return True

    errors = collect_path_errors(paths)
    if errors:
        print(f"校验失败, {file_path} 中存在 {len(errors)} 个网关不接受的路径, 已跳过写入:")
        for error in errors:
            print(f"  - {error}")
        print("请修改 proto 中 google.api.http 的路径（如 gRPC 自定义动词的冒号）后重新生成。")
        return False

    renamed_count, errors = normalize_operation_ids(paths)
    if errors:
        print(f"校验失败, {file_path} 中存在 {len(errors)} 个不合规的 operationId, 已跳过写入:")
        for error in errors:
            print(f"  - {error}")
        print("同一 rpc 的多个绑定需在 inner、项目、环境维度上可区分，请调整 proto 中的绑定路径；"
              "超长时请缩短 rpc 名或在 RPC_ABBREVIATIONS 中补充缩写。")
        return False

    try:
        with open(file_path, 'w', encoding='utf-8') as f:
            json.dump(swagger_data, f, indent=2, ensure_ascii=False)
        print(f"成功, 已为 {file_path} 注入蓝鲸网关配置: inner={inner_count}, default={default_count}, "
              f"operationId 改名={renamed_count}。")
    except Exception as e:
        print(f"写入文件失败 [{file_path}]: {e}")
        return False
    return True


if __name__ == "__main__":
    # 支持从命令行传入多个文件路径
    if len(sys.argv) < 2:
        print("使用方法: python3 inject_bk_gateway.py <file1.json> <file2.json> ...")
        sys.exit(1)

    if not all([inject_bk_gateway_config(target_file) for target_file in sys.argv[1:]]):
        sys.exit(1)