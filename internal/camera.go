package internal

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"time"
)

// Camera represents a single court's stream configuration
type Camera struct {
	ID       string
	Name     string
	URL      string
	TempDir  string
	VideoDir string
}

// StartStream launches a background FFmpeg process to capture 2s rolling chunks
func StartStream(cam Camera) {
	// Ensure temp directory exists
	camTemp := filepath.Join(cam.TempDir, cam.Name)
	os.MkdirAll(camTemp, 0755)

	// FFmpeg command: read RTSP, copy stream (no encode), split into 2s .ts chunks
	args := []string{
		"-y",
		"-rtsp_transport", "tcp",
		"-i", cam.URL,
		"-c", "copy",
		"-f", "segment",
		"-segment_time", "2",
		"-segment_format", "ts",
		"-reset_timestamps", "1",
		filepath.Join(camTemp, "chunk_%05d.ts"),
	}

	cmd := exec.Command("ffmpeg", args...)

	err := cmd.Start()
	if err != nil {
		log.Printf("[Erro] Falha ao iniciar stream da %s: %v\n", cam.Name, err)
		return
	}

	log.Printf("🎥 Captura contínua iniciada para: %s\n", cam.Name)

	// Start background cleanup to prevent disk fill-up (keeps only the last ~60 seconds)
	go cleanupOldChunks(camTemp, 60*time.Second)

	// Wait will block until the FFmpeg process crashes or stops
	cmd.Wait()
	log.Printf("⚠️ Stream da %s caiu ou foi finalizado.\n", cam.Name)
}

// old: SaveClip is called when the joystick button is pressed
// SaveClip agora retorna uma string (o caminho do arquivo salvo)
func SaveClip(cam Camera, clipDuration int) string {
	camTemp := filepath.Join(cam.TempDir, cam.Name)

	files, err := os.ReadDir(camTemp)
	if err != nil {
		log.Printf("[Erro] Não foi possível ler a pasta temp da %s: %v\n", cam.Name, err)
		return ""
	}

	var tsFiles []os.DirEntry
	for _, f := range files {
		if filepath.Ext(f.Name()) == ".ts" {
			tsFiles = append(tsFiles, f)
		}
	}

	if len(tsFiles) < 2 {
		log.Printf("⚠️ Sem fragmentos suficientes para %s. Aguarde...\n", cam.Name)
		return ""
	}

	sort.Slice(tsFiles, func(i, j int) bool {
		return tsFiles[i].Name() < tsFiles[j].Name()
	})

	chunksNeeded := (clipDuration / 2) + 1
	if chunksNeeded > len(tsFiles) {
		chunksNeeded = len(tsFiles)
	}

	selectedFiles := tsFiles[len(tsFiles)-chunksNeeded:]

	concatFileName := fmt.Sprintf("concat_%d.txt", time.Now().UnixMilli())
	concatFilePath := filepath.Join(camTemp, concatFileName)
	f, err := os.Create(concatFilePath)
	if err != nil {
		return ""
	}

	for _, ts := range selectedFiles {
		f.WriteString(fmt.Sprintf("file '%s'\n", ts.Name()))
	}
	f.Close()
	defer os.Remove(concatFilePath)

	timestamp := time.Now().Unix()
	outDir := filepath.Join(cam.VideoDir, cam.Name)
	os.MkdirAll(outDir, 0755)
	outName := filepath.Join(outDir, fmt.Sprintf("%s_%d.mp4", cam.Name, timestamp))

	log.Printf("🎬 Compilando clipe da %s (~%ds)...\n", cam.Name, clipDuration)

	concatCmd := exec.Command("ffmpeg", "-y", "-f", "concat", "-safe", "0", "-i", concatFileName, "-c", "copy", outName)
	concatCmd.Dir = camTemp

	err = concatCmd.Run()
	if err != nil {
		log.Printf("❌ Erro ao salvar vídeo %s: %v\n", cam.Name, err)
		return ""
	}

	log.Printf("✅ Vídeo salvo: %s\n", outName)
	return outName // <--- IMPORTANTE: Retornamos o caminho aqui
}

// cleanupOldChunks continuously runs in the background deleting files older than maxAge
func cleanupOldChunks(dir string, maxAge time.Duration) {
	for {
		time.Sleep(10 * time.Second) // Check every 10 seconds
		files, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		now := time.Now()
		for _, f := range files {
			if filepath.Ext(f.Name()) == ".ts" {
				info, err := f.Info()
				if err == nil && now.Sub(info.ModTime()) > maxAge {
					os.Remove(filepath.Join(dir, f.Name()))
				}
			}
		}
	}
}
