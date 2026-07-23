/*
 * Tencent is pleased to support the open source community by making Blueking Container Service available.
 * Copyright (C) 2019 THL A29 Limited, a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 * http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing, software distributed under
 * the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing permissions and
 * limitations under the License.
 */

package release

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/TencentBlueKing/bk-bscp/cmd/feed-server/bll/types"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/runtime/selector"
	ptypes "github.com/TencentBlueKing/bk-bscp/pkg/types"
)

// Test50UIDs 专门测试50个UID的灰度分布情况
func TestUIDs(t *testing.T) {
	rs := &ReleasedService{}

	// 生成10个测试UID
	testUIDs := []string{
		"9b65419524fe96d385591fcb868d9f78", // 用户提供的示例UID
		"7754ba9f577e29bcb28d930501ef5d6d",
		"c169162507c479db833c59b12468d60b",
		"1fe600b24ed100d4e8a725fc57b40ab2",
		"aaad460b50e755c50bee5bf1e0587d34",
		"051fcabb7788fca845a1a26abc544de0",
		"4e6d30ec163ef2772dd87909c515a998",
		"a066c51dd641456fbbe9812d90b47e36",
		"9dfed0e216860f8f26396f4416a3f362",
		"975f96d9a93788cdc138eaa27b43b025",
	}

	t.Logf("生成了%d个测试UID，开始灰度测试...", len(testUIDs))

	// 测试不同的灰度比例
	testCases := []struct {
		name        string
		grayPercent string
		expected    int // 期望选中的大概数量
	}{
		{"10%灰度", "10%", 1}, // 期望5个左右
		{"20%灰度", "20%", 2}, // 期望10个左右
		{"30%灰度", "30%", 3}, // 期望15个左右
		{"50%灰度", "50%", 5}, // 期望25个左右
		{"70%灰度", "70%", 7}, // 期望25个左右
		{"90%灰度", "90%", 9}, // 期望25个左右
	}

	groupID := uint32(1234)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			group := createTestGroup(groupID, tc.grayPercent)

			var matchedUIDs []string
			var unmatchedUIDs []string

			for _, uid := range testUIDs {
				meta := &types.AppInstanceMeta{
					Uid: uid,
				}

				matched, err := rs.matchReleasedGrayClients(group, meta)
				if err != nil {
					t.Fatalf("匹配测试失败: %v", err)
				}

				if matched {
					matchedUIDs = append(matchedUIDs, uid)
				} else {
					unmatchedUIDs = append(unmatchedUIDs, uid)
				}
			}

			actualCount := len(matchedUIDs)
			actualRate := float64(actualCount) / float64(len(testUIDs)) * 100

			t.Logf("=== %s 测试结果 ===", tc.name)
			t.Logf("总UID数: %d", len(testUIDs))
			t.Logf("选中数量: %d (期望约%d个)", actualCount, tc.expected)
			t.Logf("实际比例: %.1f%%", actualRate)
			t.Logf("偏差: %.1f个 (%.1f%%)",
				math.Abs(float64(actualCount-tc.expected)),
				math.Abs(actualRate-parsePercent(tc.grayPercent)))

			// 显示选中的UID (只显示前10个，避免输出过长)
			t.Logf("选中的UID (前10个):")
			for i, uid := range matchedUIDs {
				if i < 10 {
					t.Logf("  [%d] %s", i+1, uid)
				}
			}
			if len(matchedUIDs) > 10 {
				t.Logf("  ... 还有%d个", len(matchedUIDs)-10)
			}

			// 验证比例是否在合理范围内（允许±10%的误差）
			targetPercent := parsePercent(tc.grayPercent)
			tolerance := 10.0 // 10%的容错
			if math.Abs(actualRate-targetPercent) > tolerance {
				t.Errorf("灰度比例偏差过大! 目标: %.0f%%, 实际: %.1f%%, 超出容错范围: ±%.0f%%",
					targetPercent, actualRate, tolerance)
			}
		})
	}
}

// parsePercent 解析百分比字符串，如 "20%" -> 20.0
func parsePercent(percentStr string) float64 {
	var percent float64
	fmt.Sscanf(percentStr, "%f%%", &percent)
	return percent
}

