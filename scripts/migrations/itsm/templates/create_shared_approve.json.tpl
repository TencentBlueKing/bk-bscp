{
	"is_deleted": false,
	"name": "\u521b\u5efa\u4e0a\u7ebf\u5ba1\u6279",
	"desc": "",
	"flow_type": "other",
	"is_enabled": true,
	"is_revocable": true,
	"revoke_config": {
		"type": 1,
		"state": 0
	},
	"is_draft": false,
	"is_builtin": false,
	"is_task_needed": false,
	"owners": "",
	"notify_rule": "ONCE",
	"notify_freq": 0,
	"is_biz_needed": false,
	"is_auto_approve": false,
	"is_iam_used": false,
	"is_supervise_needed": true,
	"supervise_type": "EMPTY",
	"supervisor": "",
	"engine_version": "PIPELINE_V1",
	"version_number": "20241115155847",
	"table": {
		"id": 36,
		"is_deleted": false,
		"name": "\u9ed8\u8ba4_20240319192628",
		"desc": "\u9ed8\u8ba4\u57fa\u7840\u6a21\u578b",
		"version": "EMPTY",
		"fields": [{
				"id": 1,
				"is_deleted": false,
				"is_builtin": true,
				"is_readonly": false,
				"is_valid": true,
				"display": true,
				"source_type": "CUSTOM",
				"source_uri": "",
				"api_instance_id": 0,
				"kv_relation": {},
				"type": "STRING",
				"key": "title",
				"name": "\u6807\u9898",
				"layout": "COL_12",
				"validate_type": "REQUIRE",
				"show_type": 1,
				"show_conditions": {},
				"regex": "EMPTY",
				"regex_config": {},
				"custom_regex": "",
				"desc": "\u8bf7\u8f93\u5165\u6807\u9898",
				"tips": "",
				"is_tips": false,
				"default": "",
				"choice": [],
				"related_fields": {},
				"meta": {},
				"flow_type": "DEFAULT",
				"project_key": "public",
				"source": "BASE-MODEL"
			},
			{
				"id": 2,
				"is_deleted": false,
				"is_builtin": true,
				"is_readonly": false,
				"is_valid": true,
				"display": true,
				"source_type": "DATADICT",
				"source_uri": "IMPACT",
				"api_instance_id": 0,
				"kv_relation": {},
				"type": "SELECT",
				"key": "impact",
				"name": "\u5f71\u54cd\u8303\u56f4",
				"layout": "COL_12",
				"validate_type": "REQUIRE",
				"show_type": 1,
				"show_conditions": {},
				"regex": "EMPTY",
				"regex_config": {},
				"custom_regex": "",
				"desc": "\u8bf7\u9009\u62e9\u5f71\u54cd\u8303\u56f4",
				"tips": "",
				"is_tips": false,
				"default": "",
				"choice": [],
				"related_fields": {},
				"meta": {},
				"flow_type": "DEFAULT",
				"project_key": "public",
				"source": "BASE-MODEL"
			},
			{
				"id": 3,
				"is_deleted": false,
				"is_builtin": true,
				"is_readonly": false,
				"is_valid": true,
				"display": true,
				"source_type": "DATADICT",
				"source_uri": "URGENCY",
				"api_instance_id": 0,
				"kv_relation": {},
				"type": "SELECT",
				"key": "urgency",
				"name": "\u7d27\u6025\u7a0b\u5ea6",
				"layout": "COL_12",
				"validate_type": "REQUIRE",
				"show_type": 1,
				"show_conditions": {},
				"regex": "EMPTY",
				"regex_config": {},
				"custom_regex": "",
				"desc": "\u8bf7\u9009\u62e9\u7d27\u6025\u7a0b\u5ea6",
				"tips": "",
				"is_tips": false,
				"default": "",
				"choice": [],
				"related_fields": {},
				"meta": {},
				"flow_type": "DEFAULT",
				"project_key": "public",
				"source": "BASE-MODEL"
			},
			{
				"id": 4,
				"is_deleted": false,
				"is_builtin": true,
				"is_readonly": true,
				"is_valid": true,
				"display": true,
				"source_type": "DATADICT",
				"source_uri": "PRIORITY",
				"api_instance_id": 0,
				"kv_relation": {},
				"type": "SELECT",
				"key": "priority",
				"name": "\u4f18\u5148\u7ea7",
				"layout": "COL_12",
				"validate_type": "REQUIRE",
				"show_type": 1,
				"show_conditions": {},
				"regex": "EMPTY",
				"regex_config": {},
				"custom_regex": "",
				"desc": "\u8bf7\u9009\u62e9\u4f18\u5148\u7ea7",
				"tips": "",
				"is_tips": false,
				"default": "",
				"choice": [],
				"related_fields": {
					"rely_on": [
						"urgency",
						"impact"
					]
				},
				"meta": {},
				"flow_type": "DEFAULT",
				"project_key": "public",
				"source": "BASE-MODEL"
			},
			{
				"id": 5,
				"is_deleted": false,
				"is_builtin": true,
				"is_readonly": false,
				"is_valid": true,
				"display": true,
				"source_type": "RPC",
				"source_uri": "ticket_status",
				"api_instance_id": 0,
				"kv_relation": {},
				"type": "SELECT",
				"key": "current_status",
				"name": "\u5de5\u5355\u72b6\u6001",
				"layout": "COL_12",
				"validate_type": "REQUIRE",
				"show_type": 1,
				"show_conditions": {},
				"regex": "EMPTY",
				"regex_config": {},
				"custom_regex": "",
				"desc": "\u8bf7\u9009\u62e9\u5de5\u5355\u72b6\u6001",
				"tips": "",
				"is_tips": false,
				"default": "",
				"choice": [],
				"related_fields": {},
				"meta": {},
				"flow_type": "DEFAULT",
				"project_key": "public",
				"source": "BASE-MODEL"
			}
		],
		"fields_order": [
			1,
			2,
			3,
			4,
			5
		],
		"field_key_order": [
			"title",
			"impact",
			"urgency",
			"priority",
			"current_status"
		]
	},
	"task_schemas": [],
	"creator": "",
	"updated_by": "",
	"workflow_id": 159,
	"version_message": "",
	"states": {
		"947": {
			"workflow": 159,
			"id": 947,
			"key": 947,
			"name": "\u5f00\u59cb",
			"desc": "",
			"distribute_type": "PROCESS",
			"axis": {
				"x": 150,
				"y": 150
			},
			"is_builtin": true,
			"variables": {
				"inputs": [],
				"outputs": []
			},
			"tag": "DEFAULT",
			"processors_type": "OPEN",
			"processors": "",
			"assignors": "",
			"assignors_type": "EMPTY",
			"delivers": "",
			"delivers_type": "EMPTY",
			"can_deliver": false,
			"extras": {},
			"is_draft": false,
			"is_terminable": false,
			"fields": [],
			"type": "START",
			"api_instance_id": 0,
			"is_sequential": false,
			"finish_condition": {},
			"is_multi": false,
			"is_allow_skip": false,
			"creator": null,
			"create_at": "2024-11-06 15:24:00",
			"updated_by": null,
			"update_at": "2024-11-06 15:24:00",
			"end_at": null,
			"is_first_state": false
		},
		"948": {
			"workflow": 159,
			"id": 948,
			"key": 948,
			"name": "\u63d0\u5355",
			"desc": "",
			"distribute_type": "PROCESS",
			"axis": {
				"x": 285,
				"y": 150
			},
			"is_builtin": true,
			"variables": {
				"inputs": [],
				"outputs": [{
						"key": "CLUSTER_TYPE",
						"source": "field",
						"state": 2948,
						"type": "SELECT"
					},
					{
						"key": "CLUSTER_ID",
						"source": "field",
						"state": 2582,
						"type": "STRING"
					},
					{
						"key": "CPU_LIMITS",
						"source": "field",
						"state": 2718,
						"type": "INT"
					},
					{
						"key": "MEMORY_LIMITS",
						"source": "field",
						"state": 2718,
						"type": "INT"
					},
					{
						"key": "APPROVE_TYPE",
						"source": "field",
						"state": 889,
						"type": "STRING"
					}
				]
			},
			"tag": "DEFAULT",
			"processors_type": "OPEN",
			"processors": "",
			"assignors": "",
			"assignors_type": "EMPTY",
			"delivers": "",
			"delivers_type": "EMPTY",
			"can_deliver": false,
			"extras": {
				"ticket_status": {
					"name": "",
					"type": "keep"
				}
			},
			"is_draft": false,
			"is_terminable": false,
			"fields": [
				1778,
				1792,
				1783,
				1782,
				1793,
				1784,
				1785,
				1797,
				1786,
				1787,
				1788
			],
			"type": "NORMAL",
			"api_instance_id": 0,
			"is_sequential": false,
			"finish_condition": {},
			"is_multi": false,
			"is_allow_skip": false,
			"creator": null,
			"create_at": "2024-11-06 15:24:00",
			"updated_by": "admin",
			"update_at": "2024-11-15 15:06:12",
			"end_at": null,
			"is_first_state": true
		},
		"949": {
			"workflow": 159,
			"id": 949,
			"key": 949,
			"name": "\u7ed3\u675f",
			"desc": "",
			"distribute_type": "PROCESS",
			"axis": {
				"x": 1575,
				"y": 145
			},
			"is_builtin": true,
			"variables": {
				"inputs": [],
				"outputs": []
			},
			"tag": "DEFAULT",
			"processors_type": "OPEN",
			"processors": "",
			"assignors": "",
			"assignors_type": "EMPTY",
			"delivers": "",
			"delivers_type": "EMPTY",
			"can_deliver": false,
			"extras": {},
			"is_draft": false,
			"is_terminable": false,
			"fields": [],
			"type": "END",
			"api_instance_id": 0,
			"is_sequential": false,
			"finish_condition": {},
			"is_multi": false,
			"is_allow_skip": false,
			"creator": null,
			"create_at": "2024-11-06 15:24:00",
			"updated_by": null,
			"update_at": "2024-11-06 15:24:00",
			"end_at": null,
			"is_first_state": false
		},
		"950": {
			"workflow": 159,
			"id": 950,
			"key": 950,
			"name": "\u4f1a\u7b7e\u5ba1\u6279",
			"desc": "",
			"distribute_type": "PROCESS",
			"axis": {
				"x": 730,
				"y": 105
			},
			"is_builtin": false,
			"variables": {
				"inputs": [],
				"outputs": [{
						"key": "Fd6380d03621747689b9776224da468d",
						"meta": {
							"choice": [{
									"key": "false",
									"name": "\u62d2\u7edd"
								},
								{
									"key": "true",
									"name": "\u901a\u8fc7"
								}
							],
							"code": "NODE_APPROVE_RESULT",
							"type": "SELECT"
						},
						"name": "\u5ba1\u6279\u7ed3\u679c",
						"source": "global",
						"state": 2956,
						"type": "STRING"
					},
					{
						"key": "O1af1a6c7fceb2bbe9243d0cfd871028",
						"meta": {
							"code": "NODE_APPROVER"
						},
						"name": "\u5ba1\u6279\u4eba",
						"source": "global",
						"state": 2956,
						"type": "STRING"
					},
					{
						"key": "dd93d6c0341ce48260408a2964448cb7",
						"meta": {
							"code": "PROCESS_COUNT"
						},
						"name": "\u5904\u7406\u4eba\u6570",
						"source": "global",
						"state": 2956,
						"type": "INT"
					},
					{
						"key": "c6619ac6399ebb6f4208406add9d971e",
						"meta": {
							"code": "PASS_COUNT"
						},
						"name": "\u901a\u8fc7\u4eba\u6570",
						"source": "global",
						"state": 2956,
						"type": "INT"
					},
					{
						"key": "l76a275fc8b01ceeb9a33f77ddb03679",
						"meta": {
							"code": "REJECT_COUNT"
						},
						"name": "\u62d2\u7edd\u4eba\u6570",
						"source": "global",
						"state": 2956,
						"type": "INT"
					},
					{
						"key": "f73b972755824685ca4cc7edd0a0bdab",
						"meta": {
							"code": "PASS_RATE",
							"unit": "PERCENT"
						},
						"name": "\u901a\u8fc7\u7387",
						"source": "global",
						"state": 2956,
						"type": "INT"
					},
					{
						"key": "e987844249bd935b6e2b0b2609da593f",
						"meta": {
							"code": "REJECT_RATE",
							"unit": "PERCENT"
						},
						"name": "\u62d2\u7edd\u7387",
						"source": "global",
						"state": 2956,
						"type": "INT"
					}
				]
			},
			"tag": "DEFAULT",
			"processors_type": "PERSON",
			"processors": "admin",
			"assignors": "",
			"assignors_type": "EMPTY",
			"delivers": "admin",
			"delivers_type": "PERSON",
			"can_deliver": false,
			"extras": {
				"enable_terminate_ticket_when_rejected": false,
				"ticket_status": {
					"name": "",
					"type": "keep"
				}
			},
			"is_draft": false,
			"is_terminable": false,
			"fields": [
				1779,
				1780,
				1781
			],
			"type": "APPROVAL",
			"api_instance_id": 0,
			"is_sequential": false,
			"finish_condition": {
				"expressions": [{
					"expressions": [{
						"condition": ">=",
						"key": "l76a275fc8b01ceeb9a33f77ddb03679",
						"meta": {
							"code": "",
							"unit": "INT"
						},
						"source": "global",
						"tooltipInfo": {
							"content": "\u8bf7\u5148\u9009\u62e9\u5904\u7406\u4eba",
							"disabled": false,
							"placements": [
								"top"
							]
						},
						"type": "INT",
						"value": "1"
					}],
					"type": "and"
				}],
				"type": "or"
			},
			"is_multi": true,
			"is_allow_skip": true,
			"creator": null,
			"create_at": "2024-11-06 15:24:00",
			"updated_by": null,
			"update_at": "2024-11-06 15:24:00",
			"end_at": null,
			"is_first_state": false
		},
		"951": {
			"workflow": 159,
			"id": 951,
			"key": 951,
			"name": "\u6210\u529f\u56de\u8c03",
			"desc": "",
			"distribute_type": "PROCESS",
			"axis": {
				"x": 1180,
				"y": -70
			},
			"is_builtin": false,
			"variables": {
				"inputs": [],
				"outputs": []
			},
			"tag": "DEFAULT",
			"processors_type": "PERSON",
			"processors": "admin",
			"assignors": "",
			"assignors_type": "EMPTY",
			"delivers": "",
			"delivers_type": "EMPTY",
			"can_deliver": false,
			"extras": {
				"webhook_info": {
					"auth": {
						"auth_config": {
							"token": ""
						},
						"auth_type": "bearer_token"
					},
					"body": {
						"content": "{\n    \"title\": \"{{ticket_title}}\",\n    \"currentStatus\": \"{{ticket_current_status}}\",\n    \"sn\": \"{{ticket_sn}}\",\n    \"ticketUrl\": \"{{ticket_ticket_url}}\",\n    \"applyInCluster\": true,\n    \"approveResult\": true,\n    \"publish_status\": \"pending_publish\"\n}",
						"raw_type": "JSON",
						"type": "raw"
					},
					"headers": [{
							"check": true,
							"desc": "",
							"key": "X-Bkapi-Authorization",
							"select": true,
							"value": "{\"bk_app_code\": \"[[.BkAppCode]]\", \"bk_app_secret\": \"[[.BkAppSecret]]\"}"
						},
						{
							"check": true,
							"desc": "",
							"key": "X-Bkapi-User-Name",
							"select": true,
							"value": "admin"
						}
					],
					"method": "POST",
					"query_params": [],
					"settings": {
						"timeout": 10
					},
					"success_exp": "resp.data.code==0",
					"url": "[[.BCSPGateway]]/api/v1/config/biz_id/{{BIZ_ID}}/app_id/{{APP_ID}}/release_id/{{RELEASE_ID}}/approve"
				}
			},
			"is_draft": false,
			"is_terminable": false,
			"fields": [],
			"type": "WEBHOOK",
			"api_instance_id": 0,
			"is_sequential": false,
			"finish_condition": {},
			"is_multi": false,
			"is_allow_skip": false,
			"creator": null,
			"create_at": "2024-11-06 15:24:00",
			"updated_by": null,
			"update_at": "2024-11-06 15:24:00",
			"end_at": null,
			"is_first_state": false
		},
		"952": {
			"workflow": 159,
			"id": 952,
			"key": 952,
			"name": "\u5931\u8d25\u56de\u8c03",
			"desc": "",
			"distribute_type": "PROCESS",
			"axis": {
				"x": 1180,
				"y": 375
			},
			"is_builtin": false,
			"variables": {
				"outputs": [],
				"inputs": []
			},
			"tag": "DEFAULT",
			"processors_type": "PERSON",
			"processors": "admin",
			"assignors": "",
			"assignors_type": "EMPTY",
			"delivers": "",
			"delivers_type": "EMPTY",
			"can_deliver": false,
			"extras": {
				"webhook_info": {
					"method": "POST",
					"url": "[[.BCSPGateway]]/api/v1/config/biz_id/{{BIZ_ID}}/app_id/{{APP_ID}}/release_id/{{RELEASE_ID}}/approve",
					"query_params": [],
					"auth": {
						"auth_type": "bearer_token",
						"auth_config": {
							"token": ""
						}
					},
					"headers": [{
							"check": true,
							"desc": "",
							"key": "X-Bkapi-Authorization",
							"select": true,
							"value": "{\"bk_app_code\": \"[[.BkAppCode]]\", \"bk_app_secret\": \"[[.BkAppSecret]]\"}"
						},
						{
							"check": true,
							"desc": "",
							"key": "X-Bkapi-User-Name",
							"select": true,
							"value": "admin"
						}
					],
					"body": {
						"type": "raw",
						"raw_type": "JSON",
						"content": "{\n    \"title\": \"{{ticket_title}}\",\n    \"currentStatus\": \"{{ticket_current_status}}\",\n    \"sn\": \"{{ticket_sn}}\",\n    \"ticketUrl\": \"{{ticket_ticket_url}}\",\n    \"applyInCluster\": false,\n    \"approveResult\": false,\n    \"publish_status\": \"rejected_approval\"\n}"
					},
					"settings": {
						"timeout": 10
					},
					"success_exp": "resp.data.code==0"
				}
			},
			"is_draft": false,
			"is_terminable": false,
			"fields": [],
			"type": "WEBHOOK",
			"api_instance_id": 0,
			"is_sequential": false,
			"finish_condition": {},
			"is_multi": false,
			"is_allow_skip": false,
			"creator": null,
			"create_at": "2024-11-06 15:24:00",
			"updated_by": "admin",
			"update_at": "2024-11-13 14:58:40",
			"end_at": null,
			"is_first_state": false
		},
		"953": {
			"workflow": 159,
			"id": 953,
			"key": 953,
			"name": "\u6216\u7b7e\u5ba1\u6279",
			"desc": "",
			"distribute_type": "PROCESS",
			"axis": {
				"x": 585,
				"y": 240
			},
			"is_builtin": false,
			"variables": {
				"inputs": [],
				"outputs": [{
						"key": "5f0bbc10ccca4626fb49e6b3c143355b",
						"source": "field",
						"state": 807,
						"type": "RADIO"
					},
					{
						"key": "213847b79048f0b29f568ccdc37d2ec2",
						"source": "field",
						"state": 807,
						"type": "TEXT"
					},
					{
						"key": "282880f10d08c09c14f8b68a3908aa76",
						"source": "field",
						"state": 807,
						"type": "TEXT"
					},
					{
						"key": "dfbd6a168b83ed22b09421e4c8af2592",
						"meta": {
							"choice": [{
									"key": "false",
									"name": "\u62d2\u7edd"
								},
								{
									"key": "true",
									"name": "\u901a\u8fc7"
								}
							],
							"code": "NODE_APPROVE_RESULT",
							"type": "SELECT"
						},
						"name": "\u5ba1\u6279\u7ed3\u679c",
						"source": "global",
						"state": 807,
						"type": "STRING"
					},
					{
						"key": "cec0d4db7e9cb593ee6a52cedc13e9e3",
						"meta": {
							"code": "NODE_APPROVER"
						},
						"name": "\u5ba1\u6279\u4eba",
						"source": "global",
						"state": 807,
						"type": "STRING"
					},
					{
						"key": "E85493757eee799a4a4625ea3fb3964f",
						"meta": {
							"code": "PROCESS_COUNT"
						},
						"name": "\u5904\u7406\u4eba\u6570",
						"source": "global",
						"state": 807,
						"type": "INT"
					},
					{
						"key": "o2b4afe5cf09e05e4c9407073337c83f",
						"meta": {
							"code": "PASS_COUNT"
						},
						"name": "\u901a\u8fc7\u4eba\u6570",
						"source": "global",
						"state": 807,
						"type": "INT"
					},
					{
						"key": "U0c6889a997dbee7c8bb4088cafaf98c",
						"meta": {
							"code": "REJECT_COUNT"
						},
						"name": "\u62d2\u7edd\u4eba\u6570",
						"source": "global",
						"state": 807,
						"type": "INT"
					},
					{
						"key": "n7079b14306afa765a7166fa1032a286",
						"meta": {
							"code": "PASS_RATE",
							"unit": "PERCENT"
						},
						"name": "\u901a\u8fc7\u7387",
						"source": "global",
						"state": 807,
						"type": "INT"
					},
					{
						"key": "w57a184055cc89e26e1a073a68489c4a",
						"meta": {
							"code": "REJECT_RATE",
							"unit": "PERCENT"
						},
						"name": "\u62d2\u7edd\u7387",
						"source": "global",
						"state": 807,
						"type": "INT"
					}
				]
			},
			"tag": "DEFAULT",
			"processors_type": "PERSON",
			"processors": "admin",
			"assignors": "",
			"assignors_type": "EMPTY",
			"delivers": "admin",
			"delivers_type": "PERSON",
			"can_deliver": false,
			"extras": {
				"enable_terminate_ticket_when_rejected": false,
				"ticket_status": {
					"name": "",
					"type": "keep"
				}
			},
			"is_draft": false,
			"is_terminable": false,
			"fields": [
				1789,
				1790,
				1791
			],
			"type": "APPROVAL",
			"api_instance_id": 0,
			"is_sequential": false,
			"finish_condition": {
				"expressions": [{
					"expressions": [{
						"condition": ">=",
						"key": "l76a275fc8b01ceeb9a33f77ddb03679",
						"meta": {
							"code": "",
							"unit": "INT"
						},
						"source": "global",
						"tooltipInfo": {
							"content": "\u8bf7\u5148\u9009\u62e9\u5904\u7406\u4eba",
							"disabled": false,
							"placements": [
								"top"
							]
						},
						"type": "INT",
						"value": "1"
					}],
					"type": "and"
				}],
				"type": "or"
			},
			"is_multi": false,
			"is_allow_skip": true,
			"creator": null,
			"create_at": "2024-11-06 15:24:00",
			"updated_by": null,
			"update_at": "2024-11-06 15:24:00",
			"end_at": null,
			"is_first_state": false
		}
	},
	"transitions": {
		"976": {
			"workflow": 159,
			"id": 976,
			"from_state": 947,
			"to_state": 948,
			"name": "",
			"axis": {
				"start": "Right",
				"end": "Left"
			},
			"condition": {
				"expressions": [{
					"expressions": [{
						"condition": "==",
						"key": "G_INT_1",
						"value": 1
					}],
					"type": "and"
				}],
				"type": "and"
			},
			"condition_type": "default",
			"creator": null,
			"create_at": "2024-11-06 15:24:00",
			"updated_by": null,
			"update_at": "2024-11-06 15:24:00",
			"end_at": null
		},
		"977": {
			"workflow": 159,
			"id": 977,
			"from_state": 950,
			"to_state": 951,
			"name": "\u5ba1\u6279\u901a\u8fc7",
			"axis": {
				"start": "Right",
				"end": "Left"
			},
			"condition": {
				"expressions": [{
					"checkInfo": false,
					"expressions": [{
						"choiceList": [],
						"condition": "==",
						"key": "Fd6380d03621747689b9776224da468d",
						"meta": {
							"choice": [{
									"key": "false",
									"name": "\u62d2\u7edd"
								},
								{
									"key": "true",
									"name": "\u901a\u8fc7"
								}
							],
							"code": "NODE_APPROVE_RESULT",
							"type": "SELECT"
						},
						"source": "field",
						"type": "SELECT",
						"value": "true"
					}],
					"type": "and"
				}],
				"type": "and"
			},
			"condition_type": "by_field",
			"creator": null,
			"create_at": "2024-11-06 15:24:00",
			"updated_by": null,
			"update_at": "2024-11-06 15:24:00",
			"end_at": null
		},
		"978": {
			"workflow": 159,
			"id": 978,
			"from_state": 950,
			"to_state": 952,
			"name": "\u5ba1\u6279\u9a73\u56de",
			"axis": {
				"start": "Right",
				"end": "Left"
			},
			"condition": {
				"expressions": [{
					"checkInfo": false,
					"expressions": [{
						"choiceList": [],
						"condition": ">=",
						"key": "l76a275fc8b01ceeb9a33f77ddb03679",
						"meta": {
							"code": "REJECT_COUNT"
						},
						"source": "field",
						"type": "INT",
						"value": 1
					}],
					"type": "and"
				}],
				"type": "and"
			},
			"condition_type": "by_field",
			"creator": null,
			"create_at": "2024-11-06 15:24:00",
			"updated_by": null,
			"update_at": "2024-11-06 15:24:00",
			"end_at": null
		},
		"979": {
			"workflow": 159,
			"id": 979,
			"from_state": 951,
			"to_state": 949,
			"name": "\u6d41\u7a0b\u7ed3\u675f",
			"axis": {
				"start": "Right",
				"end": "Top"
			},
			"condition": {
				"expressions": [{
					"expressions": [{
						"condition": "==",
						"key": "G_INT_1",
						"value": 1
					}],
					"type": "and"
				}],
				"type": "and"
			},
			"condition_type": "default",
			"creator": null,
			"create_at": "2024-11-06 15:24:00",
			"updated_by": null,
			"update_at": "2024-11-06 15:24:00",
			"end_at": null
		},
		"980": {
			"workflow": 159,
			"id": 980,
			"from_state": 952,
			"to_state": 949,
			"name": "\u6d41\u7a0b\u7ed3\u675f",
			"axis": {
				"start": "Right",
				"end": "Bottom"
			},
			"condition": {
				"expressions": [{
					"expressions": [{
						"condition": "==",
						"key": "G_INT_1",
						"value": 1
					}],
					"type": "and"
				}],
				"type": "and"
			},
			"condition_type": "default",
			"creator": null,
			"create_at": "2024-11-06 15:24:00",
			"updated_by": null,
			"update_at": "2024-11-06 15:24:00",
			"end_at": null
		},
		"981": {
			"workflow": 159,
			"id": 981,
			"from_state": 948,
			"to_state": 950,
			"name": "\u4f1a\u7b7e",
			"axis": {
				"start": "Right",
				"end": "Left"
			},
			"condition": {
				"expressions": [{
					"checkInfo": false,
					"expressions": [{
						"choiceList": [],
						"condition": "==",
						"key": "APPROVE_TYPE",
						"meta": {},
						"source": "field",
						"type": "STRING",
						"value": "\u4f1a\u7b7e"
					}],
					"type": "and"
				}],
				"type": "and"
			},
			"condition_type": "by_field",
			"creator": null,
			"create_at": "2024-11-06 15:24:00",
			"updated_by": null,
			"update_at": "2024-11-06 15:24:00",
			"end_at": null
		},
		"982": {
			"workflow": 159,
			"id": 982,
			"from_state": 948,
			"to_state": 953,
			"name": "\u6216\u7b7e",
			"axis": {
				"start": "Right",
				"end": "Left"
			},
			"condition": {
				"expressions": [{
					"checkInfo": false,
					"expressions": [{
						"choiceList": [],
						"condition": "==",
						"key": "APPROVE_TYPE",
						"meta": {},
						"source": "field",
						"type": "STRING",
						"value": "\u6216\u7b7e"
					}],
					"type": "and"
				}],
				"type": "and"
			},
			"condition_type": "by_field",
			"creator": null,
			"create_at": "2024-11-06 15:24:00",
			"updated_by": null,
			"update_at": "2024-11-06 15:24:00",
			"end_at": null
		},
		"983": {
			"workflow": 159,
			"id": 983,
			"from_state": 953,
			"to_state": 951,
			"name": "\u5ba1\u6279\u901a\u8fc7",
			"axis": {
				"start": "Right",
				"end": "Left"
			},
			"condition": {
				"expressions": [{
					"checkInfo": false,
					"expressions": [{
						"choiceList": [],
						"condition": "==",
						"key": "dfbd6a168b83ed22b09421e4c8af2592",
						"meta": {
							"choice": [{
									"key": "false",
									"name": "\u62d2\u7edd"
								},
								{
									"key": "true",
									"name": "\u901a\u8fc7"
								}
							],
							"code": "NODE_APPROVE_RESULT",
							"type": "SELECT"
						},
						"source": "field",
						"type": "SELECT",
						"value": "true"
					}],
					"type": "and"
				}],
				"type": "and"
			},
			"condition_type": "by_field",
			"creator": null,
			"create_at": "2024-11-06 15:24:00",
			"updated_by": null,
			"update_at": "2024-11-06 15:24:00",
			"end_at": null
		},
		"984": {
			"workflow": 159,
			"id": 984,
			"from_state": 953,
			"to_state": 952,
			"name": "\u5ba1\u6279\u9a73\u56de",
			"axis": {
				"start": "Right",
				"end": "Left"
			},
			"condition": {
				"expressions": [{
					"checkInfo": false,
					"expressions": [{
						"choiceList": [],
						"condition": "==",
						"key": "dfbd6a168b83ed22b09421e4c8af2592",
						"meta": {
							"choice": [{
									"key": "false",
									"name": "\u62d2\u7edd"
								},
								{
									"key": "true",
									"name": "\u901a\u8fc7"
								}
							],
							"code": "NODE_APPROVE_RESULT",
							"type": "SELECT"
						},
						"source": "field",
						"type": "SELECT",
						"value": "false"
					}],
					"type": "and"
				}],
				"type": "and"
			},
			"condition_type": "by_field",
			"creator": null,
			"create_at": "2024-11-06 15:24:00",
			"updated_by": null,
			"update_at": "2024-11-06 15:24:00",
			"end_at": null
		}
	},
	"triggers": [{
		"rules": [{
			"name": "",
			"condition": "",
			"by_condition": false,
			"action_schemas": [{
				"id": 354,
				"creator": "",
				"updated_by": "",
				"is_deleted": false,
				"name": "",
				"display_name": "",
				"component_type": "automatic_announcement",
				"operate_type": "BACKEND",
				"delay_params": {
					"type": "custom",
					"value": 0
				},
				"can_repeat": false,
				"params": [{
						"key": "web_hook_id",
						"ref_type": "custom",
						"value": "BCS_CREATE_NAMESPACE_TICKET"
					},
					{
						"key": "chat_id",
						"ref_type": "custom",
						"value": ""
					},
					{
						"key": "content",
						"ref_type": "custom",
						"value": "\u60a8\u6709\u4e00\u6761\u5355\u636e\u5f85\u5904\u7406"
					},
					{
						"key": "mentioned_list",
						"ref_type": "import",
						"value": "${ticket_current_processors}"
					}
				],
				"inputs": {}
			}]
		}],
		"id": 354,
		"creator": "",
		"updated_by": "",
		"is_deleted": false,
		"name": "\u4f01\u5fae\u901a\u77e5",
		"desc": "",
		"signal": "THROUGH_TRANSITION",
		"sender": "3002",
		"inputs": [],
		"source_type": "workflow",
		"source_id": 159,
		"source_table_id": 0,
		"is_draft": false,
		"is_enabled": true,
		"icon": "message",
		"project_key": "alkaid-test"
	}],
	"fields": {
		"1778": {
			"id": 1778,
			"is_deleted": false,
			"is_builtin": true,
			"is_readonly": false,
			"is_valid": true,
			"display": true,
			"source_type": "CUSTOM",
			"source_uri": "",
			"api_instance_id": 0,
			"kv_relation": {},
			"type": "STRING",
			"key": "title",
			"name": "\u6807\u9898",
			"layout": "COL_12",
			"validate_type": "REQUIRE",
			"show_type": 1,
			"show_conditions": {},
			"regex": "EMPTY",
			"regex_config": {},
			"custom_regex": "",
			"desc": "\u8bf7\u8f93\u5165\u6807\u9898",
			"tips": "",
			"is_tips": false,
			"default": "",
			"choice": [],
			"related_fields": {},
			"meta": {},
			"workflow_id": 159,
			"state_id": "",
			"source": "TABLE"
		},
		"1779": {
			"id": 1779,
			"is_deleted": false,
			"is_builtin": false,
			"is_readonly": false,
			"is_valid": true,
			"display": true,
			"source_type": "CUSTOM",
			"source_uri": "",
			"api_instance_id": 0,
			"kv_relation": {},
			"type": "RADIO",
			"key": "bfaba606fe9be5d6596270a00c87d428",
			"name": "\u5ba1\u6279\u610f\u89c1",
			"layout": "COL_6",
			"validate_type": "REQUIRE",
			"show_type": 1,
			"show_conditions": {},
			"regex": "EMPTY",
			"regex_config": {},
			"custom_regex": "",
			"desc": "",
			"tips": "",
			"is_tips": false,
			"default": "true",
			"choice": [{
					"key": "true",
					"name": "\u901a\u8fc7"
				},
				{
					"key": "false",
					"name": "\u62d2\u7edd"
				}
			],
			"related_fields": {},
			"meta": {
				"code": "APPROVE_RESULT"
			},
			"workflow_id": 159,
			"state_id": 950,
			"source": "CUSTOM"
		},
		"1780": {
			"id": 1780,
			"is_deleted": false,
			"is_builtin": false,
			"is_readonly": false,
			"is_valid": true,
			"display": false,
			"source_type": "CUSTOM",
			"source_uri": "",
			"api_instance_id": 0,
			"kv_relation": {},
			"type": "TEXT",
			"key": "ff9e6f2b83c5ea1c47f36e10310980c3",
			"name": "\u5907\u6ce8",
			"layout": "COL_12",
			"validate_type": "OPTION",
			"show_type": 0,
			"show_conditions": {
				"expressions": [{
					"condition": "==",
					"key": "bfaba606fe9be5d6596270a00c87d428",
					"type": "RADIO",
					"value": "false"
				}],
				"type": "and"
			},
			"regex": "EMPTY",
			"regex_config": {},
			"custom_regex": "",
			"desc": "",
			"tips": "",
			"is_tips": false,
			"default": "",
			"choice": [],
			"related_fields": {},
			"meta": {},
			"workflow_id": 159,
			"state_id": 950,
			"source": "CUSTOM"
		},
		"1781": {
			"id": 1781,
			"is_deleted": false,
			"is_builtin": false,
			"is_readonly": false,
			"is_valid": true,
			"display": false,
			"source_type": "CUSTOM",
			"source_uri": "",
			"api_instance_id": 0,
			"kv_relation": {},
			"type": "TEXT",
			"key": "I60e9046a05cdff0951ee0acf07d4db8",
			"name": "\u5907\u6ce8",
			"layout": "COL_12",
			"validate_type": "REQUIRE",
			"show_type": 0,
			"show_conditions": {
				"expressions": [{
					"condition": "==",
					"key": "bfaba606fe9be5d6596270a00c87d428",
					"type": "RADIO",
					"value": "true"
				}],
				"type": "and"
			},
			"regex": "EMPTY",
			"regex_config": {},
			"custom_regex": "",
			"desc": "",
			"tips": "",
			"is_tips": false,
			"default": "",
			"choice": [],
			"related_fields": {},
			"meta": {},
			"workflow_id": 159,
			"state_id": 950,
			"source": "CUSTOM"
		},
		"1782": {
			"id": 1782,
			"is_deleted": false,
			"is_builtin": false,
			"is_readonly": false,
			"is_valid": true,
			"display": false,
			"source_type": "CUSTOM",
			"source_uri": "",
			"api_instance_id": 0,
			"kv_relation": {},
			"type": "STRING",
			"key": "APP",
			"name": "\u670d\u52a1\u540d\u79f0",
			"layout": "COL_12",
			"validate_type": "REQUIRE",
			"show_type": 1,
			"show_conditions": {},
			"regex": "EMPTY",
			"regex_config": {
				"rule": {
					"expressions": [{
						"condition": "",
						"key": "",
						"source": "field",
						"type": "",
						"value": ""
					}],
					"type": "and"
				}
			},
			"custom_regex": "",
			"desc": "",
			"tips": "",
			"is_tips": false,
			"default": "",
			"choice": [],
			"related_fields": {},
			"meta": {},
			"workflow_id": 159,
			"state_id": 948,
			"source": "CUSTOM"
		},
		"1783": {
			"id": 1783,
			"is_deleted": false,
			"is_builtin": false,
			"is_readonly": false,
			"is_valid": true,
			"display": false,
			"source_type": "CUSTOM",
			"source_uri": "",
			"api_instance_id": 0,
			"kv_relation": {},
			"type": "STRING",
			"key": "BIZ",
			"name": "\u4e1a\u52a1",
			"layout": "COL_12",
			"validate_type": "REQUIRE",
			"show_type": 1,
			"show_conditions": {},
			"regex": "EMPTY",
			"regex_config": {
				"rule": {
					"expressions": [{
						"condition": "",
						"key": "",
						"source": "field",
						"type": "",
						"value": ""
					}],
					"type": "and"
				}
			},
			"custom_regex": "",
			"desc": "",
			"tips": "",
			"is_tips": false,
			"default": "",
			"choice": [],
			"related_fields": {},
			"meta": {},
			"workflow_id": 159,
			"state_id": 948,
			"source": "CUSTOM"
		},
		"1784": {
			"id": 1784,
			"is_deleted": false,
			"is_builtin": false,
			"is_readonly": false,
			"is_valid": true,
			"display": false,
			"source_type": "CUSTOM",
			"source_uri": "",
			"api_instance_id": 0,
			"kv_relation": {},
			"type": "STRING",
			"key": "SCOPE",
			"name": "\u4e0a\u7ebf\u8303\u56f4",
			"layout": "COL_12",
			"validate_type": "REQUIRE",
			"show_type": 1,
			"show_conditions": {},
			"regex": "EMPTY",
			"regex_config": {
				"rule": {
					"expressions": [{
						"condition": "",
						"key": "",
						"source": "field",
						"type": "",
						"value": ""
					}],
					"type": "and"
				}
			},
			"custom_regex": "",
			"desc": "",
			"tips": "",
			"is_tips": false,
			"default": "",
			"choice": [],
			"related_fields": {},
			"meta": {},
			"workflow_id": 159,
			"state_id": 948,
			"source": "CUSTOM"
		},
		"1785": {
			"id": 1785,
			"is_deleted": false,
			"is_builtin": false,
			"is_readonly": false,
			"is_valid": true,
			"display": false,
			"source_type": "CUSTOM",
			"source_uri": "",
			"api_instance_id": 0,
			"kv_relation": {},
			"type": "LINK",
			"key": "COMPARE",
			"name": "\u7248\u672c\u5dee\u5f02\u5bf9\u6bd4",
			"layout": "COL_12",
			"validate_type": "REQUIRE",
			"show_type": 1,
			"show_conditions": {},
			"regex": "EMPTY",
			"regex_config": {
				"rule": {
					"expressions": [{
						"condition": "",
						"key": "",
						"source": "field",
						"type": "",
						"value": ""
					}],
					"type": "and"
				}
			},
			"custom_regex": "",
			"desc": "",
			"tips": "",
			"is_tips": false,
			"default": "",
			"choice": [],
			"related_fields": {},
			"meta": {},
			"workflow_id": 159,
			"state_id": 948,
			"source": "CUSTOM"
		},
		"1786": {
			"id": 1786,
			"is_deleted": false,
			"is_builtin": false,
			"is_readonly": false,
			"is_valid": true,
			"display": false,
			"source_type": "CUSTOM",
			"source_uri": "",
			"api_instance_id": 0,
			"kv_relation": {},
			"type": "STRING",
			"key": "BIZ_ID",
			"name": "\u4e1a\u52a1ID",
			"layout": "COL_12",
			"validate_type": "REQUIRE",
			"show_type": 0,
			"show_conditions": {
				"expressions": [{
						"condition": "==",
						"key": "APPROVE_TYPE",
						"type": "STRING",
						"value": "\u4f1a\u7b7e"
					},
					{
						"condition": "==",
						"key": "APPROVE_TYPE",
						"type": "STRING",
						"value": "\u6216\u7b7e"
					}
				],
				"type": "or"
			},
			"regex": "EMPTY",
			"regex_config": {
				"rule": {
					"expressions": [{
						"condition": "",
						"key": "",
						"source": "field",
						"type": "",
						"value": ""
					}],
					"type": "and"
				}
			},
			"custom_regex": "",
			"desc": "",
			"tips": "",
			"is_tips": false,
			"default": "",
			"choice": [],
			"related_fields": {},
			"meta": {},
			"workflow_id": 159,
			"state_id": 948,
			"source": "CUSTOM"
		},
		"1787": {
			"id": 1787,
			"is_deleted": false,
			"is_builtin": false,
			"is_readonly": false,
			"is_valid": true,
			"display": false,
			"source_type": "CUSTOM",
			"source_uri": "",
			"api_instance_id": 0,
			"kv_relation": {},
			"type": "STRING",
			"key": "RELEASE_ID",
			"name": "\u7248\u672cID",
			"layout": "COL_12",
			"validate_type": "REQUIRE",
			"show_type": 0,
			"show_conditions": {
				"expressions": [{
						"condition": "==",
						"key": "APPROVE_TYPE",
						"type": "STRING",
						"value": "\u4f1a\u7b7e"
					},
					{
						"condition": "==",
						"key": "APPROVE_TYPE",
						"type": "STRING",
						"value": "\u6216\u7b7e"
					}
				],
				"type": "or"
			},
			"regex": "EMPTY",
			"regex_config": {
				"rule": {
					"expressions": [{
						"condition": "",
						"key": "",
						"source": "field",
						"type": "",
						"value": ""
					}],
					"type": "and"
				}
			},
			"custom_regex": "",
			"desc": "",
			"tips": "",
			"is_tips": false,
			"default": "",
			"choice": [],
			"related_fields": {},
			"meta": {},
			"workflow_id": 159,
			"state_id": 948,
			"source": "CUSTOM"
		},
		"1788": {
			"id": 1788,
			"is_deleted": false,
			"is_builtin": false,
			"is_readonly": false,
			"is_valid": true,
			"display": false,
			"source_type": "CUSTOM",
			"source_uri": "",
			"api_instance_id": 0,
			"kv_relation": {},
			"type": "STRING",
			"key": "APP_ID",
			"name": "\u670d\u52a1ID",
			"layout": "COL_12",
			"validate_type": "REQUIRE",
			"show_type": 0,
			"show_conditions": {
				"expressions": [{
						"condition": "==",
						"key": "APPROVE_TYPE",
						"type": "STRING",
						"value": "\u6216\u7b7e"
					},
					{
						"condition": "==",
						"key": "APPROVE_TYPE",
						"type": "STRING",
						"value": "\u4f1a\u7b7e"
					}
				],
				"type": "or"
			},
			"regex": "EMPTY",
			"regex_config": {
				"rule": {
					"expressions": [{
						"condition": "",
						"key": "",
						"source": "field",
						"type": "",
						"value": ""
					}],
					"type": "and"
				}
			},
			"custom_regex": "",
			"desc": "",
			"tips": "",
			"is_tips": false,
			"default": "",
			"choice": [],
			"related_fields": {},
			"meta": {},
			"workflow_id": 159,
			"state_id": 948,
			"source": "CUSTOM"
		},
		"1789": {
			"id": 1789,
			"is_deleted": false,
			"is_builtin": false,
			"is_readonly": false,
			"is_valid": true,
			"display": true,
			"source_type": "CUSTOM",
			"source_uri": "",
			"api_instance_id": 0,
			"kv_relation": {},
			"type": "RADIO",
			"key": "u1c601ecba8dcf52920b8f3aeef4f239",
			"name": "\u5ba1\u6279\u610f\u89c1",
			"layout": "COL_6",
			"validate_type": "REQUIRE",
			"show_type": 1,
			"show_conditions": {},
			"regex": "EMPTY",
			"regex_config": {},
			"custom_regex": "",
			"desc": "",
			"tips": "",
			"is_tips": false,
			"default": "true",
			"choice": [{
					"key": "true",
					"name": "\u901a\u8fc7"
				},
				{
					"key": "false",
					"name": "\u62d2\u7edd"
				}
			],
			"related_fields": {},
			"meta": {
				"code": "APPROVE_RESULT"
			},
			"workflow_id": 159,
			"state_id": 953,
			"source": "CUSTOM"
		},
		"1790": {
			"id": 1790,
			"is_deleted": false,
			"is_builtin": false,
			"is_readonly": false,
			"is_valid": true,
			"display": false,
			"source_type": "CUSTOM",
			"source_uri": "",
			"api_instance_id": 0,
			"kv_relation": {},
			"type": "TEXT",
			"key": "s04cfaf42f2668a854db73b103ba43b9",
			"name": "\u5907\u6ce8",
			"layout": "COL_12",
			"validate_type": "OPTION",
			"show_type": 0,
			"show_conditions": {
				"expressions": [{
					"condition": "==",
					"key": "u1c601ecba8dcf52920b8f3aeef4f239",
					"type": "RADIO",
					"value": "false"
				}],
				"type": "and"
			},
			"regex": "EMPTY",
			"regex_config": {},
			"custom_regex": "",
			"desc": "",
			"tips": "",
			"is_tips": false,
			"default": "",
			"choice": [],
			"related_fields": {},
			"meta": {},
			"workflow_id": 159,
			"state_id": 953,
			"source": "CUSTOM"
		},
		"1791": {
			"id": 1791,
			"is_deleted": false,
			"is_builtin": false,
			"is_readonly": false,
			"is_valid": true,
			"display": false,
			"source_type": "CUSTOM",
			"source_uri": "",
			"api_instance_id": 0,
			"kv_relation": {},
			"type": "TEXT",
			"key": "Gd402c99592f97a23254fffa5a71d80c",
			"name": "\u5907\u6ce8",
			"layout": "COL_12",
			"validate_type": "REQUIRE",
			"show_type": 0,
			"show_conditions": {
				"expressions": [{
					"condition": "==",
					"key": "u1c601ecba8dcf52920b8f3aeef4f239",
					"type": "RADIO",
					"value": "true"
				}],
				"type": "and"
			},
			"regex": "EMPTY",
			"regex_config": {},
			"custom_regex": "",
			"desc": "",
			"tips": "",
			"is_tips": false,
			"default": "",
			"choice": [],
			"related_fields": {},
			"meta": {},
			"workflow_id": 159,
			"state_id": 953,
			"source": "CUSTOM"
		},
		"1792": {
			"id": 1792,
			"is_deleted": false,
			"is_builtin": false,
			"is_readonly": false,
			"is_valid": true,
			"display": false,
			"source_type": "CUSTOM",
			"source_uri": "",
			"api_instance_id": 0,
			"kv_relation": {},
			"type": "STRING",
			"key": "APPROVE_TYPE",
			"name": "\u5ba1\u6279\u65b9\u5f0f",
			"layout": "COL_12",
			"validate_type": "REQUIRE",
			"show_type": 1,
			"show_conditions": {},
			"regex": "EMPTY",
			"regex_config": {
				"rule": {
					"expressions": [{
						"condition": "",
						"key": "",
						"source": "field",
						"type": "",
						"value": ""
					}],
					"type": "and"
				}
			},
			"custom_regex": "",
			"desc": "",
			"tips": "",
			"is_tips": false,
			"default": "",
			"choice": [],
			"related_fields": {},
			"meta": {},
			"workflow_id": 159,
			"state_id": 948,
			"source": "CUSTOM"
		},
		"1793": {
			"id": 1793,
			"is_deleted": false,
			"is_builtin": false,
			"is_readonly": false,
			"is_valid": true,
			"display": false,
			"source_type": "CUSTOM",
			"source_uri": "",
			"api_instance_id": 0,
			"kv_relation": {},
			"type": "STRING",
			"key": "RELEASE_NAME",
			"name": "\u4e0a\u7ebf\u7248\u672c\u540d\u79f0",
			"layout": "COL_12",
			"validate_type": "REQUIRE",
			"show_type": 1,
			"show_conditions": {},
			"regex": "EMPTY",
			"regex_config": {
				"rule": {
					"expressions": [{
						"condition": "",
						"key": "",
						"source": "field",
						"type": "",
						"value": ""
					}],
					"type": "and"
				}
			},
			"custom_regex": "",
			"desc": "",
			"tips": "",
			"is_tips": false,
			"default": "",
			"choice": [],
			"related_fields": {},
			"meta": {},
			"workflow_id": 159,
			"state_id": 948,
			"source": "CUSTOM"
		},
		"1797": {
			"id": 1797,
			"is_deleted": false,
			"is_builtin": false,
			"is_readonly": false,
			"is_valid": true,
			"display": false,
			"source_type": "CUSTOM",
			"source_uri": "",
			"api_instance_id": 0,
			"kv_relation": {},
			"type": "STRING",
			"key": "MEMO",
			"name": "\u4e0a\u7ebf\u8bf4\u660e",
			"layout": "COL_12",
			"validate_type": "OPTION",
			"show_type": 1,
			"show_conditions": {},
			"regex": "EMPTY",
			"regex_config": {
				"rule": {
					"expressions": [{
						"condition": "",
						"key": "",
						"source": "field",
						"type": "",
						"value": ""
					}],
					"type": "and"
				}
			},
			"custom_regex": "",
			"desc": "",
			"tips": "",
			"is_tips": false,
			"default": "",
			"choice": [],
			"related_fields": {},
			"meta": {},
			"workflow_id": 159,
			"state_id": 948,
			"source": "CUSTOM"
		}
	},
	"notify": [],
	"extras": {
		"biz_related": false,
		"need_urge": false,
		"urgers_type": "EMPTY",
		"urgers": "",
		"task_settings": []
	}
}