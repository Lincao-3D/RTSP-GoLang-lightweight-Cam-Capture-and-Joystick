package internal

import (
	"log"
	"strconv"
	"time"

	"github.com/simulatedsimian/joystick"
)

// ListenJoysticks polls the gamepad and sends the pressed button ID to a channel
func ListenJoysticks(buttonChannel chan<- string) {
	// Usually, the primary USB encoder is joystick 0
	js, err := joystick.Open(0)
	if err != nil {
		log.Println("⚠️ Nenhum joystick encontrado! Verifique a conexão USB.")
		// We don't return here immediately in case it gets plugged in later, 
		// but for simplicity, we'll loop until it's found.
		for err != nil {
			time.Sleep(5 * time.Second)
			js, err = joystick.Open(0)
		}
		log.Println("✅ Joystick conectado com sucesso!")
	}
	defer js.Close()

	buttonCount := js.ButtonCount()
	log.Printf("🎮 Joystick detectado com %d botões.\n", buttonCount)

	// Keep track of the previous state so we only trigger once per press, not while holding
	previousState := uint32(0)

	for {
		state, err := js.Read()
		if err != nil {
			log.Println("❌ Erro ao ler joystick. Tentando reconectar...")
			time.Sleep(2 * time.Second)
			js, _ = joystick.Open(0)
			continue
		}

		// state.Buttons is a bitmask where each bit represents a button
		for i := 0; i < buttonCount; i++ {
			// Check if the i-th bit is set
			isPressed := (state.Buttons & (1 << i)) != 0
			wasPressed := (previousState & (1 << i)) != 0

			// Trigger only on the exact moment it is pressed down
			if isPressed && !wasPressed {
				// We convert the button index to string to match the JSON keys (e.g., "11")
				buttonID := strconv.Itoa(i)
				log.Printf("🕹️ Botão pressionado: %s\n", buttonID)
				
				// Send the button ID to our main app non-blockingly
				select {
				case buttonChannel <- buttonID:
				default:
					log.Println("⚠️ Canal ocupado, ignorando clique duplo.")
				}
			}
		}

		previousState = state.Buttons
		// Poll every 50ms (20 times a second is plenty fast for human reaction)
		time.Sleep(50 * time.Millisecond)
	}
}