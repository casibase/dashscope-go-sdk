package qwen

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"

	httpclient "github.com/casibase/dashscope-go-sdk/httpclient"
)

//nolint:lll
func SendMessage[T IQwenContent](ctx context.Context, payload *Request[T], cli httpclient.IHttpClient, url, token string) (*OutputResponse[T], error) {
	if payload.Model == "" {
		return nil, ErrModelNotSet
	}

	resp := OutputResponse[T]{}
	tokenOpt := httpclient.WithTokenHeaderOption(token)

	header := map[string]string{
		"Content-Type": "application/json",
	}
	if payload.HasUploadOss {
		header["X-DashScope-OssResourceResolve"] = "enable"
	}

	headerOpt := httpclient.WithHeader(header)

	err := cli.Post(ctx, url, payload, &resp, tokenOpt, headerOpt)
	if err != nil {
		return nil, err
	}
	if len(resp.Output.Choices) == 0 {
		return nil, ErrEmptyResponse
	}
	return &resp, nil
}

//nolint:lll
func SendMessageStream[T IQwenContent](ctx context.Context, payload *Request[T], cli httpclient.IHttpClient, url, token string) (*OutputResponse[T], error) {
	if payload.Model == "" {
		return nil, ErrModelNotSet
	}

	header := map[string]string{
		"Accept":       "text/event-stream",
		"Content-Type": "application/json",
	}

	if payload.HasUploadOss {
		header["X-DashScope-OssResourceResolve"] = "enable"
	}

	responseChan := asyncChatStreaming(ctx, payload, header, cli, url, token)

	return iterateStreamChannel(ctx, responseChan, payload.StreamingFn)
}

func iterateStreamChannel[T IQwenContent](ctx context.Context, channel <-chan StreamOutput[T], fn StreamingFunc) (*OutputResponse[T], error) {
	outputMessage := OutputResponse[T]{}
	for rspData := range channel {
		if rspData.Err != nil {
			return nil, &httpclient.HTTPRequestError{Message: "SSE Error: ", Cause: rspData.Err}
		}
		if len(rspData.Output.Output.Choices) == 0 {
			return nil, ErrEmptyResponse
		}

		chunk := rspData.Output.Output.Choices[0].Message.Content.ToBytes()

		if err := fn(ctx, chunk); err != nil {
			return nil, &WrapMessageError{Message: "StreamingFunc Error", Cause: err}
		}

		outputMessage.RequestID = rspData.Output.RequestID
		outputMessage.Usage = rspData.Output.Usage
		if outputMessage.Output.Choices == nil {
			outputMessage.Output.Choices = rspData.Output.Output.Choices
		} else {
			choice := outputMessage.Output.Choices[0]
			choice.Message.Role = rspData.Output.Output.Choices[0].Message.Role
			choice.Message.Content.AppendText(rspData.Output.Output.Choices[0].Message.Content.ToString())
			choice.FinishReason = rspData.Output.Output.Choices[0].FinishReason

			outputMessage.Output.Choices[0] = choice
		}
	}

	return &outputMessage, nil
}

//nolint:lll
func asyncChatStreaming[T IQwenContent](
	ctx context.Context,
	payload *Request[T],
	header map[string]string,
	cli httpclient.IHttpClient,
	url, token string,
) <-chan StreamOutput[T] {
	chanBuffer := 100
	_respChunkChannel := make(chan StreamOutput[T], chanBuffer)

	go func() {
		_combineStreamingChunk(ctx, payload, header, _respChunkChannel, cli, url, token)
	}()
	return _respChunkChannel
}

/*
 * combine SSE streaming lines to be a structed response data
 * id: xxxx
 * event: xxxxx
 * ......
 */
func _combineStreamingChunk[T IQwenContent](
	ctx context.Context,
	payload *Request[T],
	header map[string]string,
	_respChunkChannel chan StreamOutput[T],
	cli httpclient.IHttpClient,
	url string,
	token string,
) {
	defer close(_respChunkChannel)
	var _rawStreamOutChannel chan string

	var err error
	headerOpt := httpclient.WithHeader(header)
	tokenOpt := httpclient.WithTokenHeaderOption(token)

	_rawStreamOutChannel, err = cli.PostSSE(ctx, url, payload, headerOpt, tokenOpt)
	if err != nil {
		_respChunkChannel <- StreamOutput[T]{Err: err}
		return
	}

	rsp := StreamOutput[T]{}

	for v := range _rawStreamOutChannel {
		if strings.TrimSpace(v) == "" {
			// streaming out combined response
			_respChunkChannel <- rsp
			rsp = StreamOutput[T]{}
			continue
		}

		err = fillInRespData(v, &rsp)
		if err != nil {
			rsp.Err = err
			_respChunkChannel <- rsp
			break
		}
	}
}

// filled in response data line by line.
func fillInRespData[T IQwenContent](line string, output *StreamOutput[T]) error {
	if strings.TrimSpace(line) == "" {
		return nil
	}

	switch {
	case strings.HasPrefix(line, "id:"):
		output.ID = strings.TrimPrefix(line, "id:")
	case strings.HasPrefix(line, "event:"):
		output.Event = strings.TrimPrefix(line, "event:")
	case strings.HasPrefix(line, ":HTTP_STATUS/"):
		code, err := strconv.Atoi(strings.TrimPrefix(line, ":HTTP_STATUS/"))
		if err != nil {
			output.Err = fmt.Errorf("http_status err: strconv.Atoi  %w", err)
		}
		output.HTTPStatus = code
	case strings.HasPrefix(line, "data:"):
		dataJSON := strings.TrimPrefix(line, "data:")
		if output.Event == "error" {
			output.Err = &WrapMessageError{Message: dataJSON}
			return nil
		}
		outputData := OutputResponse[T]{}
		err := json.Unmarshal([]byte(dataJSON), &outputData)
		if err != nil {
			return &WrapMessageError{Message: "unmarshal OutputData Err", Cause: err}
		}

		output.Output = outputData
	default:
		data := bytes.TrimSpace([]byte(line))
		log.Printf("unknown line: %s", data)
	}

	return nil
}
