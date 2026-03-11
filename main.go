package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"arena-cam-go/internal" // Substitua pelo nome do seu módulo caso tenha mudado
)

// Config representa a estrutura do arquivo config.json
type Config struct {
	SegundosGravacao int                        `json:"segundos_gravacao"`
	PastaVideos      string                     `json:"pasta_videos"`
	PastaTemp        string                     `json:"pasta_temp"`
	UploadURL        string                     `json:"upload_url"`
	UploadToken      string                     `json:"upload_token"`
	Quadras          map[string]internal.Camera `json:"quadras"`
}

func main() {
	log.Println("🚀 Iniciando Arena Cam Go...")

	// 1. Ler arquivo de configuração
	configFile, err := os.ReadFile("config.json")
	if err != nil {
		log.Fatalf("❌ Erro ao ler config.json: %v", err)
	}

	var config Config
	if err := json.Unmarshal(configFile, &config); err != nil {
		log.Fatalf("❌ Erro ao fazer parse do config.json: %v", err)
	}

	// 2. Iniciar captura contínua de streams (FFmpeg em background)
	for id, cam := range config.Quadras {
		cam.ID = id // O ID vem da chave do JSON (ex: "11")
		cam.TempDir = config.PastaTemp
		cam.VideoDir = config.PastaVideos
		config.Quadras[id] = cam // Atualiza o map com os diretórios injetados

		go internal.StartStream(cam)
	}

	// 3. Iniciar listener do Joystick
	buttonChannel := make(chan string, 10)
	go internal.ListenJoysticks(buttonChannel)

	// 4. Loop Principal: Aguardar cliques do joystick
	log.Println("✅ Sistema pronto. Aguardando cliques do joystick...")
	for buttonID := range buttonChannel {
		cam, existe := config.Quadras[buttonID]
		if !existe {
			log.Printf("⚠️ Botão %s pressionado, mas não está mapeado no config.json\n", buttonID)
			continue
		}

		// Dispara a rotina de salvar o vídeo e fazer upload sem travar o app
		go processClip(cam, config)
	}
}

// processClip compila o vídeo localmente e depois envia para a nuvem
func processClip(cam internal.Camera, config Config) {
	// A função SaveClip retorna o caminho do arquivo salvo para podermos fazer o upload
	outName := internal.SaveClip(cam, config.SegundosGravacao)

	if outName == "" {
		log.Printf("❌ Falha ao gerar clipe da %s\n", cam.Name)
		return
	}

	if config.UploadURL != "" {
		uploadVideo(outName, config.UploadURL, config.UploadToken)
	}
}

// uploadVideo faz um POST multipart/form-data para o servidor PHP (Hostinger)
func uploadVideo(filePath string, url string, token string) {
	log.Printf("☁️ Iniciando upload de %s para %s...\n", filepath.Base(filePath), url)

	file, err := os.Open(filePath)
	if err != nil {
		log.Printf("❌ Erro ao abrir arquivo para upload: %v\n", err)
		return
	}
	defer file.Close()

	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)

	// Adiciona o campo de token
	writer.WriteField("token", token)

	// Adiciona o arquivo de vídeo
	part, err := writer.CreateFormFile("video", filepath.Base(filePath))
	if err != nil {
		log.Printf("❌ Erro ao criar formulário multipart: %v\n", err)
		return
	}
	io.Copy(part, file)
	writer.Close()

	req, err := http.NewRequest("POST", url, &requestBody)
	if err != nil {
		log.Printf("❌ Erro ao criar requisição de upload: %v\n", err)
		return
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 5 * time.Minute} // Timeout longo para uploads pesados
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("❌ Falha na conexão de upload: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusFound {
		log.Printf("✅ Upload concluído com sucesso: %s\n", filepath.Base(filePath))
	} else {
		log.Printf("⚠️ Erro no servidor (Status %d) ao enviar %s\n", resp.StatusCode, filepath.Base(filePath))
	}
}
