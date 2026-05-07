package services

import (
	"context"
	"fmt"
	"os"

	"meeting-notes/internal/audio"
)

func CheckModelLoaded(ctx context.Context, client audio.Client) error {
	h, err := client.Health(ctx)
	if err != nil {
		return fmt.Errorf("serviço de áudio indisponível: %w", err)
	}
	if !h.ModelLoaded {
		return fmt.Errorf("modelo de transcrição ainda está carregando, tente novamente em alguns segundos")
	}
	return nil
}

func ValidateWAVFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("arquivo de áudio não encontrado: %s", path)
		}
		return fmt.Errorf("erro ao verificar arquivo de áudio: %w", err)
	}
	const minBytes = 10 * 1024
	if info.Size() < minBytes {
		return fmt.Errorf("arquivo de áudio muito pequeno (%d bytes), a gravação pode estar vazia", info.Size())
	}
	return nil
}
