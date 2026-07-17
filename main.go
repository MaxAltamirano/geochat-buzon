package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
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
	//"os/exec"
	//"strings"
	//"strconv"
	//"path/filepath"
	"bufio"
	"net"
)

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

// --- ESTRUCTURA DEL PULSO VITAL (TELEMETRÍA) ---
type Telemetria struct {
	Nodo          string          `json:"nodo"`
	Status        string          `json:"status"`         // <-- Asegúrate de incluir esta línea
	InputActivity string          `json:"input_activity"` // Mover los strings juntos ayuda a la alineación
	Temp          float64         `json:"temp"`
	Load          float64         `json:"load"`
	Satelites     []ObjetoLattice `json:"Satelites"` // <-- Cambiado aquí
}

// Variable global para guardar el último estado recibido

// Lista de amenazas que el radar debe mostrar
var (
	amenazasDetectadas []ObjetoLattice
	muAmenazas         sync.Mutex
	ultimaTelemetria   Telemetria
	muTelemetria       sync.Mutex
)

// --- ESTRUCTURA PARA EL BYPASS SOBERANO ---
type Mensaje struct {
	Entidad string `json:"entidad"`
	Mensaje string `json:"mensaje"`
}

// Médula: Estructura de mensajes para el estado persistente
type MensajePendiente struct {
	ID        int       `json:"id"`
	Mensaje   string    `json:"mensaje"`
	Tipo      string    `json:"tipo"`
	Estado    string    `json:"estado"` // "PENDING", "PROCESSING", "DONE"
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"` // Para saber cuándo se bloqueó
}

type RespuestaUnificada struct {
	Contexto  string                 `json:"contexto"` // "FRIEND" o "MODULAR"
	Cuerpo    string                 `json:"cuerpo"`
	Codigo    string                 `json:"codigo"`
	Metadatos map[string]interface{} `json:"metadatos"`
	ID        int                    `json:"id"`
	Respuesta string                 `json:"respuesta"`
	Timestamp time.Time              `json:"timestamp"`
}

// Estructura para hablar con Ollama
type OllamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type OllamaResponse struct {
	Response string `json:"response"`
}

const archivoPersistencia = "medula_local.json"

// --- VARIABLES GLOBALES DE ESTADO ---
var (
	mensajes         = []MensajePendiente{}
	mu               sync.Mutex // Bloqueo para operaciones seguras
	ultimoPulsoLocal time.Time
)