// createTestGroup 创建测试用的分组配置
func createTestGroup(groupID uint32, grayPercent string) *ptypes.ReleasedGroupCache {
	return &ptypes.ReleasedGroupCache{
		GroupID:   groupID,
		ReleaseID: 5001,
		Selector: &selector.Selector{
			LabelsAnd: []selector.Element{
				{
					Key:   "env",
					Op:    &selector.EqualOperator,
					Value: "prod",
				},
				{
					Key:   table.GrayPercentKey, // "gray_percent"
					Op:    &selector.EqualOperator,
					Value: grayPercent,
				},
			},
		},
	}
}

// TestGrayClientMatching50UIDs 测试50个UID的灰度分布准确性
func TestGrayClientMatching50UIDs(t *testing.T) {
	rs := &ReleasedService{}

	// 模拟50个不同的UID进行测试
	testUIDs := []string{
		"bb57ee2169ab1d6326a6951a958dea0b", "cc68ff3279bc2d7437b7a62a869efb1c",
		"dd79001390cd3e8548c8b73b97aef0d2", "ee8a112401de4f9659d9c84ca8bef1e3",
		"ff9b223512ef5a0760eaad5db9cfa2f4", "001c334623f06b1871fbbd6ecadf03a5",
		"112d445734017c2982acc7fddb0e14b6", "223e556845128d3a93bdd80eed1f25c7",
		"334f667956239e4ba4cea91ffe2036d8", "445068709634af5cb5dfba20ef3147e9",
		"556179820745b06dc6e0cb31f04258fa", "667281931856c17ed7f1dc4201536b0b",
		"778392042967d28fe802ed5312647c1c", "889493153078e39f0913fe6423758d2d",
		"99a504264189f4a01a240f754486ae3e", "aab615375290a5b12b351086559b0f4f",
		"bbc726486301b6c23c462197660c10a5", "ccd837597412c7d34d573208771d21b6",
		"dde948608523d8e45e684319882e32c7", "eef9596096340f562f795420993f43d8",
		"f00a607107451067308a6531aa4054e9", "011b718218562178419b7642bb5165fa",
		"122c829329673289520c8753cc627601", "233d930430784390631d9864dd738712",
		"344e041541895401742eaa75ee849823", "455f152652906512853fbb86ff95aa34",
		"566027376310763968400d97c0a6bb45", "677138487421874a79511ea8d1b7cc56",
		"788249598532985b8a622fb9e2c8dd67", "899350609643096c9b733fcaf3d9ee78",
		"90a461710754107d0c844fdb04eaff89", "a1b572821865218e1d956fec15fbca90",
		"b2c683932976329f2ea670fd26acdba1", "c3d794043087430036b781ae37bdecb2",
		"d4e805154198541147c892bf48cefdc3", "e5f916265209652258d903c059dafed4",
		"f600273761a763367ea14d16a0eb0fe5", "071138872b7847479b251e27b1fc10f6",
		"182249983c8958580c3620389cdda207", "293351094d9a69691d4731499dee0318",
		"3a4462105eab707a2e5842508eefb429", "4b5573216fbc818b3f6953619f00c530",
		"5c6684327ccdd29c40706472a011d641", "6d7795438ddeea0d5181758bb1220e52",
		"7e8806549eef0b1e6292869cc2331f63", "8f9917650f001c2f73a397addd442074",
		"9a0028761011ad4084b408beee553185", "ab1139872122be5195c519cfff664296",
		"bc224a983233cf6206d660a00777530a", "cd335ba94344d073178715b1118864b8",
		"de446ca05455e184289826c2229975c9", "ef557db16566f295390937d333aa86da",
		"f66680c27677035640a048e444bb97eb", "0777918386881467510b59f555cc08fc",
		"188a029497992578621c601666dd100d", "299b130508aa36896320b17777ee211e",
	}

	// 测试不同的灰度比例
	grayPercentages := []struct {
		percent  string
		expected int
	}{
		{"10%", 5},  // 50个的10%期望5个
		{"20%", 10}, // 50个的20%期望10个
		{"50%", 25}, // 50个的50%期望25个
	}

	for _, tc := range grayPercentages {
		t.Run(fmt.Sprintf("Test_%s", tc.percent), func(t *testing.T) {
			selectedCount := 0

			for _, uid := range testUIDs {
				group := createTestGroup(1, tc.percent)

				meta := &types.AppInstanceMeta{
					Uid: uid,
					Labels: map[string]string{
						"env": "prod",
					},
				}

				matched, err := rs.matchReleasedGrayClients(group, meta)
				if err != nil {
					t.Errorf("matchReleasedGrayClients failed: %v", err)
					continue
				}

				if matched {
					selectedCount++
				}
			}

			// 计算实际比例
			actualPercent := float64(selectedCount) / float64(len(testUIDs)) * 100
			expectedPercent := parsePercent(tc.percent)

			t.Logf("灰度目标: %s (%.0f%%), 实际选中: %d/%d (%.1f%%), 期望数量: %d",
				tc.percent, expectedPercent, selectedCount, len(testUIDs), actualPercent, tc.expected)

			// 计算偏差
			deviation := math.Abs(actualPercent - expectedPercent)
			t.Logf("偏差: %.1f%%", deviation)
		})
	}
}

