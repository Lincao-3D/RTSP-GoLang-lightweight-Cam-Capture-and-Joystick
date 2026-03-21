/*
No flags usage as there is an uploader dedicated folder

	Comandos para a build do executável:

GOOS=windows GOARCH=amd64 go build -tags uploader -o uploader.exe uploader.go
smaller size: GOOS=windows GOARCH=amd64 go build -tags uploader -ldflags="-H windowsgui -s -w" -o uploader.exe uploader.go


// For using flags: uncomment package main down below to compile, altogether with .vscode/settings.json tag block
// Make sure go.mod file inside uploader folder has the same golang version you have installed declared.
*/

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Estrutura mínima para ler apenas o necessário do config.json
type Config struct {
	UploadURL   string `json:"upload_url"`
	UploadToken string `json:"upload_token"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Uso: uploader.exe <caminho_video>")
		time.Sleep(3 * time.Second)
		os.Exit(1)
	}

	filePath := os.Args[1]

	// 1. Tenta carregar as configurações do config.json na pasta pai
	// Como o uploader está em /videos/quadra_01/uploader.exe, o config está dois níveis acima
	config := loadConfigFromParent()

	// 2. Detecta o nome da quadra pela pasta atual
	absPath, _ := filepath.Abs(filePath)
	dirName := filepath.Base(filepath.Dir(absPath))

	fmt.Printf("☁️ Upload de [%s] para [%s]\n", filepath.Base(filePath), dirName)

	if err := uploadVideo(filePath, config.UploadURL, config.UploadToken, dirName); err != nil {
		log.Printf("❌ Erro: %v", err)
	} else {
		fmt.Println("✅ Upload concluído com sucesso!")
	}

	fmt.Println("\nFechando em 5 segundos...")
	time.Sleep(5 * time.Second)
}

func loadConfigFromParent() Config {
	// 1. Descobre o caminho real e absoluto de onde o uploader.exe está salvo
	exePath, err := os.Executable()
	if err != nil {
		fmt.Println("⚠️ Erro ao localizar executável, tentando caminhos relativos...")
		exePath = "." // Fallback
	}

	// 2. Calcula os diretórios subindo a árvore
	// Exemplo: C:\ArenaCam\videos\quadra_01\uploader.exe
	dirAtual := filepath.Dir(exePath) // C:\ArenaCam\videos\quadra_01
	dirPai := filepath.Dir(dirAtual)  // C:\ArenaCam\videos
	dirRaiz := filepath.Dir(dirPai)   // C:\ArenaCam

	// 3. Monta os caminhos absolutos exatos para procurar o config.json
	pathsToTry := []string{
		filepath.Join(dirRaiz, "config.json"),  // Se estiver dentro de videos/quadra_01/
		filepath.Join(dirPai, "config.json"),   // Se estiver dentro de videos/
		filepath.Join(dirAtual, "config.json"), // Se estiver na mesma pasta (raiz)
	}

	var config Config
	for _, path := range pathsToTry {
		file, err := os.ReadFile(path)
		if err == nil {
			json.Unmarshal(file, &config)
			fmt.Printf("✅ Configurações lidas de: %s\n", path) // Mostra onde achou
			return config
		}
	}

	// 4. Se chegou aqui, não achou em lugar nenhum
	fmt.Println("❌ AVISO: config.json não encontrado nas pastas verificadas!")
	return Config{
		UploadURL:   "https://www.replayai.com.br/upload.php",
		UploadToken: "TOKEN_NAO_ENCONTRADO",
	}
}

func uploadVideo(filePath, url, token, dirName string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)

	// Aqui o token vindo do JSON é injetado
	writer.WriteField("token", token)
	writer.WriteField("dir", dirName)

	part, _ := writer.CreateFormFile("video", filepath.Base(filePath))
	io.Copy(part, file)
	writer.Close()

	req, _ := http.NewRequest("POST", url, &requestBody)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{
		Timeout: 10 * time.Minute,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
