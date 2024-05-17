package tencent

import (
	"encoding/json"
	"errors"
	"net/http"
	"one-api/common"
	"one-api/common/requester"
	"one-api/types"
	"strings"
)

type tencentStreamHandler struct {
	Usage   *types.Usage
	Request *types.ChatCompletionRequest
}

func (p *TencentProvider) CreateChatCompletion(request *types.ChatCompletionRequest) (*types.ChatCompletionResponse, *types.OpenAIErrorWithStatusCode) {
	req, errWithCode := p.getChatRequest(request)
	if errWithCode != nil {
		return nil, errWithCode
	}
	defer req.Body.Close()

	tencentChatResponse := &TencentChatResponse{}
	// 发送请求
	_, errWithCode = p.Requester.SendRequest(req, tencentChatResponse, false)
	if errWithCode != nil {
		return nil, errWithCode
	}

	return p.convertToChatOpenai(tencentChatResponse, request)
}

func (p *TencentProvider) CreateChatCompletionStream(request *types.ChatCompletionRequest) (requester.StreamReaderInterface[string], *types.OpenAIErrorWithStatusCode) {
	req, errWithCode := p.getChatRequest(request)
	if errWithCode != nil {
		return nil, errWithCode
	}
	defer req.Body.Close()

	// 发送请求
	resp, errWithCode := p.Requester.SendRequestRaw(req)
	if errWithCode != nil {
		return nil, errWithCode
	}

	chatHandler := &tencentStreamHandler{
		Usage:   p.Usage,
		Request: request,
	}

	return requester.RequestStream[string](p.Requester, resp, chatHandler.handlerStream)
}

func (p *TencentProvider) getChatRequest(request *types.ChatCompletionRequest) (*http.Request, *types.OpenAIErrorWithStatusCode) {
	url, errWithCode := p.GetSupportedAPIUri(common.RelayModeChatCompletions)
	if errWithCode != nil {
		return nil, errWithCode
	}

	tencentRequest := convertFromChatOpenai(request)

	sign := p.getTencentSign(tencentRequest)
	if sign == "" {
		return nil, common.ErrorWrapper(errors.New("get tencent sign failed"), "get_tencent_sign_failed", http.StatusInternalServerError)
	}

	// 获取请求地址
	fullRequestURL := p.GetFullRequestURL(url, request.Model)
	if fullRequestURL == "" {
		return nil, common.ErrorWrapper(nil, "invalid_tencent_config", http.StatusInternalServerError)
	}

	// 获取请求头
	headers := p.GetRequestHeaders()
	headers["Authorization"] = sign
	headers["X-TC-Action"] = request.Model
	if request.Stream {
		headers["Accept"] = "text/event-stream"
	}

	// 创建请求
	req, err := p.Requester.NewRequest(http.MethodPost, fullRequestURL, p.Requester.WithBody(tencentRequest), p.Requester.WithHeader(headers))
	if err != nil {
		return nil, common.ErrorWrapper(err, "new_request_failed", http.StatusInternalServerError)
	}

	return req, nil
}

func (p *TencentProvider) convertToChatOpenai(response *TencentChatResponse, request *types.ChatCompletionRequest) (openaiResponse *types.ChatCompletionResponse, errWithCode *types.OpenAIErrorWithStatusCode) {
	error := errorHandle(&response.TencentResponseError)
	if error != nil {
		errWithCode = &types.OpenAIErrorWithStatusCode{
			OpenAIError: *error,
			StatusCode:  http.StatusBadRequest,
		}
		return
	}

	openaiResponse = &types.ChatCompletionResponse{
		Object:  "chat.completion",
		Created: common.GetTimestamp(),
		Usage:   response.Usage,
		Model:   request.Model,
	}
	if len(response.Choices) > 0 {
		choice := types.ChatCompletionChoice{
			Index: 0,
			Message: types.ChatCompletionMessage{
				Role:    "assistant",
				Content: response.Choices[0].Messages.Content,
			},
			FinishReason: response.Choices[0].FinishReason,
		}
		openaiResponse.Choices = append(openaiResponse.Choices, choice)
	}

	*p.Usage = *response.Usage

	return
}

func convertFromChatOpenai(request *types.ChatCompletionRequest) *TencentChatRequest {
	messages := make([]TencentMessage, 0, len(request.Messages))
	for i := 0; i < len(request.Messages); i++ {
		message := request.Messages[i]
		messages = append(messages, TencentMessage{
			Content: message.StringContent(),
			Role:    message.Role,
		})
	}
	stream := 0
	if request.Stream {
		stream = 1
	}
	return &TencentChatRequest{
		Timestamp:   common.GetTimestamp(),
		Expired:     common.GetTimestamp() + 24*60*60,
		QueryID:     common.GetUUID(),
		Temperature: request.Temperature,
		TopP:        request.TopP,
		Stream:      stream,
		Messages:    messages,
	}
}

// 转换为OpenAI聊天流式请求体
func (h *tencentStreamHandler) handlerStream(rawLine *[]byte, dataChan chan string, errChan chan error) {
	// 如果rawLine 前缀不为data:，则直接返回
	if !strings.HasPrefix(string(*rawLine), "data:") {
		*rawLine = nil
		return
	}

	// 去除前缀
	*rawLine = (*rawLine)[5:]

	var tencentChatResponse TencentChatResponse
	err := json.Unmarshal(*rawLine, &tencentChatResponse)
	if err != nil {
		errChan <- common.ErrorToOpenAIError(err)
		return
	}

	error := errorHandle(&tencentChatResponse.TencentResponseError)
	if error != nil {
		errChan <- error
		return
	}

	h.convertToOpenaiStream(&tencentChatResponse, dataChan)

}

func (h *tencentStreamHandler) convertToOpenaiStream(tencentChatResponse *TencentChatResponse, dataChan chan string) {
	streamResponse := types.ChatCompletionStreamResponse{
		Object:  "chat.completion.chunk",
		Created: common.GetTimestamp(),
		Model:   h.Request.Model,
	}
	if len(tencentChatResponse.Choices) > 0 {
		var choice types.ChatCompletionStreamChoice
		choice.Delta.Content = tencentChatResponse.Choices[0].Delta.Content
		if tencentChatResponse.Choices[0].FinishReason == "stop" {
			choice.FinishReason = types.FinishReasonStop
		}
		streamResponse.Choices = append(streamResponse.Choices, choice)
	}

	responseBody, _ := json.Marshal(streamResponse)
	dataChan <- string(responseBody)

	h.Usage.CompletionTokens += common.CountTokenText(tencentChatResponse.Choices[0].Delta.Content, h.Request.Model)
	h.Usage.TotalTokens = h.Usage.PromptTokens + h.Usage.CompletionTokens
}
