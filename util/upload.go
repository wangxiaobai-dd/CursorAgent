package util

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
)

func UploadFile(serverURL, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)

	fileField, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return fmt.Errorf("failed to create form file: %v", err)
	}

	if _, err = io.Copy(fileField, file); err != nil {
		return fmt.Errorf("failed to copy file content: %v", err)
	}
	writer.Close()

	response, err := http.Post(serverURL, writer.FormDataContentType(), &requestBody)
	if err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %v", err)
	}

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned non-200 status: %d, response: %s", response.StatusCode, string(responseBody))
	}

	log.Printf("finish upload file, response:%s", string(responseBody))
	return nil
}
