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

type Camera struct {
	ID       string `json:"-"`
	Name     string `json:"nome"`
	URL      string `json:"url"`
	TempDir  string `json:"-"`
	VideoDir string `json:"-"`
}

func getFFmpegPath() string {
	if exePath, err := os.Executable(); err == nil {
		local := filepath.Join(filepath.Dir(exePath), "ffmpeg.exe")
		if _, err := os.Stat(local); err == nil {
			return local
		}
	}
	return "ffmpeg"
}

// StartStream agora é NON-STOP: Se cair, reinicia automaticamente
func StartStream(cam Camera, stopChan chan bool) {
	camTemp := filepath.Join(cam.TempDir, cam.Name)
	os.MkdirAll(camTemp, 0755)

	// Cleanup goroutine tied to this stream's lifetime
	cleanupStop := make(chan bool)
	go cleanupOldChunks(camTemp, 60*time.Second, cleanupStop)

	// Cleanup will stop when StartStream returns
	defer close(cleanupStop)

	for {
		select {
		case <-stopChan:
			log.Printf("🛑 Parando captura da %s por timeout.", cam.Name)
			return
		default:
			args := []string{
				"-loglevel", "error",
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

			cmd := exec.Command(getFFmpegPath(), args...)
			if err := cmd.Start(); err != nil {
				log.Printf("❌ Falha na %s: %v. Tentando em 5s...", cam.Name, err)
				time.Sleep(5 * time.Second)
				continue
			}

			log.Printf("📹 [Capturando] %s", cam.Name)

			cmd.Wait() // Se o FFmpeg fechar, o loop continua e reinicia
			log.Printf("⚠️  Stream %s caiu. Reiniciando...", cam.Name)
			time.Sleep(3 * time.Second)
		}
	}
}

func SaveClip(cam Camera, duration int) string {
	camTemp, err := filepath.Abs(filepath.Join(cam.TempDir, cam.Name))
	if err != nil {
		log.Printf("❌ SaveClip: erro ao resolver camTemp: %v", err)
		return ""
	}

	files, err := os.ReadDir(camTemp)
	if err != nil {
		log.Printf("❌ SaveClip: erro ao ler %s: %v", camTemp, err)
		return ""
	}

	var tsFiles []os.DirEntry
	for _, f := range files {
		if filepath.Ext(f.Name()) == ".ts" {
			tsFiles = append(tsFiles, f)
		}
	}

	if len(tsFiles) < 2 {
		log.Printf("⚠️ Sem fragmentos suficientes para %s. Aguarde...", cam.Name)
		return ""
	}

	sort.Slice(tsFiles, func(i, j int) bool {
		return tsFiles[i].Name() < tsFiles[j].Name()
	})

	needed := (duration / 2) + 1
	if needed > len(tsFiles) {
		needed = len(tsFiles)
	}
	selected := tsFiles[len(tsFiles)-needed:]

	absOutDir, err := filepath.Abs(filepath.Join(cam.VideoDir, cam.Name))
	if err != nil {
		log.Printf("❌ SaveClip: erro ao resolver outDir: %v", err)
		return ""
	}
	os.MkdirAll(absOutDir, 0755)
	outName := filepath.Join(absOutDir, fmt.Sprintf("%s_%d.mp4", cam.Name, time.Now().Unix()))

	// Unique concat file per clip (like original)
	concatFile := filepath.Join(camTemp, fmt.Sprintf("concat_%d.txt", time.Now().UnixMilli()))
	f, err := os.Create(concatFile)
	if err != nil {
		log.Printf("❌ SaveClip: erro ao criar concat: %v", err)
		return ""
	}
	defer f.Close()
	defer os.Remove(concatFile) // Auto cleanup

	for _, ts := range selected {
		fmt.Fprintf(f, "file '%s'\n", ts.Name())
	}

	log.Printf("🎬 Compilando clipe da %s (~%ds)...", cam.Name, duration)

	cmd := exec.Command(
		getFFmpegPath(),
		"-loglevel", "error",
		"-y",
		"-f", "concat",
		"-safe", "0",
		"-i", filepath.Base(concatFile),
		"-c", "copy",
		outName,
	)
	cmd.Dir = camTemp

	if err := cmd.Run(); err != nil {
		log.Printf("❌ Erro ao salvar vídeo %s: %v", cam.Name, err)
		return ""
	}

	log.Printf("✅ Replay gerado: %s", outName)
	return outName
}

// cleanupOldChunks runs continuously during the stream lifetime, stops when stopChan closes
func cleanupOldChunks(dir string, maxAge time.Duration, stopChan chan bool) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-stopChan:
			log.Printf("🧹 Cleanup finalizado para %s", filepath.Base(dir))
			return
		case <-ticker.C:
			files, err := os.ReadDir(dir)
			if err != nil {
				continue
			}

			now := time.Now()
			deleted := 0
			for _, f := range files {
				if filepath.Ext(f.Name()) == ".ts" {
					if info, err := f.Info(); err == nil && now.Sub(info.ModTime()) > maxAge {
						if err := os.Remove(filepath.Join(dir, f.Name())); err == nil {
							deleted++
						}
					}
				}
			}
			if deleted > 0 {
				log.Printf("🧹 Limpou %d chunks antigos de %s", deleted, filepath.Base(dir))
			}
		}
	}
}
