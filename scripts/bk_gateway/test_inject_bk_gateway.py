#!/usr/bin/env python3
# -*- coding: utf-8 -*-

import unittest

from inject_bk_gateway import normalize_operation_ids


def build_paths(routes):
    paths = {}
    for method, path, operation_id in routes:
        paths.setdefault(path, {})[method] = {"operationId": operation_id}
    return paths


def operation_ids(paths):
    return {(method.upper(), path): config["operationId"]
            for path, item in paths.items() for method, config in item.items()}


class NormalizeOperationIdsTest(unittest.TestCase):

    def test_rename_bindings_by_route_scope(self):
        paths = build_paths([
            ("post", "/api/v1/config/biz/{bizId}/apps/{appId}/kvs/list", "Config_ListKvs"),
            ("post", "/api/v1/inner/config/biz/{bizId}/apps/{appId}/kvs/list", "Config_ListKvs2"),
            ("post", "/api/v1/config/biz/{bizId}/projects/{projectId}/envs/{envId}/apps/{appId}/kvs/list",
             "Config_ListKvs3"),
            ("put", "/api/v1/inner/config/biz/{bizId}/projects/{projectId}/envs/{envId}/apps/{appId}/kvs",
             "Config_ListKvs4"),
            ("get", "/api/v1/config/biz/{bizId}/projects/{projectId}/hooks", "Config_ListKvs5"),
            ("get", "/api/v1/inner/config/biz/{bizId}/projects/{projectId}/envs/query/by_name", "Config_ListKvs6"),
        ])

        renamed, errors = normalize_operation_ids(paths)

        self.assertEqual(errors, [])
        self.assertEqual(renamed, 5)
        self.assertEqual(sorted(operation_ids(paths).values()), sorted([
            "Config_ListKvs",
            "Config_Inner_ListKvs",
            "Config_ProjectEnv_ListKvs",
            "Config_Inner_ProjectEnv_ListKvs",
            "Config_Project_ListKvs",
            "Config_Inner_Project_ListKvs",
        ]))

    def test_keep_primary_and_unrelated_names(self):
        paths = build_paths([
            ("post", "/api/v1/config/biz/{bizId}/projects/{projectId}/envs/list", "Config_ListEnvironments"),
            ("get", "/api/v1/biz/{biz_id}/content/download_url", "get_content_download_url"),
            ("get", "/api/v1/config/biz/{bizId}/oauth2", "Config_GetOAuth2"),
        ])

        renamed, errors = normalize_operation_ids(paths)

        self.assertEqual(errors, [])
        self.assertEqual(renamed, 0)
        self.assertEqual(sorted(operation_ids(paths).values()),
                         ["Config_GetOAuth2", "Config_ListEnvironments", "get_content_download_url"])

    def test_report_conflict_when_bindings_share_scope(self):
        paths = build_paths([
            ("post", "/api/v1/config/create/release/app_id/{appId}/biz_id/{bizId}", "Config_CreateRelease"),
            ("post", "/api/v1/config/biz/{bizId}/apps/{appId}/releases", "Config_CreateRelease2"),
        ])

        _, errors = normalize_operation_ids(paths)

        self.assertEqual(len(errors), 1)
        self.assertIn("Config_CreateRelease", errors[0])


if __name__ == "__main__":
    unittest.main()
