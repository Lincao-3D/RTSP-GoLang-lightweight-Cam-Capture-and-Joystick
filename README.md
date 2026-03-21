# Arena Cam Go

*en*🇺🇸 - *Continuous capture and instant replay recording system via RTSP, optimized for very low CPU/RAM consumption.*
*pt*🇧🇷 - Sistema de captura contínua e gravação de replay instantâneo via RTSP, otimizado para baixíssimo consumo de CPU/RAM.

## 🛠️ Requisitos de Sistema (Local)

1. **Golang:** Necessário para compilar o código.
2. **FFmpeg:** Este sistema exige que o executável `ffmpeg` esteja instalado e adicionado às variáveis de ambiente (PATH) do Windows.
   - **Onde baixar:** Faça o download das builds oficiais para Windows em [gyan.dev/ffmpeg/builds](https://www.gyan.dev/ffmpeg/builds/) (Baixe a versão `ffmpeg-git-full.7z` ou a `essentials`).
   - Preferivelmente, adicione o arquivo `ffmpeg.exe` à pasta raiz, pois o programa irá procurar primeiramente na raiz, caso contrário, utilizará o PATH do sistema.
   - Caso ainda não tenha ffmpeg no sistema, extraia o arquivo e adicione a pasta `bin` (onde o `ffmpeg.exe` está) à variável de ambiente `Path` do seu sistema Windows.
3. **Driver de Joystick:** O Windows deve reconhecer o encoder USB nativamente (verifique em "Configurar controladores de jogo USB").

## 🚀 Como Compilar

Abra o terminal no diretório do projeto e execute:
```bash
# Para compilar para Windows a partir do WSL ou Linux
GOOS=windows GOARCH=amd64 go build -o arena-cam.exe main.go

# Para um arquivo ainda menor, rode:
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o arena-cam-go.exe .
```
- Ícone: Para embutir o ícone, você deve ter um arquivo rsrc.syso (ou similar) na pasta, gerado a partir do seu icon_BR.ico - e, também, utilizar o "." no final do seu comando de build ao invés do arquivo `.go` (para o Go ler todo o conteúdo da pasta).
```bash
# use isto:
go build [flags] -o nome.exe .

# ao invés de 
go build [flags] main.go
```

## Estrutura Final de Pastas
Após o build, a distribuição para o cliente deve seguir este padrão:
```bash
Raiz/
├── arena-cam-go.exe     (Interface de logs e menu)
├── ffmpeg.exe           (Obrigatoriamente aqui para evitar erros de PATH)
├── config.json          (Configurações globais e tokens)
└── videos/
    ├── quadra_01/
    │   └── uploader.exe (Cópia do uploader manual)
    ├── quadra_02/
    │   └── uploader.exe
    └── ...
```

## ☁️ Configuração de Hospedagem (Hostinger / PHP)
Se os scripts PHP (upload.php, videos.php, etc.) forem hospedados em uma hospedagem compartilhada, será necessário ajustar as limitações do servidor, caso contrário os vídeos vão falhar ao subir.

## ⚠️ AVISO IMPORTANTE PARA HOSTINGER:
Hospedagens compartilhadas possuem limites estritos para upload de arquivos via PHP. Você precisará acessar o hPanel do cliente, ir até a seção Configuração PHP (ou PHP Options) e realizar as seguintes alterações:

Aumentar **upload_max_filesize** para pelo menos **256M**.

Aumentar **post_max_size** para pelo menos **256M**.

Aumentar **max_execution_time** para **300** (5 minutos) para garantir que os vídeos não sofram timeout enquanto estiverem sendo enviados através da internet da arena.

## ⚙️ Configuração (config.json)
Mantenha o **config.json** na mesma pasta do executável **arena-cam.exe**:

JSON
```bash
{
  "segundos_gravacao": 45,
  "pasta_videos": "videos",
  "pasta_temp": "temp_chunks",
  "upload_url": "[https://site-do-cliente.com/upload.php](https://site-do-cliente.com/upload.php)",
  "upload_token": "SEU_TOKEN_SECRETO_AQUI",
  "quadras": {
    "0": {
      "nome": "quadra1",
      "url": "rtsp://admin:admin@192.166.1.122:554/Streaming/Channels/101"
    }
  }
}
```
(A chave "0" refere-se ao ID do botão do Joystick).


This structure sets you up with a fast, scalable background daemon that avoids the pitfalls of trying to do real-time computer vision processing in a scripted language. Let me know if you need to fine-tune the FFmpeg parameters or tweak the HTTP upload behavior!