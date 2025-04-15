package paraformer

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	httpclient "github.com/casibase/dashscope-go-sdk/httpclient"
	"github.com/google/uuid"
)

func ConnRecognitionClient(request *Request, token string) (*httpclient.WsClient, error) {
	// Initialize the client with the necessary parameters.
	header := http.Header{}
	header.Add("Authorization", token)

	client := httpclient.NewWsClient(ParaformerWSURL, header)

	if err := client.ConnClient(request); err != nil {
		return nil, err
	}

	return client, nil
}

func CloseRecognitionClient(cli *httpclient.WsClient) error {
	return cli.CloseClient()
}

func SendRadioData(cli *httpclient.WsClient, bytesData []byte) {
	cli.SendBinaryDates(bytesData)
}

type ResultWriter interface {
	WriteResult(str string) error
}

func HandleRecognitionResult(ctx context.Context, cli *httpclient.WsClient, fn StreamingFunc) error {
	outputChan, errChan := cli.ResultChans()

	for {
		select {
		case output, ok := <-outputChan:
			if !ok {
				return fmt.Errorf("outputChan is closed")
			}

			// streaming callback func
			if err := fn(ctx, output.Data); err != nil {
				return err
			}

		case err := <-errChan:
			if err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// task_id length 32.
func GenerateTaskID() string {
	u, err := uuid.NewUUID()
	if err != nil {
		panic(err)
	}
	uuid := strings.ReplaceAll(u.String(), "-", "")

	return uuid
}