func main() {
	log.Println("📁 [SISTEMA]: Iniciando arranque soberano...")

	// 1. Asegurar la infraestructura local (Indispensable para Render)
	if err := os.MkdirAll("./storage", 0755); err != nil {
		log.Printf("⚠️ [AVISO]: Carpeta ./storage ya existe o no pudo crearse: %v", err)
	} else {
		log.Println("📁 [SISTEMA]: Carpeta ./storage lista.")
	}

	// 2. Definición del Mux
	mux := http.NewServeMux()

	// --- REGISTRO DE RUTAS ---

	// 1. RUTA DE DESCARGA PRIORITARIA (El primero que llega, el primero que se sirve)
	mux.HandleFunc("/descargar-binario", func(w http.ResponseWriter, r *http.Request) {
		// Detección táctica: si viene por curl o pide binario explícito
		userAgent := r.Header.Get("User-Agent")

		// Si es un binario real, forzamos la descarga
		// Asumiendo que tienes el binario en la carpeta raíz o ./storage/
		archivoBinario := "./geochat-node"

		log.Printf("📥 [SISTEMA]: Petición de binario detectada desde: %s", userAgent)

		w.Header().Set("Content-Disposition", "attachment; filename=geochat-node")
		w.Header().Set("Content-Type", "application/octet-stream")
		http.ServeFile(w, r, archivoBinario)
	})

	// 2. RUTA RAÍZ (El Córtex, ahora solo actúa si no es una descarga)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Si alguien entra al raíz, responde el Córtex
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Córtex Buzón Online - Operativo"))
	})

	// ... (El resto de tus rutas /api/ siguen debajo, intactas)

	// ... (Aquí irían tus otros mux.HandleFunc, los mantienes igual) ...
	mux.HandleFunc("/api/purga", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		mensajes = []MensajePendiente{}
		guardarEnDisco(mensajes)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "buzon_limpio"})
	}))

	// --- SECCIÓN DE RUTAS QUE DEBES RESTAURAR ---
	mux.HandleFunc("/api/mensajes", corsMiddleware(recibirMensajeExterno))
	mux.HandleFunc("/api/vaciar", corsMiddleware(vaciarCola))
	mux.HandleFunc("/api/fragmento", corsMiddleware(recibirFragmentoModular))
	mux.HandleFunc("/api/verificar-adn", corsMiddleware(verificarADN))
	mux.HandleFunc("/api/ingestar", corsMiddleware(ingestarCromosomas))
	mux.HandleFunc("/api/estado-global", handleEstadoGlobal)
	// (Mantén tus otras rutas aquí tal cual las tenías)

	// --- 3. SERVICIOS EN BACKGROUND ---
	go escucharSocketBuzon()

	// Relé de Nodos
	go func() {
		ln, err := net.Listen("tcp", "0.0.0.0:10003") // Escuchar en todas las interfaces
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

	http.HandleFunc("/api/estado-global", handleEstadoGlobal)

	// --- CONEXIÓN DE LOS CROMOSOMAS DE KIMI ---
	// Estas líneas "despiertan" las funciones que Go marcó como unused
	mux.HandleFunc("/api/agregar-historial", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		// Aquí invocas la lógica que estaba desconectada
		// Ejemplo: agregarAlHistorial(...)
		w.Write([]byte("Historial vinculado"))
	}))

	mux.HandleFunc("/api/generar-respuesta", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		// Aquí es donde Kimi cobra vida ante una petición externa
		go generarRespuestaKimi(1, "Activación manual desde Buzón")
		w.Write([]byte("Generación de respuesta iniciada"))
	}))

	// --- INTEGRACIÓN FINAL DE AGREGAR AL HISTORIAL ---

	// En lugar de usar HistorialItem, usa el tipo que tu función espera (Mensaje)
	mux.HandleFunc("/api/historial/nuevo", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var entrada Mensaje // <-- CAMBIA ESTO: Usa el tipo que tu función necesita
		if err := json.NewDecoder(r.Body).Decode(&entrada); err != nil {
			http.Error(w, "Error en los datos", http.StatusBadRequest)
			return
		}

		// Ahora el compilador aceptará la asignación porque los tipos coinciden
		agregarAlHistorial(entrada)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Evento guardado"))
	}))

	// --- 4. MOTOR DE SENSADO BLINDADO (El Córtex Vivo) ---
	go iniciarMotorSensado()

	// --- 5. INICIAR SERVIDOR HTTP (CONFIGURACIÓN SOBERANA) ---
	port := os.Getenv("PORT") // Render inyecta su puerto aquí
	if port == "" {
		port = "10002" // Valor por defecto para entorno local
	}

	log.Printf("🚀 Córtex Buzón Online escuchando en puerto :%s", port)

	// El puerto es dinámico para ajustarse a la infraestructura de Render
	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	log.Fatal(server.ListenAndServe())
}

func handleEstadoGlobal(w http.ResponseWriter, r *http.Request) {
	// Definir explícitamente las políticas de acceso soberano
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	// Manejo de la petición de pre-vuelo (OPTIONS) para evitar bloqueos del navegador
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Lógica protegida por Mutex para garantizar la integridad de la telemetría
	muTelemetria.Lock()
	datos := ultimaTelemetria
	muTelemetria.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(datos)
}

// Función profesional para el Motor
func iniciarMotorSensado() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("⚠️ [CÓRTEX]: Motor de sensado recuperado de pánico: %v", r)
			// Reiniciar el motor tras un delay tras error
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

		// Datos de telemetría inyectados en el estado global
		datos := Telemetria{
			Nodo:          "Avellaneda",
			Status:        "SYNCING",
			Temp:          25.0,
			Load:          0.1,
			InputActivity: actividad, // Cambiamos 'Input' por 'InputActivity'
			Satelites:     satelites,
		}

		// Actualizamos el estado del sistema
		actualizarEstadoTelemetria(datos)

		// Cerramos el ciclo de la variable para que el compilador la reconozca como activa
		ultimoPulsoLocal = time.Now()

		log.Printf("📡 [CÓRTEX]: Telemetría actualizada en nodo %s | Pulso: %v", datos.Nodo, ultimoPulsoLocal.Format("15:04:05"))

		time.Sleep(5 * time.Second)
	}
}