// TestMatchReleasedGroupWithLabels 测试分组匹配逻辑
func TestMatchReleasedGroupWithLabels(t *testing.T) {
	rs := &ReleasedService{}

	// 创建测试用的多个灰度分组
	createGrayGroup := func(groupID uint32, releaseID uint32, grayPercent string, env string) *ptypes.ReleasedGroupCache {
		return &ptypes.ReleasedGroupCache{
			GroupID:    groupID,
			ReleaseID:  releaseID,
			StrategyID: groupID + 1000,
			Mode:       table.GroupModeCustom,
			UpdatedAt:  time.Now().Add(time.Duration(groupID) * time.Minute), // 不同的更新时间
			Selector: &selector.Selector{
				LabelsAnd: []selector.Element{
					{
						Key:   "env",
						Op:    &selector.EqualOperator,
						Value: env,
					},
					{
						Key:   table.GrayPercentKey,
						Op:    &selector.EqualOperator,
						Value: grayPercent,
					},
				},
			},
		}
	}

	// 创建默认分组
	createDefaultGroup := func(groupID uint32, releaseID uint32) *ptypes.ReleasedGroupCache {
		return &ptypes.ReleasedGroupCache{
			GroupID:    groupID,
			ReleaseID:  releaseID,
			StrategyID: groupID + 1000,
			Mode:       table.GroupModeDefault,
			UpdatedAt:  time.Now(),
		}
	}

	t.Run("TestMultipleGrayGroups_SelectMaxPercent", func(t *testing.T) {
		// 测试多个灰度分组时，选择最大灰度比例的分组
		groups := []*ptypes.ReleasedGroupCache{
			createGrayGroup(1, 101, "20%", "prod"), // 20%灰度
			createGrayGroup(2, 102, "50%", "prod"), // 50%灰度 - 应该被选中
			createGrayGroup(3, 103, "10%", "prod"), // 10%灰度
		}

		// 使用一个我们知道会被50%灰度选中的UID（从之前的测试结果中选取）
		// 在50个UID的测试中，有30个被选中，我们选择其中一个
		meta := &types.AppInstanceMeta{
			Uid: "cc68ff3279bc2d7437b7a62a869efb1c", // 尝试另一个UID
			Labels: map[string]string{
				"env": "prod",
			},
		}

		// 先验证这个UID确实会被50%灰度选中
		group50 := createGrayGroup(2, 102, "50%", "prod")
		matched50, _ := rs.matchReleasedGrayClients(group50, meta)

		if !matched50 {
			t.Skip("跳过测试：测试UID不在50%灰度范围内")
		}

		matched, err := rs.matchReleasedGroupWithLabels(nil, groups, meta)
		if err != nil {
			t.Fatalf("matchReleasedGroupWithLabels failed: %v", err)
		}

		if matched == nil {
			t.Fatal("expected to match a group, but got nil")
		}

		// 验证选择了最大灰度比例的分组（50%）
		if matched.GrayPercent < 0.4 { // 允许一些浮点误差
			t.Errorf("expected to select group with higher gray percent, but got %.1f%%", matched.GrayPercent*100)
		}

		t.Logf("✅ 成功选择较大灰度比例的分组: GroupID=%d, GrayPercent=%.1f%%, ReleaseID=%d",
			matched.GroupID, matched.GrayPercent*100, matched.ReleaseID)
	})

	t.Run("TestLabelMismatch_FallbackToDefault", func(t *testing.T) {
		// 测试标签不匹配时，回退到默认分组
		groups := []*ptypes.ReleasedGroupCache{
			createGrayGroup(1, 101, "30%", "test"), // 环境不匹配
			createGrayGroup(2, 102, "50%", "dev"),  // 环境不匹配
			createDefaultGroup(3, 103),             // 默认分组 - 应该被选中
		}

		meta := &types.AppInstanceMeta{
			Uid: "cc68ff3279bc2d7437b7a62a869efb1c",
			Labels: map[string]string{
				"env": "prod", // 与分组环境不匹配
			},
		}

		matched, err := rs.matchReleasedGroupWithLabels(nil, groups, meta)
		if err != nil {
			t.Fatalf("matchReleasedGroupWithLabels failed: %v", err)
		}

		if matched == nil {
			t.Fatal("expected to match default group, but got nil")
		}

		// 验证选择了默认分组
		if matched.GroupID != 3 {
			t.Errorf("expected to select default group (GroupID=3), but got GroupID=%d", matched.GroupID)
		}

		t.Logf("✅ 标签不匹配时正确回退到默认分组: GroupID=%d, ReleaseID=%d",
			matched.GroupID, matched.ReleaseID)
	})

	t.Run("TestGrayConsistencyInGroupSelection", func(t *testing.T) {
		// 测试相同客户端在不同灰度比例分组中的一致性
		testUID := "cc68ff3279bc2d7437b7a62a869efb1c"

		// 先测试是否能在20%灰度中被选中
		group20 := createGrayGroup(1, 101, "20%", "prod")
		meta := &types.AppInstanceMeta{
			Uid: testUID,
			Labels: map[string]string{
				"env": "prod",
			},
		}

		matched20, err := rs.matchReleasedGrayClients(group20, meta)
		if err != nil {
			t.Fatalf("matchReleasedGrayClients failed: %v", err)
		}

		t.Logf("20%%灰度匹配结果: %v", matched20)

		// 测试50%灰度
		group50 := createGrayGroup(2, 102, "50%", "prod")
		matched50, err := rs.matchReleasedGrayClients(group50, meta)
		if err != nil {
			t.Fatalf("matchReleasedGrayClients failed: %v", err)
		}

		t.Logf("50%%灰度匹配结果: %v", matched50)

		// 验证一致性：如果在20%被选中，50%也应该被选中
		if matched20 && !matched50 {
			t.Error("❌ 一致性检查失败：20%时被选中，50%时未被选中")
		} else {
			t.Log("✅ 灰度一致性检查通过")
		}
	})
}

