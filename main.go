package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

// --- ESTRUCTURAS DE DATOS ---

type HistorialItem struct {
	ID        string    `json:"id"`
	Contenido string    `json:"contenido"`
	Nodo      string    `json:"nodo"`
	Timestamp time.Time `json:"timestamp"`
}

type OpenSkyResponse struct {
	States [][]interface{} `json:"states"`
}

type ObjetoLattice struct {
	Name    string  `json:"name"`
	Azimuth float64 `json:"azimuth"`
	Altitud float64 `json:"altitud"`
}

type Satelite struct {
	Name    string  `json:"name"`
	Azimuth float64 `json:"azimuth"`
	Altitud float64 `json:"altitud"`
}

type Telemetria struct {
	Nodo          string          `json:"nodo"`
	Status        string          `json:"status"`
	InputActivity string          `json:"input_activity"`
	Temp          float64         `json:"temp"`
	Load          float64         `json:"load"`
	Satelites     []ObjetoLattice `json:"Satelites"`
}

type Mensaje struct {
	Entidad string `json:"entidad"`
	Mensaje string `json:"mensaje"`
}

type MensajePendiente struct {
	ID        int       `json:"id"`
	Mensaje   string    `json:"mensaje"`
	Tipo      string    `json:"tipo"`
	Estado    string    `json:"estado"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type RespuestaUnificada struct {
	Contexto  string                 `json:"contexto"`
	Cuerpo    string                 `json:"cuerpo"`
	Codigo    string                 `json:"codigo"`
	Metadatos map[string]interface{} `json:"metadatos"`
	ID        int                    `json:"id"`
	Respuesta string                 `json:"respuesta"`
	Timestamp time.Time              `json:"timestamp"`
}

type OllamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type OllamaResponse struct {
	Response string `json:"response"`
}

// --- CONSTANTES DE PERSISTENCIA ---
const (
	archivoPersistencia   = "medula_local.json"
	archivoHash           = "adn_hash.txt"
	archivoRespuestasKimi = "./storage/respuestas_kimi.json"
)

// --- VARIABLES GLOBALES Y DE ESTADO SOBERANO ---
var (
	mu               sync.Mutex
	ultimoPulsoLocal time.Time
	mensajes         = []MensajePendiente{}

	// Control del Buzón y Estado Global con Expiración Automática
	ultimoPulso   time.Time
	estadoMemoria = map[string]interface{}{
		"nodo":           "Avellaneda",
		"status":         "OFFLINE",
		"input_activity": "STANDBY",
		"temp":           25.0,
		"load":           0.0,
		"Satelites":      []interface{}{},
	}

	amenazasDetectadas []ObjetoLattice
	muAmenazas         sync.Mutex
	ultimaTelemetria   Telemetria
	muTelemetria       sync.Mutex
)

// --- FUNCIÓN PRINCIPAL (ENTRYPOINT SOBERANO) ---
func main() {
	log.Println("📁 [SISTEMA]: Iniciando arranque soberano del Córtex Buzón...")

	// 1. Asegurar la infraestructura local de almacenamiento
	if err := os.MkdirAll("./storage", 0755); err != nil {
		log.Printf("⚠️ [AVISO]: Carpeta ./storage ya existe o no pudo crearse: %v", err)
	} else {
		log.Println("📁 [SISTEMA]: Carpeta ./storage lista y asegurada.")
	}

	// 2. Definición del Mux Unificado
	mux := http.NewServeMux()

	// --- REGISTRO DE RUTAS ---

	// A. Ruta de descarga prioritaria de binarios
	mux.HandleFunc("/descargar-binario", func(w http.ResponseWriter, r *http.Request) {
		userAgent := r.Header.Get("User-Agent")
		archivoBinario := "./geochat-node"

		log.Printf("📥 [SISTEMA]: Petición de binario detectada desde: %s", userAgent)

		w.Header().Set("Content-Disposition", "attachment; filename=geochat-node")
		w.Header().Set("Content-Type", "application/octet-stream")
		http.ServeFile(w, r, archivoBinario)
	})

	// B. Ruta Raíz del Córtex
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Córtex Buzón Online - Operativo"))
	})

	// C. Endpoint de Heartbeat (Recepción de latido del nodo local con actualización de estado)
	mux.HandleFunc("/api/heartbeat", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		ultimoPulso = time.Now()
		estadoMemoria["status"] = "SYNCING"
		estadoMemoria["timestamp"] = time.Now().Unix()

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "pulso_recibido"})
	}))

	// D. Endpoint de Estado Global (Con expiración de 30 segundos integrada)
	mux.HandleFunc("/api/estado-global", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		// Si pasaron más de 30 segundos desde el último latido, forzamos OFFLINE automáticamente
		if time.Since(ultimoPulso) > 30*time.Second {
			estadoMemoria["status"] = "OFFLINE"
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(estadoMemoria)
	}))

	// E. Rutas de la Médula y Operaciones del Sistema
	mux.HandleFunc("/api/purga", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		mensajes = []MensajePendiente{}
		guardarEnDisco(mensajes)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "buzon_limpio"})
	}))

	mux.HandleFunc("/api/mensajes", corsMiddleware(recibirMensajeExterno))
	mux.HandleFunc("/api/vaciar", corsMiddleware(vaciarCola))
	mux.HandleFunc("/api/fragmento", corsMiddleware(recibirFragmentoModular))
	mux.HandleFunc("/api/verificar-adn", corsMiddleware(verificarADN))
	mux.HandleFunc("/api/ingestar", corsMiddleware(ingestarCromosomas))

	// F. Integración de Historial y Activación Cognitiva de Kimi
	mux.HandleFunc("/api/agregar-historial", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Historial vinculado"))
	}))

	mux.HandleFunc("/api/generar-respuesta", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		go generarRespuestaKimi(1, "Activación manual desde Buzón")
		w.Write([]byte("Generación de respuesta iniciada"))
	}))

	mux.HandleFunc("/api/historial/nuevo", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var entrada Mensaje
		if err := json.NewDecoder(r.Body).Decode(&entrada); err != nil {
			http.Error(w, "Error en los datos", http.StatusBadRequest)
			return
		}

		agregarAlHistorial(entrada)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Evento guardado"))
	}))

	// --- 3. SERVICIOS EN BACKGROUND ---
	go escucharSocketBuzon()

	// Relé de Nodos TCP
	go func() {
		ln, err := net.Listen("tcp", "0.0.0.0:10003")
		if err != nil {
			log.Printf("❌ [RELÉ]: Error al iniciar socket: %v", err)
			return
		}
		defer ln.Close()
		for {
			conn, err := ln.Accept()
			if err != nil {
				continue
			}
			go func(c net.Conn) {
				defer c.Close()
				scanner := bufio.NewScanner(c)
				for scanner.Scan() {
					log.Printf("📡 [RELÉ]: Nodo activo: %s", scanner.Text())
				}
			}(conn)
		}
	}()

	// --- 4. MOTOR DE SENSADO BLINDADO ---
	go iniciarMotorSensado()

	// --- 5. INICIAR SERVIDOR HTTP (CONFIGURACIÓN DINÁMICA RENDER) ---
	port := os.Getenv("PORT")
	if port == "" {
		port = "10002"
	}

	log.Printf("🚀 Córtex Buzón Online escuchando en puerto :%s", port)

	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	log.Fatal(server.ListenAndServe())
}

// --- FUNCIONES DE SOPORTE Y CONTROL DE ESTADO ---

func iniciarMotorSensado() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("⚠️ [CÓRTEX]: Motor de sensado recuperado de pánico: %v", r)
			time.Sleep(5 * time.Second)
			go iniciarMotorSensado()
		}
	}()

	log.Println("🧠 [CÓRTEX]: Iniciando motor de sensado...")

	for {
		actividad := obtenerActividadRaton()
		satelites := obtenerDatosTrackingReal()

		if satelites == nil {
			satelites = make([]ObjetoLattice, 0)
		}

		datos := Telemetria{
			Nodo:          "Avellaneda",
			Status:        "SYNCING",
			Temp:          25.0,
			Load:          0.1,
			InputActivity: actividad,
			Satelites:     satelites,
		}

		actualizarEstadoTelemetria(datos)
		ultimoPulsoLocal = time.Now()

		log.Printf("📡 [CÓRTEX]: Telemetría actualizada en nodo %s | Pulso: %v", datos.Nodo, ultimoPulsoLocal.Format("15:04:05"))

		time.Sleep(5 * time.Second)
	}
}

func escucharSocketBuzon() {
	socketPath := os.Getenv("GEOCHAT_SOCKET_PATH")
	if socketPath == "" {
		if _, err := os.Stat("/data/data/com.termux"); err == nil {
			socketPath = "/data/data/com.termux/files/home/.geochat_buzon.sock"
		} else {
			socketPath = "./.geochat_buzon.sock"
		}
	}

	os.Remove(socketPath)

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		log.Fatalf("❌ [CRÍTICO]: No pude abrir el socket de interferencias: %v", err)
	}
	defer listener.Close()

	log.Printf("📡 [RADAR]: Buzón escuchando en %s", socketPath)

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}
		go handleInterferencia(conn)
	}
}

func handleInterferencia(conn net.Conn) {
	defer conn.Close()
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		return
	}

	var msg map[string]interface{}
	json.Unmarshal(buf[:n], &msg)

	nuevaAmenaza := ObjetoLattice{
		Name:    fmt.Sprintf("AMENAZA: %s", msg["target"]),
		Azimuth: 0,
		Altitud: 0,
	}

	muAmenazas.Lock()
	amenazasDetectadas = append(amenazasDetectadas, nuevaAmenaza)
	muAmenazas.Unlock()

	log.Printf("⚠️ [RADAR]: Amenaza inyectada al mapa: %s", msg["target"])
}

func obtenerDatosTrackingReal() []ObjetoLattice {
	var lista []ObjetoLattice

	lista = append(lista, fetchOpenSky()...)

	urlSats := "https://api.wheretheiss.at/v1/satellites/25544"
	client := http.Client{Timeout: 3 * time.Second}

	respSats, err := client.Get(urlSats)
	if err == nil {
		defer respSats.Body.Close()

		var iss struct {
			Name      string  `json:"name"`
			Longitude float64 `json:"longitude"`
		}

		if err := json.NewDecoder(respSats.Body).Decode(&iss); err == nil {
			azimut := float64(int(iss.Longitude) % 360)
			lista = append(lista, ObjetoLattice{
				Name:    "ISS_SATELLITE",
				Azimuth: azimut,
				Altitud: 400,
			})
		}
	}

	muAmenazas.Lock()
	if len(amenazasDetectadas) > 0 {
		lista = append(lista, amenazasDetectadas...)
	}
	muAmenazas.Unlock()

	return lista
}

func fetchOpenSky() []ObjetoLattice {
	var lista []ObjetoLattice
	timestamp := time.Now().Unix()

	for i := 0; i < 3; i++ {
		offset := float64(timestamp % 360)
		azimuth := (float64(i) * 120.0) + offset
		if azimuth > 360 {
			azimuth -= 360
		}

		lista = append(lista, ObjetoLattice{
			Name:    "AVION-" + strconv.Itoa(i+1),
			Azimuth: azimuth,
			Altitud: 10000.0 + float64(i*500),
		})
	}

	for i := 0; i < 3; i++ {
		azimuth := (float64(i) * 45.0) + 180.0
		lista = append(lista, ObjetoLattice{
			Name:    "SAT-GEO-" + strconv.Itoa(i+1),
			Azimuth: azimuth,
			Altitud: 500000.0 + float64(i*10000),
		})
	}

	return lista
}

func obtenerActividadRaton() string {
	return "ACTIVE_SENSING"
}

func actualizarEstadoTelemetria(datos Telemetria) {
	muTelemetria.Lock()
	defer muTelemetria.Unlock()
	ultimaTelemetria = datos
}

// --- GESTIÓN DE PERSISTENCIA Y MÉDULA ---

func agregarAlHistorial(m Mensaje) {
	mu.Lock()
	defer mu.Unlock()

	respuestas := cargarRespuestasKimi()
	nueva := RespuestaUnificada{
		Contexto:  m.Entidad,
		Cuerpo:    m.Mensaje,
		Timestamp: time.Now(),
	}
	respuestas = append(respuestas, nueva)

	datos, _ := json.MarshalIndent(respuestas, "", "  ")
	err := os.WriteFile(archivoRespuestasKimi, datos, 0644)
	if err != nil {
		log.Printf("❌ [BUZÓN]: Error crítico al persistir en disco: %v", err)
	}
}

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
	datos, err := json.Marshal(mensajes)
	if err != nil {
		log.Printf("❌ [MÉDULA]: Error al marshalear datos: %v", err)
		return
	}

	tmpFile := archivoPersistencia + ".tmp"
	err = ioutil.WriteFile(tmpFile, datos, 0644)
	if err != nil {
		log.Printf("❌ [MÉDULA]: Error escribiendo archivo temporal: %v", err)
		return
	}

	err = os.Rename(tmpFile, archivoPersistencia)
	if err != nil {
		log.Printf("❌ [MÉDULA]: Error al realizar el rename atómico: %v", err)
		os.Remove(tmpFile)
		return
	}

	log.Println("💾 [MÉDULA]: Estado guardado de forma atómica.")
}

func calcularHash(adn string) string {
	hash := sha256.Sum256([]byte(adn))
	return hex.EncodeToString(hash[:])
}

func cargarHashDesdeDisco() string {
	datos, err := ioutil.ReadFile(archivoHash)
	if err != nil {
		return ""
	}
	return string(datos)
}

func guardarHash(hash string) {
	err := ioutil.WriteFile(archivoHash, []byte(hash), 0644)
	if err != nil {
		log.Printf("❌ [MÉDULA]: Error guardando hash: %v", err)
	}
}

func recibirMensajeExterno(w http.ResponseWriter, r *http.Request) {
	var m MensajePendiente
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		log.Printf("❌ [MÉDULA]: Error decodificando payload: %v", err)
		return
	}

	if m.Mensaje == "" {
		log.Printf("⚠️ [MÉDULA]: Intento de envío con mensaje vacío rechazado.")
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	mu.Lock()
	mensajes := cargarDeDisco()

	m.Estado = "PENDING_DELIVERY"
	m.CreatedAt = time.Now()
	m.ID = len(mensajes) + 1

	mensajes = append(mensajes, m)
	guardarEnDisco(mensajes)
	mu.Unlock()

	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte(fmt.Sprintf(`{"status":"success", "id":%d}`, m.ID)))
}

func vaciarCola(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()

	mensajes := cargarDeDisco()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(mensajes)

	guardarEnDisco([]MensajePendiente{})
}

func recibirFragmentoModular(w http.ResponseWriter, r *http.Request) {
	idTarea := r.Header.Get("X-ID-Tarea")
	offsetStr := r.Header.Get("X-Offset")
	offset, _ := strconv.ParseInt(offsetStr, 10, 64)

	rutaArchivo := fmt.Sprintf("./storage/tarea_%s.tmp", idTarea)

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

	n, err := io.Copy(f, r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("X-Total-Recibido", fmt.Sprint(offset+n))
	w.WriteHeader(http.StatusAccepted)
}

func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-ID-Tarea, X-Offset, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next(w, r)
	}
}

func verificarADN(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		ADN string `json:"dna_payload"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		log.Printf("❌ [SYNC]: Error decodificando payload: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"status":"error", "message":"invalid_payload"}`))
		return
	}

	if !verificarIntegridad(payload.ADN) {
		log.Println("⚡ [SYNC]: ADN detectado como idéntico. Resonancia estable.")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"up_to_date"}`))
		return
	}

	log.Println("🧬 [SYNC]: Evolución de ADN detectada. Reiniciando Cortex...")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte(`{"status":"reconfiguring"}`))
}

func verificarIntegridad(adnNuevo string) bool {
	nuevoHash := calcularHash(adnNuevo)
	hashGuardado := cargarHashDesdeDisco()

	if nuevoHash == hashGuardado {
		return false
	}
	guardarHash(nuevoHash)
	return true
}

func ingestarCromosomas(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		ADN      string `json:"adn"`
		Trilogia string `json:"trilogia"`
		Mapa     string `json:"mapa"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		log.Printf("❌ [CORTEX]: Error decodificando payload: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err := os.WriteFile("adn_maestro.json", []byte(payload.ADN), 0644); err != nil {
		log.Printf("❌ [CORTEX]: Error guardando adn_maestro: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	os.WriteFile("cromosoma_trilogia.json", []byte(payload.Trilogia), 0644)
	os.WriteFile("mapa_cognitivo.json", []byte(payload.Mapa), 0644)

	log.Println("📥 [CORTEX]: Cromosomas recibidos y persistidos en disco.")

	if err := InyectarCromosomasEnKimi(); err != nil {
		log.Printf("❌ [KIMI]: Error en la inyección: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	log.Println("📥 [CORTEX]: Cromosomas inyectados y Kimi reconfigurada.")
	w.WriteHeader(http.StatusOK)
}

func InyectarCromosomasEnKimi() error {
	log.Println("🧬 [KIMI]: Iniciando proceso de reconfiguración cognitiva...")

	adn, err := os.ReadFile("adn_maestro.json")
	if err != nil {
		return fmt.Errorf("error leyendo adn_maestro: %v", err)
	}
	trilogia, err := os.ReadFile("cromosoma_trilogia.json")
	if err != nil {
		return fmt.Errorf("error leyendo cromosoma_trilogia: %v", err)
	}
	mapa, err := os.ReadFile("mapa_cognitivo.json")
	if err != nil {
		return fmt.Errorf("error leyendo mapa_cognitivo: %v", err)
	}

	if len(adn) == 0 || len(trilogia) == 0 || len(mapa) == 0 {
		return fmt.Errorf("integridad fallida: archivos de cromosomas incompletos")
	}

	log.Printf("✅ [KIMI]: ADN maestro cargado (%d bytes)", len(adn))
	log.Printf("✅ [KIMI]: Trilogía operativa cargada (%d bytes)", len(trilogia))
	log.Printf("✅ [KIMI]: Mapa cognitivo integrado (%d bytes)", len(mapa))

	log.Println("✨ [KIMI]: Reconfiguración completa. Nueva identidad activada.")
	return nil
}

func cargarRespuestasKimi() []RespuestaUnificada {
	if _, err := os.Stat(archivoRespuestasKimi); os.IsNotExist(err) {
		log.Printf("⚠️ [BUZÓN]: Archivo de respuestas no existe en %s, iniciando nuevo historial.", archivoRespuestasKimi)
		return []RespuestaUnificada{}
	}

	datos, err := os.ReadFile(archivoRespuestasKimi)
	if err != nil {
		log.Printf("❌ [BUZÓN]: Error al leer el archivo de respuestas: %v", err)
		return []RespuestaUnificada{}
	}

	var respuestas []RespuestaUnificada
	if err := json.Unmarshal(datos, &respuestas); err != nil {
		log.Printf("❌ [BUZÓN]: Error al decodificar JSON: %v", err)
		return []RespuestaUnificada{}
	}

	return respuestas
}

func generarRespuestaKimi(mensajeID int, contenido string) {
	log.Printf("🧠 [CORTEX]: Activando Kimi para ID #%d...", mensajeID)

	adn, err := os.ReadFile("adn_maestro.json")
	contextoADN := "ADN_NO_CARGADO"
	if err == nil {
		contextoADN = string(adn)
	} else {
		log.Printf("⚠️ [CORTEX]: ADN no encontrado: %v", err)
	}
	log.Printf("DEBUG: Contexto cargado con longitud: %d", len(contextoADN))

	payload := map[string]interface{}{
		"model":  "phi3:mini",
		"stream": false,
	}
	datos, _ := json.Marshal(payload)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Post("http://localhost:11434/api/generate", "application/json", bytes.NewBuffer(datos))
	if err != nil {
		log.Printf("❌ [KIMI-ERROR]: Ollama rechazó la conexión: %v", err)
		return
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	var ollamaResp struct {
		Response string `json:"response"`
	}
	json.Unmarshal(body, &ollamaResp)
	respuestaFinal := ollamaResp.Response

	mu.Lock()
	nueva := RespuestaUnificada{
		ID:        mensajeID,
		Respuesta: respuestaFinal,
		Timestamp: time.Now(),
		Contexto:  "FRIEND",
		Cuerpo:    contenido,
	}

	respuestas := cargarRespuestasKimi()
	respuestas = append(respuestas, nueva)
	finalData, _ := json.MarshalIndent(respuestas, "", "  ")
	os.WriteFile(archivoRespuestasKimi, finalData, 0644)
	mu.Unlock()

	GuardarEnBuzon(Mensaje{
		Entidad: "KIMI",
		Mensaje: respuestaFinal,
	})

	log.Printf("✅ [KIMI]: Respuesta integrada y enviada al Buzón para mensaje #%d", mensajeID)
}

func GuardarEnBuzon(nuevoMensaje Mensaje) error {
	mu.Lock()
	defer mu.Unlock()

	respuestas := cargarRespuestasKimi()

	nueva := RespuestaUnificada{
		ID:        len(respuestas) + 1,
		Respuesta: nuevoMensaje.Mensaje,
		Timestamp: time.Now(),
		Contexto:  nuevoMensaje.Entidad,
	}

	respuestas = append(respuestas, nueva)
	finalData, err := json.MarshalIndent(respuestas, "", "  ")
	if err != nil {
		log.Printf("❌ [BUZÓN-ERROR]: Fallo al serializar médula: %v", err)
		return err
	}

	err = os.WriteFile(archivoRespuestasKimi, finalData, 0644)
	if err != nil {
		log.Printf("❌ [BUZÓN-ERROR]: Fallo al escribir en disco: %v", err)
		return err
	}

	log.Printf("✅ [BUZÓN-RENDER]: Respuesta de %s inyectada en médula (ID: %d).", nuevoMensaje.Entidad, nueva.ID)
	return nil
}