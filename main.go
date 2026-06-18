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
	Mensaje   string    `json:"mensaje"` // Renombrado de Contenido a Mensaje para coincidir con Vue
	Tipo      string    `json:"tipo"`    // "KIMI" o "USUARIO"
	Estado    string    `json:"estado"`
	CreatedAt time.Time `json:"created_at"`
}

const archivoPersistencia = "medula_local.json"
// --- VARIABLES GLOBALES DE ESTADO ---
var (
	mensajes = []MensajePendiente{}
	mu       sync.Mutex // Bloqueo para operaciones seguras

)


func main() {
	log.Println("🧬 MÉDULA LOCAL: Operando con persistencia en disco.")

	// Crea un nuevo Mux en lugar de usar el default
	mux := http.NewServeMux()

	// 1. Rutas del Buzón
	// En el servidor del Buzón (Render)
	// En main.go (Buzón)
	mux.HandleFunc("/api/purga", func(w http.ResponseWriter, r *http.Request) {
        mu.Lock() // Bloqueamos para evitar conflictos
        mensajes = []MensajePendiente{} // Limpiamos memoria
        guardarEnDisco(mensajes)        // ¡ESTO ES LO QUE TE FALTABA! Limpiamos el archivo físico
        mu.Unlock()                     // Liberamos
        
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]string{"status": "buzon_limpio"})
        log.Println("🧹 [BUZÓN]: Purga ejecutada y disco sincronizado.")
    })
	mux.HandleFunc("/api/enviar", recibirMensajeExterno)
	mux.HandleFunc("/api/sincronizar", vaciarCola)
	mux.HandleFunc("/api/ordenar", recibirMensajeExterno)
	mux.HandleFunc("/api/upload_modular", recibirFragmentoModular)

	// 2. Ruta de Salud (Crucial para que Render no tire 404)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Córtex Buzón Online - Operativo"))
	})

	// 3. Ruta de Salida
	mux.HandleFunc("/api/buzon/salida", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		mensajes := cargarDeDisco()
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mensajes)
	})

	// Iniciar servidor
	port := os.Getenv("PORT")
	if port == "" {
		port = "10000"
	}

	log.Printf("🚀 Córtex Buzón Online escuchando en :%s", port)

	// Escuchar en 0.0.0.0 es obligatorio en entornos cloud como Render
	server := &http.Server{
		Addr:    "0.0.0.0:" + port,
		Handler: mux,
	}
	log.Fatal(server.ListenAndServe())
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
		log.Printf("❌ [MÉDULA]: Error decodificando payload: %v", err)
		return
	}

	// --- LOG DE DEPURACIÓN (Capa de Observabilidad) ---
	// Esto nos dirá exactamente si el JSON llega con el campo "tipo" lleno o vacío
	log.Printf("🔍 [DEBUG]: JSON decodificado -> ID: %d, Contenido: %.20s..., Tipo recibido: '%s'", m.ID, m.Mensaje, m.Tipo)
	// --------------------------------------------------

	mu.Lock()
	mensajes := cargarDeDisco()
	m.Estado = "PENDING_DELIVERY"
	m.CreatedAt = time.Now()
	m.ID = len(mensajes) + 1
	mensajes = append(mensajes, m)
	guardarEnDisco(mensajes)
	mu.Unlock()

	// Lógica de Alineación Cognitiva
	if m.Tipo == "MODULAR" {
		log.Printf("🏗️ [MÉDULA-NODO]: Estructura modular recibida (ID #%d). Preparando para compilación.", m.ID)
	} else {
		log.Printf("💬 [MÉDULA-NODO]: Respuesta literal registrada (ID #%d). Tipo detectado: '%s'", m.ID, m.Tipo)
	}

	w.WriteHeader(http.StatusAccepted)
}

func vaciarCola(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()

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
