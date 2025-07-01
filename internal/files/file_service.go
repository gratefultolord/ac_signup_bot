package files

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/uuid"
)

type FileService struct {
	botAPI *tgbotapi.BotAPI
	docDir string
}

func NewFileService(botAPI *tgbotapi.BotAPI, docDir string) (*FileService, error) {
	if err := os.MkdirAll(docDir, 0755); err != nil {
		return nil, fmt.Errorf("FilService: cannot create dir %s: %w", docDir, err)
	}

	return &FileService{
		botAPI: botAPI,
		docDir: docDir,
	}, nil
}

func (fs *FileService) SaveFile(fileID string) (string, error) {
	file, err := fs.botAPI.GetFile(tgbotapi.FileConfig{FileID: fileID})
	if err != nil {
		return "", fmt.Errorf("FileService.SaveFile: cannot get file: %w", err)
	}

	fileExt := filepath.Ext(file.FilePath)
	if fileExt == "" {
		fileExt = ".jpg"
	}

	fileName := fmt.Sprintf("%s%s", uuid.New().String(), fileExt)
	filePath := filepath.Join(fs.docDir, fileName)

	resp, err := http.Get(file.Link(fs.botAPI.Token))
	if err != nil {
		return "", fmt.Errorf("FileService.SaveFile: cannot download file: %w", err)
	}

	defer resp.Body.Close()

	out, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("FileService.SaveFile: cannot create file: %w", err)
	}

	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return "", fmt.Errorf("FileService.SaveFile: cannot save file: %w", err)
	}

	return filePath, nil
}

func (fs *FileService) DeleteFile(path string) error {
	if path == "" {
		return nil
	}

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("FileService.DeleteFile: %w", err)
	}

	return nil
}
