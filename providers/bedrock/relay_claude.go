package bedrock

import (
	"net/http"
	"one-api/common/config"
	"one-api/common/requester"
	"one-api/providers/bedrock/category"
	"one-api/providers/claude"
)

func (p *BedrockProvider) CreateClaudeChat(request *claude.ClaudeRequest) (*claude.ClaudeResponse, *claude.ClaudeErrorWithStatusCode) {
	req, errWithCode := p.getClaudeRequest(request)
	if errWithCode != nil {
		return nil, errWithCode
	}
	defer req.Body.Close()

	claudeResponse := &claude.ClaudeResponse{}
	// // 发送请求
	_, openaiErr := p.Requester.SendRequest(req, claudeResponse, false)
	if openaiErr != nil {
		return nil, claude.OpenaiErrToClaudeErr(openaiErr)
	}

	claude.ClaudeUsageToOpenaiUsage(&claudeResponse.Usage, p.GetUsage())

	return claudeResponse, nil
}

func (p *BedrockProvider) CreateClaudeChatStream(request *claude.ClaudeRequest) (requester.StreamReaderInterface[string], *claude.ClaudeErrorWithStatusCode) {
	req, errWithCode := p.getClaudeRequest(request)
	if errWithCode != nil {
		return nil, errWithCode
	}
	defer req.Body.Close()

	chatHandler := &claude.ClaudeRelayStreamHandler{
		Usage:     p.Usage,
		ModelName: request.Model,
		Prefix:    `{"type"`,
		AddEvent:  true,
	}

	// 发送请求
	resp, openaiErr := p.Requester.SendRequestRaw(req)
	if openaiErr != nil {
		return nil, claude.OpenaiErrToClaudeErr(openaiErr)
	}

	stream, openaiErr := RequestStream(resp, chatHandler.HandlerStream)
	if openaiErr != nil {
		return nil, claude.OpenaiErrToClaudeErr(openaiErr)
	}

	return stream, nil
}

func (p *BedrockProvider) getClaudeRequest(request *claude.ClaudeRequest) (*http.Request, *claude.ClaudeErrorWithStatusCode) {
	var err error
	p.Category, err = category.GetCategory(request.Model)
	if err != nil || p.Category == nil {
		return nil, claude.StringErrorWrapper("bedrock provider not found", "bedrock_err", http.StatusInternalServerError, true)
	}

	url, errWithCode := p.GetSupportedAPIUri(config.RelayModeChatCompletions)
	if errWithCode != nil {
		return nil, claude.StringErrorWrapper("bedrock config error", "invalid_bedrock_config", http.StatusInternalServerError, true)
	}

	if request.Stream {
		url += "-with-response-stream"
	}

	// 获取请求地址
	fullRequestURL := p.GetFullRequestURL(url, p.Category.ModelName)
	if fullRequestURL == "" {
		return nil, claude.StringErrorWrapper("bedrock config error", "invalid_bedrock_config", http.StatusInternalServerError, true)
	}

	headers := p.GetRequestHeaders()

	if headers == nil {
		return nil, claude.StringErrorWrapper("bedrock config error", "invalid_bedrock_config", http.StatusInternalServerError, true)
	}

	bedrockRequest := &category.ClaudeRequest{
		ClaudeRequest:    request,
		AnthropicVersion: category.AnthropicVersion,
	}
	bedrockRequest.Model = ""
	bedrockRequest.Stream = false

	// 创建请求
	req, err := p.Requester.NewRequest(http.MethodPost, fullRequestURL, p.Requester.WithBody(bedrockRequest), p.Requester.WithHeader(headers))
	if err != nil {
		return nil, claude.StringErrorWrapper(err.Error(), "new_request_failed", http.StatusInternalServerError, true)
	}

	p.Sign(req)

	return req, nil
}
