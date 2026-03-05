package controller

// fallback_relay.go — Token-level model fallback chain (yocloud custom)
//
// When a token has FallbackModels configured (comma-separated model list),
// and the primary model exhausts all channel retries, this logic will
// sequentially try each fallback model with full channel retry.

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// getFallbackModels retrieves the fallback model chain for the current token.
// Returns nil if no fallback is configured.
func getFallbackModels(c *gin.Context) []string {
	tokenId := c.GetInt("token_id")
	userId := c.GetInt("id")
	if tokenId == 0 || userId == 0 {
		return nil
	}
	token, err := model.GetTokenByIds(tokenId, userId)
	if err != nil || token == nil {
		return nil
	}
	return token.GetFallbackModels()
}

// relayWithRetry executes the channel-level retry loop for a single model.
// This is extracted from the original Relay() function for reuse in fallback chain.
// Returns the last error, or nil on success.
func relayWithRetry(
	c *gin.Context,
	relayInfo *relaycommon.RelayInfo,
	relayFormat types.RelayFormat,
) *types.NewAPIError {
	retryParam := &service.RetryParam{
		Ctx:        c,
		TokenGroup: relayInfo.TokenGroup,
		ModelName:  relayInfo.OriginModelName,
		Retry:      common.GetPointer(0),
	}
	relayInfo.RetryIndex = 0
	relayInfo.LastError = nil

	var newAPIError *types.NewAPIError

	for ; retryParam.GetRetry() <= common.RetryTimes; retryParam.IncreaseRetry() {
		relayInfo.RetryIndex = retryParam.GetRetry()
		channel, channelErr := getChannel(c, relayInfo, retryParam)
		if channelErr != nil {
			logger.LogError(c, channelErr.Error())
			newAPIError = channelErr
			break
		}

		addUsedChannel(c, channel.Id)
		bodyStorage, bodyErr := common.GetBodyStorage(c)
		if bodyErr != nil {
			if common.IsRequestBodyTooLargeError(bodyErr) {
				newAPIError = types.NewErrorWithStatusCode(bodyErr, types.ErrorCodeReadRequestBodyFailed, 413, types.ErrOptionWithSkipRetry())
			} else {
				newAPIError = types.NewErrorWithStatusCode(bodyErr, types.ErrorCodeReadRequestBodyFailed, 400, types.ErrOptionWithSkipRetry())
			}
			break
		}
		c.Request.Body = io.NopCloser(bodyStorage)

		switch relayFormat {
		case types.RelayFormatOpenAIRealtime:
			newAPIError = relay.WssHelper(c, relayInfo)
		case types.RelayFormatClaude:
			newAPIError = relay.ClaudeHelper(c, relayInfo)
		case types.RelayFormatGemini:
			newAPIError = geminiRelayHandler(c, relayInfo)
		default:
			newAPIError = relayHandler(c, relayInfo)
		}

		if newAPIError == nil {
			relayInfo.LastError = nil
			return nil
		}

		newAPIError = service.NormalizeViolationFeeError(newAPIError)
		relayInfo.LastError = newAPIError

		processChannelError(c, *types.NewChannelError(channel.Id, channel.Type, channel.Name,
			channel.ChannelInfo.IsMultiKey,
			common.GetContextKeyString(c, constant.ContextKeyChannelKey),
			channel.GetAutoBan()), newAPIError)

		if !shouldRetry(c, newAPIError, common.RetryTimes-retryParam.GetRetry()) {
			break
		}
	}

	return newAPIError
}

// tryFallbackModels attempts each fallback model in sequence.
// For each model, it rewrites the request body's "model" field and runs full channel retry.
// Returns nil on first success, or the last error if all fallbacks fail.
func tryFallbackModels(
	c *gin.Context,
	relayInfo *relaycommon.RelayInfo,
	relayFormat types.RelayFormat,
	fallbackModels []string,
	originalModel string,
	meta *types.TokenCountMeta,
	tokens int,
) *types.NewAPIError {
	for i, fbModel := range fallbackModels {
		// Skip if same as original (already tried)
		if fbModel == originalModel {
			continue
		}

		logger.LogInfo(c, fmt.Sprintf("[fallback] 降级调用 %d/%d: %s → %s",
			i+1, len(fallbackModels), originalModel, fbModel))

		// Rewrite model name in stored request body
		if err := rewriteRequestModel(c, fbModel); err != nil {
			logger.LogError(c, fmt.Sprintf("[fallback] 重写请求体失败: %s", err.Error()))
			continue
		}

		// Update relay info for new model
		relayInfo.OriginModelName = fbModel
		c.Set("original_model", fbModel)

		// Recalculate price for new model
		_, priceErr := helper.ModelPriceHelper(c, relayInfo, tokens, meta)
		if priceErr != nil {
			logger.LogWarn(c, fmt.Sprintf("[fallback] 模型 %s 定价失败，跳过: %s", fbModel, priceErr.Error()))
			continue
		}

		// Reset channel selection state for fresh try
		c.Set("use_channel", []string{})

		// Run full retry loop for this fallback model
		fbErr := relayWithRetry(c, relayInfo, relayFormat)
		if fbErr == nil {
			logger.LogInfo(c, fmt.Sprintf("[fallback] 降级成功: 最终使用模型 %s (第 %d 级降级)", fbModel, i+1))
			return nil
		}

		logger.LogWarn(c, fmt.Sprintf("[fallback] 模型 %s 全部渠道失败: %s", fbModel, fbErr.Error()))
	}

	return relayInfo.LastError
}

// rewriteRequestModel rewrites the "model" field in the stored request body.
func rewriteRequestModel(c *gin.Context, newModel string) error {
	bodyStorage, err := common.GetBodyStorage(c)
	if err != nil {
		return err
	}
	bodyBytes, err := bodyStorage.Bytes()
	if err != nil {
		return err
	}

	// Parse JSON, replace model field, re-serialize
	var bodyMap map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &bodyMap); err != nil {
		return fmt.Errorf("parse request body: %w", err)
	}
	bodyMap["model"] = newModel
	newBody, err := json.Marshal(bodyMap)
	if err != nil {
		return fmt.Errorf("serialize request body: %w", err)
	}

	// Create new body storage and replace
	newStorage, err := common.CreateBodyStorage(newBody)
	if err != nil {
		return fmt.Errorf("create body storage: %w", err)
	}
	c.Set(common.KeyBodyStorage, newStorage)
	return nil
}
