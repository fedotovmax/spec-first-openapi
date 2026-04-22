package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	client_v1 "github.com/fedotovmax/spec-first-openapi/pkg/openapi/client/v1"
	"github.com/google/uuid"
)

func main() {

	client, err := client_v1.NewClientWithResponses("http://localhost:5000")

	if err != nil {
		panic(err)
	}

	resp, err := client.GetTaskByIDWithResponse(context.Background(), uuid.New())

	if err != nil {
		panic(err)
	}

	status := resp.StatusCode()

	switch status {
	case http.StatusOK:

		b, _ := json.MarshalIndent(resp.JSON200, "", "  ")

		fmt.Println(string(b))
	default:
		fmt.Println("Error, status: ", status)
	}

}
