package openai

import (
	"encoding/json"
	"io"
	"net/http"
	"one-api/common"
	"one-api/common/requester"
	"one-api/types"
	"strings"
)

func (p *OpenAIProvider) CreateCompletion(request *types.CompletionRequest) (openaiResponse *types.CompletionResponse, errWithCode *types.OpenAIErrorWithStatusCode) {
	req, errWithCode := p.GetRequestTextBody(common.RelayModeCompletions, request.Model, request)
	if errWithCode != nil {
		return nil, errWithCode
	}
	defer req.Body.Close()

	response := &OpenAIProviderCompletionResponse{}
	// 发送请求
	_, errWithCode = p.Requester.SendRequest(req, response, false)
	if errWithCode != nil {
		return nil, errWithCode
	}

	// 检测是否错误
	openaiErr := ErrorHandle(&response.OpenAIErrorResponse)
	if openaiErr != nil {
		errWithCode = &types.OpenAIErrorWithStatusCode{
			OpenAIError: *openaiErr,
			StatusCode:  http.StatusBadRequest,
		}
		return nil, errWithCode
	}

	*p.Usage = *response.Usage

	return &response.CompletionResponse, nil
}

func (p *OpenAIProvider) CreateCompletionStream(request *types.CompletionRequest) (stream requester.StreamReaderInterface[string], errWithCode *types.OpenAIErrorWithStatusCode) {
	req, errWithCode := p.GetRequestTextBody(common.RelayModeCompletions, request.Model, request)
	if errWithCode != nil {
		return nil, errWithCode
	}
	defer req.Body.Close()

	// 发送请求
	resp, errWithCode := p.Requester.SendRequestRaw(req)
	if errWithCode != nil {
		return nil, errWithCode
	}

	chatHandler := OpenAIStreamHandler{
		Usage:     p.Usage,
		ModelName: request.Model,
	}

	return requester.RequestStream[string](p.Requester, resp, chatHandler.handlerCompletionStream)
}

func (h *OpenAIStreamHandler) handlerCompletionStream(rawLine *[]byte, dataChan chan string, errChan chan error) {
	// 如果rawLine 前缀不为data:，则直接返回
	if !strings.HasPrefix(string(*rawLine), "data: ") {
		*rawLine = nil
		return
	}

	// 去除前缀
	*rawLine = (*rawLine)[6:]

	// 如果等于 DONE 则结束
	if string(*rawLine) == "[DONE]" {
		errChan <- io.EOF
		*rawLine = requester.StreamClosed
		return
	}

	var openaiResponse OpenAIProviderCompletionResponse
	err := json.Unmarshal(*rawLine, &openaiResponse)
	if err != nil {
		errChan <- common.ErrorToOpenAIError(err)
		return
	}

	error := ErrorHandle(&openaiResponse.OpenAIErrorResponse)
	if error != nil {
		errChan <- error
		return
	}

	dataChan <- string(*rawLine)

	countTokenText := common.CountTokenText(openaiResponse.getResponseText(), h.ModelName)
	h.Usage.CompletionTokens += countTokenText
	h.Usage.TotalTokens += countTokenText
}
