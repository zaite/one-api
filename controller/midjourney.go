// Author: Calcium-Ion
// GitHub: https://github.com/Calcium-Ion/new-api
// Path: controller/midjourney.go
package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"one-api/common"
	"one-api/common/logger"
	"one-api/model"
	"one-api/providers"
	provider "one-api/providers/midjourney"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

var (
	taskActive int32 = 0
	lock       sync.Mutex
	cond       = sync.NewCond(&lock)
)

func InitMidjourneyTask() {
	common.SafeGoroutine(func() {
		midjourneyTask()
	})

	ActivateUpdateMidjourneyTaskBulk()
}

func midjourneyTask() {
	for {
		lock.Lock()
		for atomic.LoadInt32(&taskActive) == 0 {
			cond.Wait() // 等待激活信号
		}
		lock.Unlock()
		UpdateMidjourneyTaskBulk()
	}
}

func ActivateUpdateMidjourneyTaskBulk() {
	if atomic.LoadInt32(&taskActive) == 1 {
		return
	}

	lock.Lock()
	atomic.StoreInt32(&taskActive, 1)
	cond.Signal() // 通知等待的任务
	lock.Unlock()
}

func DeactivateMidjourneyTaskBulk() {
	if atomic.LoadInt32(&taskActive) == 0 {
		return
	}

	lock.Lock()
	atomic.StoreInt32(&taskActive, 0)
	lock.Unlock()
}

func UpdateMidjourneyTaskBulk() {
	ctx := context.WithValue(context.Background(), logger.RequestIdKey, "MidjourneyTask")
	for {
		logger.LogInfo(ctx, "running")

		tasks := model.GetAllUnFinishTasks()

		// 如果没有未完成的任务，则等待
		if len(tasks) == 0 {
			DeactivateMidjourneyTaskBulk()
			logger.LogInfo(ctx, "no tasks, waiting...")
			return
		}

		logger.LogWarn(ctx, fmt.Sprintf("检测到未完成的任务数有: %v", len(tasks)))
		taskChannelM := make(map[int][]string)
		taskM := make(map[string]*model.Midjourney)
		nullTaskIds := make([]int, 0)
		for _, task := range tasks {
			if task.MjId == "" {
				// 统计失败的未完成任务
				nullTaskIds = append(nullTaskIds, task.Id)
				continue
			}
			taskM[task.MjId] = task
			taskChannelM[task.ChannelId] = append(taskChannelM[task.ChannelId], task.MjId)
		}
		if len(nullTaskIds) > 0 {
			err := model.MjBulkUpdateByTaskIds(nullTaskIds, map[string]any{
				"status":   "FAILURE",
				"progress": "100%",
			})
			if err != nil {
				logger.LogError(ctx, fmt.Sprintf("Fix null mj_id task error: %v", err))
			} else {
				logger.LogInfo(ctx, fmt.Sprintf("Fix null mj_id task success: %v", nullTaskIds))
			}
		}
		if len(taskChannelM) == 0 {
			continue
		}

		for channelId, taskIds := range taskChannelM {
			logger.LogWarn(ctx, fmt.Sprintf("渠道 #%d 未完成的任务有: %d", channelId, len(taskIds)))
			if len(taskIds) == 0 {
				continue
			}
			midjourneyChannel := model.ChannelGroup.GetChannel(channelId)
			if midjourneyChannel == nil {
				err := model.MjBulkUpdate(taskIds, map[string]any{
					"fail_reason": fmt.Sprintf("获取渠道信息失败，请联系管理员，渠道ID：%d", channelId),
					"status":      "FAILURE",
					"progress":    "100%",
				})
				logger.LogError(ctx, fmt.Sprintf("UpdateMidjourneyTask error: %v", err))
				continue
			}

			providers := providers.GetProvider(midjourneyChannel, nil)
			mjProvider, ok := providers.(*provider.MidjourneyProvider)
			if !ok {
				err := model.MjBulkUpdate(taskIds, map[string]any{
					"fail_reason": fmt.Sprintf("获取供应商失败，请联系管理员，渠道ID：%d", channelId),
					"status":      "FAILURE",
					"progress":    "100%",
				})
				logger.LogError(ctx, fmt.Sprintf("get task error: provider not found, error: %v", err))
				continue
			}

			err := MjTaskHandler(ctx, mjProvider, taskIds, taskM)
			if err != nil {
				logger.LogError(ctx, fmt.Sprintf("MjTaskHandler error: %v", err))
			}
		}
		time.Sleep(time.Duration(15) * time.Second)
	}
}