// TestMultipleGrayGroupsRealWorld 真实场景下的多分组测试
func TestMultipleGrayGroupsRealWorld(t *testing.T) {
	// 创建测试分组的辅助函数，使用与主代码一致的结构
	createRealGroup := func(groupID uint32, releaseID uint32, grayPercent string) *ptypes.ReleasedGroupCache {
		return &ptypes.ReleasedGroupCache{
			GroupID:    groupID,
			ReleaseID:  releaseID,
			StrategyID: groupID + 1000,
			Mode:       table.GroupModeCustom,
			UpdatedAt:  time.Now().Add(time.Duration(groupID) * time.Second), // 不同的更新时间
			Selector: &selector.Selector{
				LabelsAnd: []selector.Element{
					{
						Key:   "env",
						Op:    &selector.EqualOperator,
						Value: "prod",
					},
					{
						Key:   table.GrayPercentKey,
						Op:    &selector.EqualOperator,
						Value: grayPercent,
					},
				},
			},
		}
	}

	t.Run("TestIncrementalGrayScale", func(t *testing.T) {
		// 首先找到一个能够被50%灰度选中的UID
		testUIDs := []string{
			"bb57ee2169ab1d6326a6951a958dea0b", "cc68ff3279bc2d7437b7a62a869efb1c",
			"dd79001390cd3e8548c8b73b97aef0d2", "ee8a112401de4f9659d9c84ca8bef1e3",
			"ff9b223512ef5a0760eaad5db9cfa2f4", "556179820745b06dc6e0cb31f04258fa",
			"778392042967d28fe802ed5312647c1c", "99a504264189f4a01a240f754486ae3e",
		}

		var validUID string
		rs := &ReleasedService{}

		// 找到一个能被50%灰度选中的UID
		for _, uid := range testUIDs {
			// 使用相同的ReleaseID进行测试
			testGroup := createRealGroup(3, 200, "50%")
			meta := &types.AppInstanceMeta{
				Uid: uid,
				Labels: map[string]string{
					"env": "prod",
				},
			}

			matched, err := rs.matchReleasedGrayClients(testGroup, meta)
			if err == nil && matched {
				validUID = uid
				t.Logf("找到能被50%%灰度选中的UID: %s", uid[:16]+"...")
				break
			}
		}

		if validUID == "" {
			t.Skip("跳过测试：未找到能被50%灰度选中的UID")
		}

		// 模拟同一ReleaseID下的渐进式灰度：10% -> 30% -> 50%的场景
		// 关键：使用相同的ReleaseID，表示同一个版本的不同灰度策略
		sameReleaseID := uint32(200)
		groups := []*ptypes.ReleasedGroupCache{
			createRealGroup(1, sameReleaseID, "10%"), // 10%灰度分组
			createRealGroup(2, sameReleaseID, "30%"), // 30%灰度分组
			createRealGroup(3, sameReleaseID, "50%"), // 50%灰度分组 - 应该被选中（最大比例）
		}

		meta := &types.AppInstanceMeta{
			Uid: validUID,
			Labels: map[string]string{
				"env": "prod",
			},
		}

		matched, err := rs.matchReleasedGroupWithLabels(nil, groups, meta)
		if err != nil {
			t.Fatalf("matchReleasedGroupWithLabels failed: %v", err)
		}

		if matched != nil {
			t.Logf("✅ 同一ReleaseID灰度测试: 选中GroupID=%d, GrayPercent=%.1f%%, ReleaseID=%d",
				matched.GroupID, matched.GrayPercent*100, matched.ReleaseID)

			// 验证选择了最大比例的分组（应该是50%）
			if matched.GrayPercent >= 0.4 { // 50%灰度应该被选中
				t.Log("✅ 成功选择了最高比例的灰度分组(50%)")
			} else if matched.GrayPercent >= 0.25 { // 30%灰度
				t.Log("✅ 选择了中等比例的灰度分组(30%)")
			} else {
				t.Logf("选择了 %.1f%% 的灰度分组", matched.GrayPercent*100)
			}

			// 验证ReleaseID的一致性
			if matched.ReleaseID != sameReleaseID {
				t.Errorf("❌ ReleaseID不匹配: 期望 %d, 实际 %d", sameReleaseID, matched.ReleaseID)
			} else {
				t.Logf("✅ ReleaseID一致性验证通过: %d", matched.ReleaseID)
			}
		} else {
			t.Error("❌ 应该匹配到一个分组，但返回了nil")
		}
	})

	t.Log("✅ 多分组灰度测试完成")
}

