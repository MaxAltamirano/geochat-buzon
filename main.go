package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
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
	http.HandleFunc("/api/ordenar", recibirMensajeExterno)
	http.HandleFunc("/api/upload_modular", recibirFragmentoModular) // <-- Esto elimina el error "unused"

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
func recibirFragmentoModular(w http.ResponseWriter, r *http.Request) {
	// 1. Identificar quién envía y qué parte
	idTarea := r.Header.Get("X-ID-Tarea")
	offsetStr := r.Header.Get("X-Offset")
	offset, _ := strconv.ParseInt(offsetStr, 10, 64)

	// 2. Ruta soberana donde se reconstruye el módulo
	rutaArchivo := fmt.Sprintf("./storage/tarea_%s.tmp", idTarea)

	// 3. Apertura inteligente: Si es nuevo (offset 0), crea; si es reintento, append
	flags := os.O_WRONLY | os.O_CREATE
	if offset > 0 {
		flags |= os.O_APPEND
	}
	f, err := os.OpenFile(rutaArchivo, flags, 0644)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer f.Close()

	// 4. Escribir el fragmento recibido
	n, err := io.Copy(f, r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// 5. Reportar al local: "Recibí X bytes, total acumulado: (offset + n)"
	w.Header().Set("X-Total-Recibido", fmt.Sprint(offset+n))
	w.WriteHeader(http.StatusAccepted)

}