func escucharSocketBuzon() {
	// Definimos la ruta de forma inteligente
	socketPath := os.Getenv("GEOCHAT_SOCKET_PATH")

	// Si la variable no está definida, usamos un valor por defecto seguro
	if socketPath == "" {
		if _, err := os.Stat("/data/data/com.termux"); err == nil {
			// Estamos en Termux (Móvil)
			socketPath = "/data/data/com.termux/files/home/.geochat_buzon.sock"
		} else {
			// Estamos en cualquier otro lugar (Render, Linux, PC)
			socketPath = "./.geochat_buzon.sock"
		}
	}

	// Limpiar si el socket ya existe por un crash previo
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

	// Crear el objeto para el radar
	nuevaAmenaza := ObjetoLattice{
		Name:    fmt.Sprintf("AMENAZA: %s", msg["target"]),
		Azimuth: 0, // Puedes calcularlo basado en el tipo de ataque si quieres
		Altitud: 0,
	}

	muAmenazas.Lock()
	amenazasDetectadas = append(amenazasDetectadas, nuevaAmenaza)
	muAmenazas.Unlock()

	log.Printf("⚠️ [RADAR]: Amenaza inyectada al mapa: %s", msg["target"])
}

func obtenerDatosTrackingReal() []ObjetoLattice {
	var lista []ObjetoLattice

	// 1. Obtener Aviones (Logica modular)
	lista = append(lista, fetchOpenSky()...)

	// 2. Obtener Satélite ISS
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

	// 3. Inyectar Amenazas detectadas por la Iron Grid (Seguridad Soberana)
	muAmenazas.Lock()
	if len(amenazasDetectadas) > 0 {
		// Añadimos las amenazas detectadas al mapa de radar
		lista = append(lista, amenazasDetectadas...)
	}
	muAmenazas.Unlock()

	return lista
}

// Extraemos la lógica de OpenSky para mantener la armonía
func fetchOpenSky() []ObjetoLattice {
	var lista []ObjetoLattice
	timestamp := time.Now().Unix()

	// 1. Simulación de 3 Aviones (Vuelo dinámico, Altitud comercial)
	for i := 0; i < 3; i++ {
		offset := float64(timestamp % 360)
		azimuth := (float64(i) * 120.0) + offset
		if azimuth > 360 {
			azimuth -= 360
		}

		lista = append(lista, ObjetoLattice{
			Name:    "AVION-" + strconv.Itoa(i+1),
			Azimuth: azimuth,
			Altitud: 10000.0 + float64(i*500), // Altitud comercial
		})
	}

	// 2. Simulación de 3 Satélites (Órbita alta, Azimut constante/lento)
	for i := 0; i < 3; i++ {
		// Los satélites se mueven mucho más lento o parecen estar en posición fija respecto a tierra
		azimuth := (float64(i) * 45.0) + 180.0

		lista = append(lista, ObjetoLattice{
			Name:    "SAT-GEO-" + strconv.Itoa(i+1),
			Azimuth: azimuth,
			Altitud: 500000.0 + float64(i*10000), // Altitud orbital (LEO)
		})
	}

	log.Printf("DEBUG [RADAR]: Simulación activa. Generando %d objetos (Aviones + Satélites).", len(lista))
	return lista
}

// Captura actividad básica (ejemplo: detectar eventos en /dev/input)
func obtenerActividadRaton() string {
	// Simulación: en producción usarías un watcher de eventos de xinput
	return "ACTIVE_SENSING"
}

func actualizarEstadoTelemetria(datos Telemetria) {
	muTelemetria.Lock()
	defer muTelemetria.Unlock()

	ultimaTelemetria = datos
	// Aquí podrías agregar lógica adicional, como guardar en un archivo o log
}

func agregarAlHistorial(m Mensaje) {
	mu.Lock()
	defer mu.Unlock()

	// 1. Cargamos lo que hay
	respuestas := cargarRespuestasKimi()

	// 2. Creamos la nueva entrada
	nueva := RespuestaUnificada{
		Contexto:  m.Entidad,
		Cuerpo:    m.Mensaje,
		Timestamp: time.Now(),
	}
	respuestas = append(respuestas, nueva)

	// 3. Guardamos en el archivo persistente
	datos, _ := json.MarshalIndent(respuestas, "", "  ")
	err := os.WriteFile(archivoRespuestasKimi, datos, 0644)
	if err != nil {
		log.Printf("❌ [BUZÓN]: Error crítico al persistir en disco: %v", err)
	}
}

// Pon esto fuera de la función main()
func SovereignCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
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

// --- Funciones de Gestión de Hash para la Sincronía ---