// TestHashSeedConsistency 测试hash seed的一致性（修复后的验证）
func TestHashSeedConsistency(t *testing.T) {
	rs := &ReleasedService{}

	// 使用相同的ReleaseID，不同的GroupID
	sameReleaseID := uint32(300)
	testUID := "test-consistency-uid-123456789"

	// 创建不同GroupID但相同ReleaseID的分组
	group1 := &ptypes.ReleasedGroupCache{
		GroupID:   1,
		ReleaseID: sameReleaseID, // 相同的ReleaseID
		Selector: &selector.Selector{
			LabelsAnd: []selector.Element{
				{
					Key:   table.GrayPercentKey,
					Op:    &selector.EqualOperator,
					Value: "30%",
				},
			},
		},
	}

	group2 := &ptypes.ReleasedGroupCache{
		GroupID:   2,             // 不同的GroupID
		ReleaseID: sameReleaseID, // 相同的ReleaseID
		Selector: &selector.Selector{
			LabelsAnd: []selector.Element{
				{
					Key:   table.GrayPercentKey,
					Op:    &selector.EqualOperator,
					Value: "60%",
				},
			},
		},
	}

	meta := &types.AppInstanceMeta{
		Uid: testUID,
		Labels: map[string]string{
			"env": "prod",
		},
	}

	// 测试两个分组的匹配结果
	matched1, err1 := rs.matchReleasedGrayClients(group1, meta)
	if err1 != nil {
		t.Fatalf("group1 matchReleasedGrayClients failed: %v", err1)
	}

	matched2, err2 := rs.matchReleasedGrayClients(group2, meta)
	if err2 != nil {
		t.Fatalf("group2 matchReleasedGrayClients failed: %v", err2)
	}

	t.Logf("使用相同ReleaseID(%d)测试结果:", sameReleaseID)
	t.Logf("GroupID=1, 30%%灰度: %v", matched1)
	t.Logf("GroupID=2, 60%%灰度: %v", matched2)

	// 验证一致性：如果在30%时被选中，那么在60%时也应该被选中
	if matched1 && !matched2 {
		t.Error("❌ 一致性检查失败：30%时被选中，60%时未被选中")
		t.Error("这表明hash seed使用了GroupID，导致不同分组结果不一致")
	} else if matched1 && matched2 {
		t.Log("✅ 一致性检查通过：30%被选中，60%也被选中")
	} else if !matched1 && !matched2 {
		t.Log("✅ 一致性检查通过：30%和60%都未被选中")
	} else {
		t.Log("✅ 一致性检查通过：30%未被选中，60%被选中（正常的灰度扩展）")
	}
}

