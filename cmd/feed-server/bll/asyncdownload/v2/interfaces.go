// * Tencent is pleased to support the open source community by making Blueking Container Service available.
//  * Copyright (C) 2019 THL A29 Limited, a Tencent company. All rights reserved.
//  * Licensed under the MIT License (the "License"); you may not use this file except
//  * in compliance with the License. You may obtain a copy of the License at
//  * http://opensource.org/licenses/MIT
//  * Unless required by applicable law or agreed to in writing, software distributed under
//  * the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
//  * either express or implied. See the License for the specific language governing permissions and
//  * limitations under the License.

// * Tencent is pleased to support the open source community by making Blueking Container Service available.
//  * Copyright (C) 20\d\d THL A29 Limited, a Tencent company. All rights reserved.
//  * Licensed under the MIT License (the "License"); you may not use this file except
//  * in compliance with the License. You may obtain a copy of the License at
//  * http://opensource.org/licenses/MIT
//  * Unless required by applicable law or agreed to in writing, software distributed under
//  * the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
//  * either express or implied. See the License for the specific language governing permissions and
//  * limitations under the License.

package v2

import (
	"context"
	"io"
	"time"

	"github.com/TencentBlueKing/bk-bscp/cmd/feed-server/bll/types"
	"github.com/TencentBlueKing/bk-bscp/internal/components/gse"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
)

type TransferFileClient interface {
	AsyncExtensionsTransferFile(ctx context.Context, req *gse.TransferFileReq) (*gse.CommonTaskRespData, error)
	AsyncTerminateTransferFile(ctx context.Context, req *gse.TerminateTransferFileTaskReq) (*gse.CommonTaskRespData, error)
	GetExtensionsTransferFileResult(ctx context.Context, req *gse.GetTransferFileResultReq) (*gse.TransferFileResultData, error)
}

type SourceDownloader interface {
	Download(kt *kit.Kit, sign string) (io.ReadCloser, int64, error)
}

type Metrics interface {
	ObserveV2BatchCreated(batch *types.AsyncDownloadV2Batch)
	ObserveV2BatchTransition(batch *types.AsyncDownloadV2Batch, oldState string)
	ObserveV2TaskCreated(task *types.AsyncDownloadV2Task)
	ObserveV2TaskTransition(task *types.AsyncDownloadV2Task, oldState string, oldUpdatedAt time.Time)
	SetV2DueBacklog(count int)
	SetV2OldestDueAgeSeconds(age float64)
	IncV2TaskRepair(reason string)
	ObserveV2ShardDispatch(status string, duration time.Duration)
}