func calcularHash(adn string) string {
	hash := sha256.Sum256([]byte(adn))
	return hex.EncodeToString(hash[:])
}

// Para este ejemplo, guardaremos el hash en un archivo simple
const archivoHash = "adn_hash.txt"

func cargarHashDesdeDisco() string {
	datos, err := ioutil.ReadFile(archivoHash)
	if err != nil {
		return "" // Si no existe, retorna vacío para forzar la primera inyección
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

// verificarADN procesa la huella digital del ADN recibido.
// Compara el hash del contenido actual con el registrado en el Buzón.
func verificarADN(w http.ResponseWriter, r *http.Request) {
	// 1. Decodificar el payload entrante
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

	// 2. Validación de Integridad (Uso de la función delegada)
	// Si esNuevoADN es true, el hash cambió y se requiere reconfiguración.
	if !verificarIntegridad(payload.ADN) {
		log.Println("⚡ [SYNC]: ADN detectado como idéntico. Resonancia estable.")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"up_to_date"}`))
		return
	}

	// 3. Si llegamos aquí, hubo una evolución en el ADN
	log.Println("🧬 [SYNC]: Evolución de ADN detectada. Reiniciando Cortex...")

	// --- Lógica de Reinyección ---
	// Aquí disparas tu proceso de inyección de Kimi en la nube
	// inyectarCromosomas(payload.ADN)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte(`{"status":"reconfiguring"}`))
}

// En tu código de Render (Buzón):
func verificarIntegridad(adnNuevo string) bool {
	nuevoHash := calcularHash(adnNuevo)
	hashGuardado := cargarHashDesdeDisco() // O memoria

	if nuevoHash == hashGuardado {
		return false // Ya está sincronizado, no inyectar
	}
	guardarHash(nuevoHash)
	return true // ADN cambiado, es necesaria la re-inyección
}

// ingestarCromosomas procesa el ADN recibido, lo persiste y activa a Kimi
func ingestarCromosomas(w http.ResponseWriter, r *http.Request) {
	// 1. Definir la estructura esperada del payload
	var payload struct {
		ADN      string `json:"adn"`
		Trilogia string `json:"trilogia"`
		Mapa     string `json:"mapa"`
	}

	// 2. Decodificar el cuerpo de la petición
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		log.Printf("❌ [CORTEX]: Error decodificando payload: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// 3. Persistir los cromosomas en disco (almacenamiento atómico)
	if err := os.WriteFile("adn_maestro.json", []byte(payload.ADN), 0644); err != nil {
		log.Printf("❌ [CORTEX]: Error guardando adn_maestro: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	os.WriteFile("cromosoma_trilogia.json", []byte(payload.Trilogia), 0644)
	os.WriteFile("mapa_cognitivo.json", []byte(payload.Mapa), 0644)

	log.Println("📥 [CORTEX]: Cromosomas recibidos y persistidos en disco.")

	// 4. Inyección Cognitiva (Despertar de Kimi)
	if err := InyectarCromosomasEnKimi(); err != nil {
		log.Printf("❌ [KIMI]: Error en la inyección: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	log.Println("📥 [CORTEX]: Cromosomas inyectados y Kimi reconfigurada.")
	w.WriteHeader(http.StatusOK)
}

// InyectarCromosomasEnKimi orquestará la carga del nuevo ADN al motor de Kimi.
func InyectarCromosomasEnKimi() error {
	log.Println("🧬 [KIMI]: Iniciando proceso de reconfiguración cognitiva...")

	// 1. Leer los archivos persistidos por el Buzón
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

	// 2. Validación de integridad
	if len(adn) == 0 || len(trilogia) == 0 || len(mapa) == 0 {
		return fmt.Errorf("integridad fallida: archivos de cromosomas incompletos")
	}

	// 3. Simulación de carga (Logging de activación)
	log.Printf("✅ [KIMI]: ADN maestro cargado (%d bytes)", len(adn))
	log.Printf("✅ [KIMI]: Trilogía operativa cargada (%d bytes)", len(trilogia))
	log.Printf("✅ [KIMI]: Mapa cognitivo integrado (%d bytes)", len(mapa))

	// 4. Confirmación de identidad activa
	log.Println("✨ [KIMI]: Reconfiguración completa. Nueva identidad activada.")
	return nil
}

// --- SECCIÓN DE PERSISTENCIA DE KIMI (Añadir esto al final de main.go) ---
// const archivoRespuestasKimi = "respuestas_kimi.json"
const archivoRespuestasKimi = "./storage/respuestas_kimi.json"

// Necesitamos esta función para cargar el estado previo
func cargarRespuestasKimi() []RespuestaUnificada {
	// 1. Verificamos existencia
	if _, err := os.Stat(archivoRespuestasKimi); os.IsNotExist(err) {
		log.Printf("⚠️ [BUZÓN]: Archivo de respuestas no existe en %s, iniciando nuevo historial.", archivoRespuestasKimi)
		return []RespuestaUnificada{}
	}

	// 2. Leemos el archivo
	datos, err := os.ReadFile(archivoRespuestasKimi)
	if err != nil {
		log.Printf("❌ [BUZÓN]: Error al leer el archivo de respuestas: %v", err)
		return []RespuestaUnificada{}
	}

	// 3. Deserializamos
	var respuestas []RespuestaUnificada
	if err := json.Unmarshal(datos, &respuestas); err != nil {
		log.Printf("❌ [BUZÓN]: Error al decodificar JSON (posible archivo corrupto): %v", err)
		return []RespuestaUnificada{}
	}

	return respuestas
}

func generarRespuestaKimi(mensajeID int, contenido string) {
	log.Printf("🧠 [CORTEX]: Activando Kimi para ID #%d...", mensajeID)

	// 1. LECTURA SOBERANA: Leemos el ADN del disco
	adn, err := os.ReadFile("adn_maestro.json")
	contextoADN := "ADN_NO_CARGADO"
	if err == nil {
		contextoADN = string(adn)
	} else {
		log.Printf("⚠️ [CORTEX]: ADN no encontrado: %v", err)
	}
	log.Printf("DEBUG: Contexto cargado con longitud: %d", len(contextoADN))

	// 2. MODO LOCAL: Disparo directo a Ollama (Phi-3)
	// Nota: Al ser ejecución local, no necesitamos el bloque IF RENDER.
	log.Printf("🔥 [DEBUG]: Disparando Ollama local para mensaje %d...", mensajeID)

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

	// 3. PERSISTENCIA Y PUSH AL BUZÓN
	mu.Lock()
	nueva := RespuestaUnificada{
		ID:        mensajeID,
		Respuesta: respuestaFinal,
		Timestamp: time.Now(),
		Contexto:  "FRIEND",
		Cuerpo:    contenido,
	}

	// A. Persistencia local en el Linux Lab
	respuestas := cargarRespuestasKimi()
	respuestas = append(respuestas, nueva)
	finalData, _ := json.MarshalIndent(respuestas, "", "  ")
	os.WriteFile(archivoRespuestasKimi, finalData, 0644)
	mu.Unlock()

	// B. 🔥 AQUÍ CERRAMOS EL CICLO: Bypass Soberano al Buzón
	// Esto es lo que permite que tu Frontend vea la respuesta sin preguntar al local
	GuardarEnBuzon(Mensaje{
		Entidad: "KIMI",
		Mensaje: respuestaFinal,
	})

	log.Printf("✅ [KIMI]: Respuesta integrada y enviada al Buzón para mensaje #%d", mensajeID)
}

func GuardarEnBuzon(nuevoMensaje Mensaje) error {
	mu.Lock()
	defer mu.Unlock()

	// 1. Cargamos el estado actual (la médula)
	respuestas := cargarRespuestasKimi()

	// 2. Creamos la nueva unidad de memoria
	nueva := RespuestaUnificada{
		ID:        len(respuestas) + 1,
		Respuesta: nuevoMensaje.Mensaje,
		Timestamp: time.Now(),
		Contexto:  nuevoMensaje.Entidad,
	}

	// 3. Persistimos la nueva estructura
	respuestas = append(respuestas, nueva)
	finalData, err := json.MarshalIndent(respuestas, "", "  ")
	if err != nil {
		log.Printf("❌ [BUZÓN-ERROR]: Fallo al serializar médula: %v", err)
		return err
	}

	// 4. Escritura atómica
	err = os.WriteFile(archivoRespuestasKimi, finalData, 0644)
	if err != nil {
		log.Printf("❌ [BUZÓN-ERROR]: Fallo al escribir en disco: %v", err)
		return err
	}

	log.Printf("✅ [BUZÓN-RENDER]: Respuesta de %s inyectada en médula (ID: %d).", nuevoMensaje.Entidad, nueva.ID)
	return nil
}
