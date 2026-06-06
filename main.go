package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

// Médula: Estructura de mensajes para el estado persistente
type MensajePendiente struct {
	ID        int       `json:"id"`
	Contenido string    `json:"contenido"`
	Estado    string    `json:"estado"`
	CreatedAt time.Time `json:"created_at"`
}

const archivoPersistencia = "medula_local.json"

var (
	mutex sync.Mutex
)

func main() {
	log.Println("🧬 MÉDULA LOCAL: Operando con persistencia en disco.")

	// 2. Rutas del Buzón
	http.HandleFunc("/api/enviar", recibirMensajeExterno)
	http.HandleFunc("/api/sincronizar", vaciarCola)

	// Iniciar servidor
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("🚀 Córtex Buzón Online (Persistencia Local) en puerto %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

// Funciones auxiliares de persistencia
func cargarDeDisco() []MensajePendiente {
	if _, err := os.Stat(archivoPersistencia); os.IsNotExist(err) {
		return []MensajePendiente{}
	}
	datos, err := ioutil.ReadFile(archivoPersistencia)
	if err != nil {
		return []MensajePendiente{}
	}
	var mensajes []MensajePendiente
	json.Unmarshal(datos, &mensajes)
	return mensajes
}

func guardarEnDisco(mensajes []MensajePendiente) {
	datos, _ := json.Marshal(mensajes)
	ioutil.WriteFile(archivoPersistencia, datos, 0644)
}

func recibirMensajeExterno(w http.ResponseWriter, r *http.Request) {
	var m MensajePendiente
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	mutex.Lock()
	mensajes := cargarDeDisco()
	m.Estado = "PENDING_DELIVERY"
	m.CreatedAt = time.Now()
	// Asignación simple de ID basada en el largo del slice para este modo local
	m.ID = len(mensajes) + 1
	mensajes = append(mensajes, m)
	guardarEnDisco(mensajes)
	mutex.Unlock()

	w.WriteHeader(http.StatusAccepted)
}

func vaciarCola(w http.ResponseWriter, r *http.Request) {
	mutex.Lock()
	defer mutex.Unlock()

	mensajes := cargarDeDisco()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(mensajes)

	// Limpiamos el archivo tras el handshake
	guardarEnDisco([]MensajePendiente{})
}