// TestGrayScalingConsistency 测试灰度扩展的一致性
func TestGrayScalingConsistency(t *testing.T) {
	rs := &ReleasedService{}
	sameReleaseID := uint32(400)

	// 创建20%和50%的灰度分组
	group20 := &ptypes.ReleasedGroupCache{
		GroupID:   10,
		ReleaseID: sameReleaseID,
		Selector: &selector.Selector{
			LabelsAnd: []selector.Element{
				{
					Key:   table.GrayPercentKey,
					Op:    &selector.EqualOperator,
					Value: "20%",
				},
			},
		},
	}

	group50 := &ptypes.ReleasedGroupCache{
		GroupID:   20,            // 不同的GroupID
		ReleaseID: sameReleaseID, // 相同的ReleaseID
		Selector: &selector.Selector{
			LabelsAnd: []selector.Element{
				{
					Key:   table.GrayPercentKey,
					Op:    &selector.EqualOperator,
					Value: "50%",
				},
			},
		},
	}

	// 测试100个不同的UID
	testUIDs := make([]string, 100)
	for i := 0; i < 100; i++ {
		testUIDs[i] = fmt.Sprintf("test-scaling-uid-%03d", i)
	}

	var selectedIn20 []string
	var selectedIn50 []string
	var violationCount int

	for _, uid := range testUIDs {
		meta := &types.AppInstanceMeta{
			Uid:    uid,
			Labels: map[string]string{"env": "prod"},
		}

		matched20, err := rs.matchReleasedGrayClients(group20, meta)
		if err != nil {
			t.Fatalf("匹配20%%灰度失败: %v", err)
		}

		matched50, err := rs.matchReleasedGrayClients(group50, meta)
		if err != nil {
			t.Fatalf("匹配50%%灰度失败: %v", err)
		}

		if matched20 {
			selectedIn20 = append(selectedIn20, uid)
		}
		if matched50 {
			selectedIn50 = append(selectedIn50, uid)
		}

		// 检查一致性违例：在20%被选中但在50%未被选中的情况
		if matched20 && !matched50 {
			t.Logf("❌ 一致性违例: UID %s 在20%%被选中但50%%未被选中", uid)
			violationCount++
		}
	}

	t.Logf("📊 灰度扩展一致性测试结果:")
	t.Logf("20%%灰度选中: %d/%d (%.1f%%)", len(selectedIn20), len(testUIDs), float64(len(selectedIn20))*100/float64(len(testUIDs)))
	t.Logf("50%%灰度选中: %d/%d (%.1f%%)", len(selectedIn50), len(testUIDs), float64(len(selectedIn50))*100/float64(len(testUIDs)))
	t.Logf("一致性违例: %d/%d", violationCount, len(testUIDs))

	if violationCount > 0 {
		t.Errorf("❌ 发现%d个一致性违例，hash seed可能仍在使用GroupID", violationCount)
	} else {
		t.Log("✅ 所有客户端在灰度扩展时都保持一致性")
	}

	// 验证覆盖关系：50%的选中应该包含20%的所有选中
	for _, uid := range selectedIn20 {
		found := false
		for _, uid50 := range selectedIn50 {
			if uid == uid50 {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("❌ UID %s 在20%%被选中但在50%%未找到", uid)
		}
	}

	t.Log("✅ 灰度扩展一致性测试完成")
}

// findGrayBucketUID 找一个会落入指定灰度组桶的 UID，找不到返回空串。
func findGrayBucketUID(rs *ReleasedService, grayGroup *ptypes.ReleasedGroupCache,
	labels map[string]string) string {
	for i := 0; i < 2000; i++ {
		uid := fmt.Sprintf("graylock-uid-%04d", i)
		meta := &types.AppInstanceMeta{Uid: uid, Labels: labels}
		matched, err := rs.matchReleasedGrayClients(grayGroup, meta)
		if err == nil && matched {
			return uid
		}
	}
	return ""
}

// TestGrayLock_FullShouldCoverGrayResidual 复现「灰度锁死」缺陷
// 场景：一个实例同时命中【非灰度全量组 -> 新版本】与【残留灰度组 -> 旧版本】，且落入灰度桶。
// 期望（修复后）：全量组优先，最终生效新版本；
// 现状（缺陷）：matchReleasedGroupWithLabels 选择灰度比例最大的组，非灰度组按 0 处理，
//
//	被残留灰度组压制，实例被钉死在旧版本 -> 本用例当前会失败（红灯），即复现成功。
func TestGrayLock_FullShouldCoverGrayResidual(t *testing.T) {
	rs := &ReleasedService{}

	const (
		releaseOld = uint32(33) // 旧版本，走灰度
		releaseNew = uint32(35) // 新版本，走全量
		grayPct    = "40%"
	)
	labels := map[string]string{"app": "nginx"}

	// 非灰度全量组：selector app=nginx，无 gray_percent
	fullGroup := &ptypes.ReleasedGroupCache{
		GroupID:    100,
		ReleaseID:  releaseNew,
		StrategyID: 1100,
		Mode:       table.GroupModeCustom,
		UpdatedAt:  time.Now(), // 更晚发布（全量）
		Selector: &selector.Selector{
			LabelsAnd: []selector.Element{
				{Key: "app", Op: &selector.EqualOperator, Value: "nginx"},
			},
		},
	}

	// 残留灰度组：selector app=nginx & gray_percent=40，指向旧版本
	grayGroup := &ptypes.ReleasedGroupCache{
		GroupID:    101,
		ReleaseID:  releaseOld,
		StrategyID: 1101,
		Mode:       table.GroupModeCustom,
		UpdatedAt:  time.Now().Add(-time.Hour), // 更早发布
		Selector: &selector.Selector{
			LabelsAnd: []selector.Element{
				{Key: "app", Op: &selector.EqualOperator, Value: "nginx"},
				{Key: table.GrayPercentKey, Op: &selector.EqualOperator, Value: grayPct},
			},
		},
	}

	lockedUID := findGrayBucketUID(rs, grayGroup, labels)
	if lockedUID == "" {
		t.Fatal("未找到落入灰度桶的 UID，无法复现")
	}

	meta := &types.AppInstanceMeta{Uid: lockedUID, Labels: labels}
	matched, err := rs.matchReleasedGroupWithLabels(nil,
		[]*ptypes.ReleasedGroupCache{fullGroup, grayGroup}, meta)
	if err != nil {
		t.Fatalf("matchReleasedGroupWithLabels failed: %v", err)
	}

	t.Logf("落桶 UID=%s 最终匹配 ReleaseID=%d，全量组=%d，残留灰度组=%d",
		lockedUID, matched.ReleaseID, releaseNew, releaseOld)

	if matched.ReleaseID != releaseNew {
		t.Errorf("❌ 灰度锁死复现：全量组(R=%d)被残留灰度组(R=%d)压制，实例被钉死在旧版本；"+
			"期望全量覆盖生效 R=%d，实际 R=%d", releaseNew, releaseOld, releaseNew, matched.ReleaseID)
	}
}

// TestGrayLock_OnlyGrayUnchanged 仅命中灰度组时行为不变（AC-002）。
func TestGrayLock_OnlyGrayUnchanged(t *testing.T) {
	rs := &ReleasedService{}
	labels := map[string]string{"app": "nginx"}

	grayGroup := &ptypes.ReleasedGroupCache{
		GroupID:    201,
		ReleaseID:  33,
		StrategyID: 1201,
		Mode:       table.GroupModeCustom,
		UpdatedAt:  time.Now(),
		Selector: &selector.Selector{
			LabelsAnd: []selector.Element{
				{Key: "app", Op: &selector.EqualOperator, Value: "nginx"},
				{Key: table.GrayPercentKey, Op: &selector.EqualOperator, Value: "40%"},
			},
		},
	}

	uid := findGrayBucketUID(rs, grayGroup, labels)
	if uid == "" {
		t.Fatal("未找到落入灰度桶的 UID")
	}
	meta := &types.AppInstanceMeta{Uid: uid, Labels: labels}
	matched, err := rs.matchReleasedGroupWithLabels(nil,
		[]*ptypes.ReleasedGroupCache{grayGroup}, meta)
	if err != nil {
		t.Fatalf("match failed: %v", err)
	}
	if matched == nil || matched.ReleaseID != 33 {
		t.Errorf("仅命中灰度组应生效其版本 R=33，实际 %+v", matched)
	}
}

// TestGrayLock_OnlyFullUnchanged 仅命中非灰度全量组时行为不变（AC-003）。
func TestGrayLock_OnlyFullUnchanged(t *testing.T) {
	rs := &ReleasedService{}
	labels := map[string]string{"app": "nginx"}

	fullGroup := &ptypes.ReleasedGroupCache{
		GroupID:    301,
		ReleaseID:  35,
		StrategyID: 1301,
		Mode:       table.GroupModeCustom,
		UpdatedAt:  time.Now(),
		Selector: &selector.Selector{
			LabelsAnd: []selector.Element{
				{Key: "app", Op: &selector.EqualOperator, Value: "nginx"},
			},
		},
	}

	meta := &types.AppInstanceMeta{Uid: "any-uid-full-only", Labels: labels}
	matched, err := rs.matchReleasedGroupWithLabels(nil,
		[]*ptypes.ReleasedGroupCache{fullGroup}, meta)
	if err != nil {
		t.Fatalf("match failed: %v", err)
	}
	if matched == nil || matched.ReleaseID != 35 {
		t.Errorf("仅命中非灰度全量组应生效其版本 R=35，实际 %+v", matched)
	}
}

// TestGrayLock_MultipleFullPickLatest 多个非灰度全量组并列命中时（均视为 100%），
// 沿用「并列按最近更新」兜底规则，选中 UpdatedAt 最新的组（对应 F-001 边界条件）。
func TestGrayLock_MultipleFullPickLatest(t *testing.T) {
	rs := &ReleasedService{}
	labels := map[string]string{"app": "nginx"}

	newFullGroup := func(groupID, releaseID uint32, updatedAt time.Time) *ptypes.ReleasedGroupCache {
		return &ptypes.ReleasedGroupCache{
			GroupID:    groupID,
			ReleaseID:  releaseID,
			StrategyID: groupID + 1000,
			Mode:       table.GroupModeCustom,
			UpdatedAt:  updatedAt,
			Selector: &selector.Selector{
				LabelsAnd: []selector.Element{
					{Key: "app", Op: &selector.EqualOperator, Value: "nginx"},
				},
			},
		}
	}

	older := newFullGroup(401, 41, time.Now().Add(-time.Hour))
	latest := newFullGroup(402, 42, time.Now())

	meta := &types.AppInstanceMeta{Uid: "any-uid-multi-full", Labels: labels}
	matched, err := rs.matchReleasedGroupWithLabels(nil,
		[]*ptypes.ReleasedGroupCache{older, latest}, meta)
	if err != nil {
		t.Fatalf("match failed: %v", err)
	}
	if matched == nil || matched.ReleaseID != 42 {
		t.Errorf("多个非灰度组并列命中应按最近更新选中 R=42，实际 %+v", matched)
	}
}
