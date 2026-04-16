package asyncdownload

import (
	"fmt"
	"path"
	"strings"
)

const (
	v2DueBatchesKey = "AsyncBatchDueV2"
)

func buildFileVersionKey(bizID, appID uint32, filePath, fileName, signature string) string {
	return fmt.Sprintf("%d:%d:%s:%s", bizID, appID, path.Join(filePath, fileName), signature)
}

func buildTargetID(agentID, containerID string) string {
	return fmt.Sprintf("%s:%s", agentID, containerID)
}

func buildBatchScopeKey(fileVersionKey, targetUser, targetDir string) string {
	return fmt.Sprintf("%s|%s|%s", fileVersionKey, targetUser, targetDir)
}

func buildInflightTargetKey(targetID, targetUser, targetDir string) string {
	return fmt.Sprintf("%s|%s|%s", targetID, targetUser, targetDir)
}

func parseTargetID(targetID string) (string, string) {
	parts := strings.SplitN(targetID, ":", 2)
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], parts[1]
}

func batchOpenKey(batchScopeKey string) string {
	return fmt.Sprintf("AsyncBatchOpenV2:%s", batchScopeKey)
}

func batchMetaKey(batchID string) string {
	return fmt.Sprintf("AsyncBatchMetaV2:%s", batchID)
}

func batchMetaPattern() string {
	return "AsyncBatchMetaV2:*"
}

func batchTargetsKey(batchID string) string {
	return fmt.Sprintf("AsyncBatchTargetsV2:%s", batchID)
}

func batchTasksKey(batchID string) string {
	return fmt.Sprintf("AsyncBatchTasksV2:%s", batchID)
}

func batchDispatchedTargetsKey(batchID string) string {
	return fmt.Sprintf("AsyncBatchDispatchedTargetsV2:%s", batchID)
}

func inflightKey(fileVersionKey, inflightTargetKey string) string {
	return fmt.Sprintf("AsyncTargetInflightV2:%s:%s", fileVersionKey, inflightTargetKey)
}

func taskMetaKey(taskID string) string {
	return fmt.Sprintf("AsyncTaskMetaV2:%s", taskID)
}
