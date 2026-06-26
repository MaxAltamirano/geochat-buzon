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
	Mensaje   string    `json:"mensaje"`
	Tipo      string    `json:"tipo"`
	Estado    string    `json:"estado"` // "PENDING", "PROCESSING", "DONE"
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"` // Para saber cuándo se bloqueó
}

const archivoPersistencia = "medula_local.json"

// --- VARIABLES GLOBALES DE ESTADO ---
var (
	mensajes = []MensajePendiente{}
	mu       sync.Mutex // Bloqueo para operaciones seguras

)

func main() {
	log.Println("🧬 MÉDULA LOCAL: Operando con persistencia en disco.")

	// Crea un nuevo Mux
	mux := http.NewServeMux()

	// --- RUTAS DE SALUD ---
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Córtex Buzón Online - Operativo"))
	})

	// --- RUTAS PROTEGIDAS POR CORS ---
	
	mux.HandleFunc("/api/purga", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		mensajes = []MensajePendiente{}
		guardarEnDisco(mensajes)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "buzon_limpio"})
		log.Println("🧹 [BUZÓN]: Purga ejecutada.")
	}))

	mux.HandleFunc("/api/enviar", corsMiddleware(recibirMensajeExterno))
	mux.HandleFunc("/api/sincronizar", corsMiddleware(vaciarCola))
	mux.HandleFunc("/api/ordenar", corsMiddleware(recibirMensajeExterno))
	mux.HandleFunc("/api/upload_modular", corsMiddleware(recibirFragmentoModular))

	mux.HandleFunc("/api/buzon/salida", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		lista := cargarDeDisco()
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(lista)
	}))

	mux.HandleFunc("/api/marcar_procesando", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.Atoi(r.URL.Query().Get("id"))
		mu.Lock()
		lista := cargarDeDisco()
		for i := range lista {
			if lista[i].ID == id {
				lista[i].Estado = "PROCESSING"
				lista[i].UpdatedAt = time.Now()
			}
		}
		guardarEnDisco(lista)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))

	// --- INICIALIZACIÓN DE SERVIDOR ---
	port := os.Getenv("PORT")
	if port == "" {
		port = "10000"
	}

	log.Printf("🚀 Córtex Buzón Online escuchando en :%s", port)

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
	// 1. Serializamos los datos
	datos, err := json.Marshal(mensajes)
	if err != nil {
		log.Printf("❌ [MÉDULA]: Error al marshalear datos: %v", err)
		return
	}

	// 2. Creamos un archivo temporal en la misma carpeta que el destino
	// El nombre .tmp nos asegura que no afectamos al archivo principal
	tmpFile := archivoPersistencia + ".tmp"

	// 3. Escribimos en el temporal
	err = ioutil.WriteFile(tmpFile, datos, 0644)
	if err != nil {
		log.Printf("❌ [MÉDULA]: Error escribiendo archivo temporal: %v", err)
		return
	}

	// 4. Renombramos el temporal al nombre original (Operación Atómica)
	// Esto garantiza que el cambio sea instantáneo y seguro
	err = os.Rename(tmpFile, archivoPersistencia)
	if err != nil {
		log.Printf("❌ [MÉDULA]: Error al realizar el rename atómico: %v", err)
		// Intentamos limpiar el temporal si algo salió mal
		os.Remove(tmpFile)
		return
	}

	log.Println("💾 [MÉDULA]: Estado guardado de forma atómica.")
}

func recibirMensajeExterno(w http.ResponseWriter, r *http.Request) {
	var m MensajePendiente
	// 1. Decodificación segura
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		log.Printf("❌ [MÉDULA]: Error decodificando payload: %v", err)
		return
	}

	// 2. Validación de Integridad (Soberanía de Datos)
	if m.Mensaje == "" {
		log.Printf("⚠️ [MÉDULA]: Intento de envío con mensaje vacío rechazado.")
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	// 3. Persistencia Segura (Capa de Bloqueo)
	mu.Lock()
	mensajes := cargarDeDisco()

	m.Estado = "PENDING_DELIVERY"
	m.CreatedAt = time.Now()
	m.ID = len(mensajes) + 1 // Asignación de ID basada en el estado actual

	mensajes = append(mensajes, m)
	guardarEnDisco(mensajes)
	mu.Unlock()

	// 4. Log de Observabilidad
	log.Printf("🔍 [DEBUG]: JSON decodificado -> ID: %d, Contenido: %.20s..., Tipo: '%s'", m.ID, m.Mensaje, m.Tipo)

	// 5. Feedback de Alineación Cognitiva
	if m.Tipo == "MODULAR" {
		log.Printf("🏗️ [MÉDULA-NODO]: Estructura modular recibida (ID #%d). Preparando compilación.", m.ID)
	} else {
		log.Printf("💬 [MÉDULA-NODO]: Respuesta literal registrada (ID #%d). Tipo: '%s'", m.ID, m.Tipo)
	}

	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte(fmt.Sprintf(`{"status":"success", "id":%d}`, m.ID)))
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

func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-ID-Tarea, X-Offset")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next(w, r)
	}
}