//go:build main && !uploader


// package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"arena-cam-go/internal"
)

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

	fmt.Println("======= CONFIGURAÇÃO INICIAL =======")
	fmt.Print("1. Ativar Upload Automático? (s/n): ")
	var resp string
	fmt.Scanln(&resp)
	autoUpload := (resp == "s" || resp == "S")

	fmt.Print("2. Timeout para desligar captura/PC (em horas, 0 para ilimitado): ")
	var hours int
	fmt.Scanln(&hours)

	config := loadConfig()
	stopChan := make(chan bool)

	for id, cam := range config.Quadras {
		cam.ID = id
		cam.TempDir = config.PastaTemp
		cam.VideoDir = config.PastaVideos
		config.Quadras[id] = cam

		go internal.StartStream(cam, stopChan)
	}

	if hours > 0 {
		go func() {
			log.Printf("⏰ Timeout configurado para %d horas...", hours)
			time.Sleep(time.Duration(hours) * time.Hour)
			log.Println("⏰ Timeout atingido! Encerrando gravações...")
			close(stopChan)
			time.Sleep(5 * time.Second)
			log.Println("💻 Desligando computador...")
			exec.Command("shutdown", "/s", "/t", "60").Run()
			os.Exit(0)
		}()
	}

	buttonChannel := make(chan string, 10)
	go internal.ListenJoysticks(buttonChannel)

	log.Println("✅ Sistema pronto. Aguardando cliques do joystick...")
	for buttonID := range buttonChannel {
		cam, existe := config.Quadras[buttonID]
		if !existe {
			log.Printf("⚠️ Botão %s não mapeado no config.json\n", buttonID)
			continue
		}
		go processClip(cam, config, autoUpload)
	}
}

func processClip(cam internal.Camera, config Config, autoUpload bool) {
	outName := internal.SaveClip(cam, config.SegundosGravacao)
	if outName == "" {
		return
	}

	if autoUpload {
		// ✅ Corrigido: Passando cam.Name como 4º argumento
		uploadVideo(outName, config.UploadURL, config.UploadToken, cam.Name)
	} else {
		handleManualUpload(outName, cam.Name, config)
	}
}

func handleManualUpload(filePath, quadra string, config Config) {
	fmt.Printf("\n🎬 Replay gerado na %s: %s\n", quadra, filepath.Base(filePath))
	fmt.Print("Deseja fazer upload para o site agora? (s/n): ")
	var res string
	fmt.Scanln(&res)

	if res == "s" || res == "S" {
		// ✅ Corrigido: Passando quadra como 4º argumento
		uploadVideo(filePath, config.UploadURL, config.UploadToken, quadra)
	} else {
		log.Printf("📁 Vídeo mantido apenas localmente: %s", filePath)
	}
}

func uploadVideo(filePath string, url string, token string, quadraName string) {
	file, err := os.Open(filePath)
	if err != nil {
		log.Printf("❌ Erro ao abrir arquivo: %v", err)
		return
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	writer.WriteField("token", token)
	writer.WriteField("dir", quadraName) // Envia "quadra_01", etc.

	part, _ := writer.CreateFormFile("video", filepath.Base(filePath))
	io.Copy(part, file)
	writer.Close()

	log.Printf("☁️ Enviando %s para %s...", filepath.Base(filePath), quadraName)

	req, _ := http.NewRequest("POST", url, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // Evita falso positivo de redirecionamento
		},
		Timeout: 5 * time.Minute,
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("❌ Falha de rede: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		log.Printf("✅ Upload concluído com sucesso!")
	} else {
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.Printf("⚠️ Erro no Servidor (%d): %s\n", resp.StatusCode, string(bodyBytes))
	}
}

func loadConfig() Config {
	configFile, err := os.ReadFile("config.json")
	if err != nil {
		log.Fatalf("❌ Erro ao ler config.json: %v", err)
	}
	var config Config
	json.Unmarshal(configFile, &config)
	return config
}