func MjTaskHandler(ctx context.Context, mjProvider *provider.MidjourneyProvider, taskIds []string, taskM map[string]*model.Midjourney) error {

	responseItems, err := mjProvider.GetTaskListByCondition(taskIds)
	if err != nil {
		return fmt.Errorf("get task error: %v", err)
	}

	for _, responseItem := range responseItems {
		task := taskM[responseItem.MjId]

		useTime := (time.Now().UnixNano() / int64(time.Millisecond)) - task.SubmitTime
		// 如果时间超过一小时，且进度不是100%，则认为任务失败
		if useTime > 3600000 && task.Progress != "100%" {
			responseItem.FailReason = "上游任务超时（超过1小时）"
			responseItem.Status = "FAILURE"
		}
		if !checkMjTaskNeedUpdate(task, responseItem) {
			continue
		}
		task.Code = 1
		task.Progress = responseItem.Progress
		task.PromptEn = responseItem.PromptEn
		task.State = responseItem.State
		task.SubmitTime = responseItem.SubmitTime
		task.StartTime = responseItem.StartTime
		task.FinishTime = responseItem.FinishTime
		task.ImageUrl = responseItem.ImageUrl
		task.Status = responseItem.Status
		task.FailReason = responseItem.FailReason
		if responseItem.Properties != nil {
			propertiesStr, _ := json.Marshal(responseItem.Properties)
			task.Properties = string(propertiesStr)
		}
		if responseItem.Buttons != nil {
			buttonStr, _ := json.Marshal(responseItem.Buttons)
			task.Buttons = string(buttonStr)
		}

		if (task.Progress != "100%" && responseItem.FailReason != "") || (task.Progress == "100%" && task.Status == "FAILURE") {
			logger.LogError(ctx, task.MjId+" 构建失败，"+task.FailReason)
			task.Progress = "100%"
			err = model.CacheUpdateUserQuota(task.UserId)
			if err != nil {
				logger.LogError(ctx, "error update user quota cache: "+err.Error())
			} else {
				quota := task.Quota
				if quota != 0 {
					err = model.IncreaseUserQuota(task.UserId, quota)
					if err != nil {
						logger.LogError(ctx, "fail to increase user quota: "+err.Error())
					}
					logContent := fmt.Sprintf("构图失败 %s，补偿 %s", task.MjId, common.LogQuota(quota))
					model.RecordLog(task.UserId, model.LogTypeSystem, logContent)
				}
			}
		}
		err = task.Update()
		if err != nil {
			logger.LogError(ctx, "UpdateMidjourneyTask task error: "+err.Error())
		}
	}

	return nil
}

func checkMjTaskNeedUpdate(oldTask *model.Midjourney, newTask provider.MidjourneyDto) bool {
	if oldTask.Code != 1 {
		return true
	}
	if oldTask.Progress != newTask.Progress {
		return true
	}
	if oldTask.PromptEn != newTask.PromptEn {
		return true
	}
	if oldTask.State != newTask.State {
		return true
	}
	if oldTask.SubmitTime != newTask.SubmitTime {
		return true
	}
	if oldTask.StartTime != newTask.StartTime {
		return true
	}
	if oldTask.FinishTime != newTask.FinishTime {
		return true
	}
	if oldTask.ImageUrl != newTask.ImageUrl {
		return true
	}
	if oldTask.Status != newTask.Status {
		return true
	}
	if oldTask.FailReason != newTask.FailReason {
		return true
	}
	if oldTask.FinishTime != newTask.FinishTime {
		return true
	}
	if oldTask.Progress != "100%" && newTask.FailReason != "" {
		return true
	}

	return false
}

func GetAllMidjourney(c *gin.Context) {
	var params model.MJTaskQueryParams
	if err := c.ShouldBindQuery(&params); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	midjourneys, err := model.GetAllMJTasks(&params)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    midjourneys,
	})
}

func GetUserMidjourney(c *gin.Context) {
	userId := c.GetInt("id")

	var params model.MJTaskQueryParams
	if err := c.ShouldBindQuery(&params); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	midjourneys, err := model.GetAllUserMJTask(userId, &params)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    midjourneys,
	})
}
