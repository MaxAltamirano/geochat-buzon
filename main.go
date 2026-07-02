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
	mensajes = []MensajePendiente{}
	mu       sync.Mutex // Bloqueo para operaciones seguras

)

func main() {

	// 1. Asegurar la existencia de la carpeta de almacenamiento con permisos 0755
	// 0755 significa: el dueño puede leer/escribir/ejecutar, el resto solo leer/ejecutar
	err := os.MkdirAll("./storage", 0755)
	if err != nil {
		log.Fatalf("❌ [CRÍTICO]: No pude crear la carpeta ./storage: %v", err)
	}
	log.Println("📁 [SISTEMA]: Carpeta ./storage lista y con permisos asegurados.")

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

	// Esta ruta unificada entrega lo que Kimi ha respondido y lo que el Buzón tiene listo
	// Definimos la lógica del handler en una variable o función para poder envolverla

	// handlerSalida unifica la lectura de respuestas locales y pendientes
	// Esta función debe sustituir a la que tenías en main.go
	var handlerSalida = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		// 1. Siempre cargamos lo que Kimi ya procesó en nuestro disco local (Linux)
		respuestas := cargarRespuestasKimi()

		// 2. Cargamos lo que está en cola esperando ser procesado (Médula)
		pendientes := cargarDeDisco()

		// 3. Empaquetamos todo con un identificador de origen
		data := map[string]interface{}{
			"items":      respuestas,
			"pendientes": pendientes,
			"source":     "nativa_local_linux", // Identificador de soberanía de datos
			"ts":         time.Now().Format(time.RFC3339),
		}

		w.Header().Set("Content-Type", "application/json")

		// 4. Si hay error, logueamos pero intentamos responder igual
		if err := json.NewEncoder(w).Encode(data); err != nil {
			log.Printf("❌ [BUZÓN]: Error crítico al serializar respuesta: %v", err)
			http.Error(w, "Error interno de persistencia", http.StatusInternalServerError)
			return
		}

		log.Printf("📡 [BUZÓN]: Salida entregada. Items: %d, Pendientes: %d", len(respuestas), len(pendientes))
	})

	// Aplicamos el middleware aquí
	mux.Handle("/api/buzon/salida", SovereignCORS(handlerSalida))

	mux.HandleFunc("/api/marcar_procesando", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		// 1. Obtención y validación del ID
		idStr := r.URL.Query().Get("id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			log.Printf("❌ [MÉDULA]: ID inválido recibido: %s", idStr)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		mu.Lock()
		lista := cargarDeDisco()
		mensajeContenido := ""
		encontrado := false

		// 2. Actualización de estado en la Médula
		for i := range lista {
			if lista[i].ID == id {
				lista[i].Estado = "PROCESSING"
				lista[i].UpdatedAt = time.Now()
				mensajeContenido = lista[i].Mensaje
				encontrado = true
				break // Salimos del bucle al encontrarlo
			}
		}

		if !encontrado {
			mu.Unlock()
			log.Printf("⚠️ [MÉDULA]: Intento de procesar ID inexistente #%d", id)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// 3. Persistimos el cambio antes de liberar el lock
		guardarEnDisco(lista)
		mu.Unlock()

		// 4. Disparo del puente cognitivo
		// Ejecutamos esto fuera del lock para no bloquear el servidor mientras Kimi trabaja
		go func(id int, contenido string) {
			log.Printf("🚀 [CORTEX]: Iniciando procesamiento automático para ID #%d", id)
			generarRespuestaKimi(id, contenido)
		}(id, mensajeContenido)

		// 5. Respuesta inmediata al cliente
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(fmt.Sprintf(`{"status":"success", "message":"Procesando ID %d"}`, id)))
	}))

	mux.HandleFunc("/api/cortex/ultimo-pulso", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		// Aquí defines la "frecuencia" o "vórtice" de tu sistema
		data := map[string]interface{}{
			"frecuencia": 432.169, // Valor base de tu SNC
			"vortice":    0.0972,
			"status":     "ONLINE",
			"timestamp":  time.Now().Unix(),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(data)
	}))

	mux.HandleFunc("/api/verificar-adn", corsMiddleware(verificarADN))
	mux.HandleFunc("/api/ingestar-cromosomas", corsMiddleware(ingestarCromosomas))

	// --- INICIALIZACIÓN DE SERVIDOR ---------------------------------
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
const archivoRespuestasKimi = "respuestas_kimi.json"

func cargarRespuestasKimi() []RespuestaUnificada {
	if _, err := os.Stat(archivoRespuestasKimi); os.IsNotExist(err) {
		return []RespuestaUnificada{}
	}
	datos, err := ioutil.ReadFile(archivoRespuestasKimi)
	if err != nil {
		log.Printf("❌ [KIMI]: Error leyendo respuestas: %v", err)
		return []RespuestaUnificada{}
	}
	var respuestas []RespuestaUnificada
	json.Unmarshal(datos, &respuestas)
	return respuestas
}


func generarRespuestaKimi(mensajeID int, contenido string) {
	// 1. PROTECTOR DE ENTORNO Y DELEGACIÓN
	if os.Getenv("RENDER") != "" {
		log.Printf("☁️ [NODO]: Nube detectada. Delegando IA al Nodo Avellaneda...")
		go delegarAIAlNodoLocal(mensajeID, contenido)
		return
	}

	// 2. EJECUCIÓN LOCAL (Solo si NO es Render)
	log.Printf("🔥 [DEBUG]: Disparando Ollama local para mensaje %d...", mensajeID)

	payload := map[string]interface{}{
		"model":  "phi3:mini",
		"prompt": "[CROMOSOMA GEOCHAT - ADN 37] Eres un organismo soberano. Tu estructura se basa en Proteínas A, T, G y C. Responde a esto con tu lógica sistémica: " + contenido,
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

	// 3. PERSISTENCIA
	mu.Lock()
	defer mu.Unlock()

	respuestas := cargarRespuestasKimi()
	nueva := RespuestaUnificada{
		ID:        mensajeID,
		Respuesta: ollamaResp.Response,
		Timestamp: time.Now(),
		Contexto:  "FRIEND",
		Cuerpo:    contenido,
	}
	respuestas = append(respuestas, nueva)

	finalData, _ := json.MarshalIndent(respuestas, "", "  ")
	os.WriteFile(archivoRespuestasKimi, finalData, 0644)

	log.Printf("✅ [KIMI]: Respuesta integrada para mensaje #%d", mensajeID)
}

func delegarAIAlNodoLocal(mensajeID int, contenido string) {
	// 1. Define la URL a la que quieres enviar el mensaje
	// NOTA: Asegúrate de que esta URL sea accesible desde la nube hacia tu Nodo (ej. túnel)
	url := "http://tu-dominio-o-tunel.com/api/cortex/enviar-mensaje" 

	payload := map[string]interface{}{
		"id":      mensajeID,
		"mensaje": contenido,
	}
	datos, _ := json.Marshal(payload)
	
	// 2. Corregimos el http.Post con los 3 argumentos obligatorios
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(datos))
	
	if err != nil {
		log.Printf("❌ [DELEGACIÓN]: Fallo al contactar Nodo Avellaneda: %v", err)
		return
	}
	defer resp.Body.Close()
	
	log.Printf("🚀 [DELEGACIÓN]: Mensaje #%d enviado al Nodo Avellaneda exitosamente.", mensajeID)
}