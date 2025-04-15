package main

import (
	"context"
	dashscopego "github.com/casibase/dashscope-go-sdk"
	"github.com/casibase/dashscope-go-sdk/embedding"
	"log"
	//"os"

	"github.com/casibase/dashscope-go-sdk/qwen"
)

func main() {
	model := qwen.QwenTurbo
	//token := os.Getenv("DASHSCOPE_API_KEY")
	token := "sk-f038f63cb4e3446fb61ba2e9e93a06a8"

	if token == "" {
		panic("token is empty")
	}

	cli := dashscopego.NewTongyiClient(model, token)

	//content := qwen.TextContent{Text: "tell me a joke"}

	//input := dashscopego.TextInput{
	//	Messages: []dashscopego.TextMessage{
	//		{Role: "user", Content: &content},
	//	},
	//}

	// callback function:  print stream result
	//streamCallbackFn := func(ctx context.Context, chunk []byte) error {
	//	log.Print(string(chunk))
	//	return nil
	//}
	//req := &dashscopego.TextRequest{
	//	Input:       input,
	//	StreamingFn: streamCallbackFn,
	//}
	req := &embedding.Request{
		Input: struct {
			Texts []string `json:"texts"`
		}{
			Texts: []string{"tell me a joke"}, // 输入文本
		},
	}

	ctx := context.TODO()
	//resp, err := cli.CreateCompletion(ctx, req)
	resp, err := cli.CreateEmbedding(ctx, req)
	if err != nil {
		panic(err)
	}

	//log.Println("\nnon-stream result: ")
	//log.Println(resp.Output.Choices[0].Message.Content.ToString())
	log.Println(resp)
}
