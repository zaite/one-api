package relay

import (
	"fmt"
	"net/http"
	"one-api/common"
	"one-api/model"
	"one-api/relay/util"
	"one-api/types"
	"time"

	"github.com/gin-gonic/gin"
)

func Relay(c *gin.Context) {
	relay := Path2Relay(c, c.Request.URL.Path)
	if relay == nil {
		common.AbortWithMessage(c, http.StatusNotFound, "Not Found")
		return
	}

	if err := relay.setRequest(); err != nil {
		common.AbortWithMessage(c, http.StatusBadRequest, err.Error())
		return
	}

	cacheProps := relay.GetChatCache()
	cacheProps.SetHash(relay.getRequest())

	// 获取缓存
	cache := cacheProps.GetCache()

	if cache != nil {
		// 说明有缓存， 直接返回缓存内容
		cacheProcessing(c, cache)
		return
	}

	if err := relay.setProvider(relay.getOriginalModel()); err != nil {
		common.AbortWithMessage(c, http.StatusServiceUnavailable, err.Error())
		return
	}

	apiErr, done := RelayHandler(relay)
	if apiErr == nil {
		return
	}

	channel := relay.getProvider().GetChannel()
	go processChannelRelayError(c.Request.Context(), channel.Id, channel.Name, apiErr)

	retryTimes := common.RetryTimes
	if done || !shouldRetry(c, apiErr.StatusCode) {
		common.LogError(c.Request.Context(), fmt.Sprintf("relay error happen, status code is %d, won't retry in this case", apiErr.StatusCode))
		retryTimes = 0
	}

	for i := retryTimes; i > 0; i-- {
		// 冻结通道
		model.ChannelGroup.Cooldowns(channel.Id)
		if err := relay.setProvider(relay.getOriginalModel()); err != nil {
			continue
		}

		channel = relay.getProvider().GetChannel()
		common.LogError(c.Request.Context(), fmt.Sprintf("using channel #%d(%s) to retry (remain times %d)", channel.Id, channel.Name, i))
		apiErr, done = RelayHandler(relay)
		if apiErr == nil {
			return
		}
		go processChannelRelayError(c.Request.Context(), channel.Id, channel.Name, apiErr)
		if done || !shouldRetry(c, apiErr.StatusCode) {
			break
		}
	}

	if apiErr != nil {
		if apiErr.StatusCode == http.StatusTooManyRequests {
			apiErr.OpenAIError.Message = "当前分组上游负载已饱和，请稍后再试"
		}
		relayResponseWithErr(c, apiErr)
	}
}

func RelayHandler(relay RelayBaseInterface) (err *types.OpenAIErrorWithStatusCode, done bool) {
	promptTokens, tonkeErr := relay.getPromptTokens()
	if tonkeErr != nil {
		err = common.ErrorWrapper(tonkeErr, "token_error", http.StatusBadRequest)
		done = true
		return
	}

	usage := &types.Usage{
		PromptTokens: promptTokens,
	}

	relay.getProvider().SetUsage(usage)

	var quota *util.Quota
	quota, err = util.NewQuota(relay.getContext(), relay.getModelName(), promptTokens)
	if err != nil {
		done = true
		return
	}

	err, done = relay.send()

	if err != nil {
		quota.Undo(relay.getContext())
		return
	}

	quota.Consume(relay.getContext(), usage)
	if usage.CompletionTokens > 0 {
		cacheProps := relay.GetChatCache()
		go cacheProps.StoreCache(relay.getContext().GetInt("channel_id"), usage.PromptTokens, usage.CompletionTokens, relay.getModelName())
	}

	return
}

func cacheProcessing(c *gin.Context, cacheProps *util.ChatCacheProps) {
	responseCache(c, cacheProps.Response)

	// 写入日志
	tokenName := c.GetString("token_name")

	requestTime := 0
	requestStartTimeValue := c.Request.Context().Value("requestStartTime")
	if requestStartTimeValue != nil {
		requestStartTime, ok := requestStartTimeValue.(time.Time)
		if ok {
			requestTime = int(time.Since(requestStartTime).Milliseconds())
		}
	}

	model.RecordConsumeLog(c.Request.Context(), cacheProps.UserId, cacheProps.ChannelID, cacheProps.PromptTokens, cacheProps.CompletionTokens, cacheProps.ModelName, tokenName, 0, "缓存", requestTime)
}
