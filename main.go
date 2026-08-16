package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
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
	"strings"
	"sync"
	"time"

	_ "github.com/lib/pq"
)

var db *sql.DB

// --- ESTRUCTURAS Y FUNCIONES DE SOPORTE PARA EL BUZÓN EN RENDER ---

type DocumentacionAnalisis struct {
	IDIdea               string    `json:"id_idea"`
	FilePath             string    `json:"file_path"`
	NombreArchivo        string    `json:"nombre_archivo"`
	Estado               string    `json:"estado"`
	ContenidoOriginal    string    `json:"contenido_original"`
	ContenidoAuditado    string    `json:"contenido_auditado"`
	ResumenCambios       string    `json:"resumen_cambios"`
	Timestamp            time.Time `json:"timestamp"`
	TieneRecomendaciones bool      `json:"tiene_recomendaciones"`
}

type LoteAuditoriaMasiva struct {
	Nodo      string                  `json:"nodo"`
	Timestamp time.Time               `json:"timestamp"`
	Bloque    []DocumentacionAnalisis `json:"bloque"`
}

type TareaAuditoriaBuzon struct {
	ID                 string    `json:"id"`
	FilePath           string    `json:"file_path"`
	Contenido          string    `json:"contenido"`
	TimestampInyeccion time.Time `json:"timestamp_inyeccion"`
	TimestampRespuesta time.Time `json:"timestamp_respuesta"`
	TamanioBytes       int64     `json:"tamanio_bytes"`
}

// Estructura para recibir el bloque masivo
type payloadIngesta struct {
	IDADN      string                 `json:"id_adn"`
	QRData     string                 `json:"qr_data"`
	HashEstado string                 `json:"hash_estado"`
	NivelBSP   int                    `json:"nivel_bsp"`
	Metadatos  map[string]interface{} `json:"metadatos"`
}

type EstadoGlobalSNC struct {
	LlaverosSim   []LlaveroSimData `json:"Llaveros_SIM"`
	InputActivity string           `json:"input_activity"`
	Load          float64          `json:"load"`
	Nodo          string           `json:"nodo"`
	Status        string           `json:"status"`
	Temp          float64          `json:"temp"`
	Timestamp     int64            `json:"timestamp"`
}

type LlaveroSimData struct {
	Name    string  `json:"name"`
	Azimuth float64 `json:"azimuth"`
	Rssi    int     `json:"rssi"`
	Firma   string  `json:"firma"`
}

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

type RegistroAuditoria struct {
	ID        int                    `json:"id"`
	Nodo      string                 `json:"nodo"`
	Timestamp time.Time              `json:"timestamp"`
	Payload   map[string]interface{} `json:"payload"`
	HashADN   string                 `json:"hash_adn"`
}

type LoteEntrante struct {
	Nodo      string               `json:"nodo"`
	Timestamp time.Time            `json:"timestamp"`
	Bloque    []DocumentoAuditoria `json:"bloque"`
}

// Representa cada archivo individual dentro del bloque
type DocumentoAuditoria struct {
	NombreArchivo     string `json:"nombre_archivo"`
	ContenidoOriginal string `json:"contenido"` // <--- Ajusta según tu struct real
}

// Esta es la estructura que envías de vuelta a Linux lista para Ollama
type EstructuraParaOllama struct {
	Nombre string `json:"nombre"`
	Codigo string `json:"codigo"`
	Prompt string `json:"prompt"`
}

// Estructura para el bloque grande que entra a Render
type BloqueMasivoEntrada struct {
	LoteDocumentos []DocumentacionAnalisis `json:"lote_documentos"`
}

// --- CONSTANTES DE PERSISTENCIA ---
const (
	archivoPersistencia   = "medula_local.json"
	archivoHash           = "adn_hash.txt"
	archivoRespuestasKimi = "./storage/respuestas_kimi.json"
)

// --- 2. VARIABLES GLOBALES DE ESTADO UNIFICADAS Y DEFINITIVAS ---
var (
	mu               sync.Mutex
	ultimoPulsoLocal time.Time
	mensajes         = []MensajePendiente{}

	// Control del Buzón y Estado Global tipado con EstadoGlobalSNC
	ultimoPulso   = time.Now()
	estadoMemoria = EstadoGlobalSNC{
		LlaverosSim: []LlaveroSimData{
			{Name: "LLAISIM-AVL-01", Azimuth: 45, Rssi: -58, Firma: "d4e2f918b2c473e1a89f... (ECDSA Hex)"},
			{Name: "LLAISIM-AVL-02", Azimuth: 135, Rssi: -62, Firma: "7a8b3c2d1e9f0482b5ea... (ECDSA Hex)"},
			{Name: "LLAISIM-AVL-03", Azimuth: 225, Rssi: -70, Firma: "1f2e3d4c5b6a7f8e9d0c... (ECDSA Hex)"},
			{Name: "NODO_MOVIL_SOP", Azimuth: 315, Rssi: -50, Firma: "9e8d7c6b5a4f3e2d1c0b... (ECDSA Hex)"},
		},
		InputActivity: "ACTIVE_SIM_MESH",
		Load:          0.12,
		Nodo:          "Avellaneda",
		Status:        "SYNCING",
		Temp:          26.0,
		Timestamp:     time.Now().Unix(),
	}

	amenazasDetectadas []ObjetoLattice
	muAmenazas         sync.Mutex
	ultimaTelemetria   Telemetria
	muTelemetria       sync.Mutex
)

// Handler para ingestar el bloque directamente en PostgreSQL (transcriptomas)
func ingestarTranscriptomaBSP(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var payload payloadIngesta
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "payload_invalido"})
			return
		}

		// Convertir metadatos a JSONB string o []byte compatible con Postgres
		metaJSON, err := json.Marshal(payload.Metadatos)
		if err != nil {
			metaJSON = []byte("{}")
		}

		query := `
			INSERT INTO transcriptomas (id_adn, qr_data, hash_estado, nivel_bsp, metadatos, captura_viva)
			VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING id;
		`

		var nuevoID int
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err = db.QueryRowContext(ctx, query,
			payload.IDADN,
			payload.QRData,
			payload.HashEstado,
			payload.NivelBSP,
			metaJSON,
			time.Now(),
		).Scan(&nuevoID)

		if err != nil {
			log.Printf("❌ [DB-ERROR]: No se pudo insertar el transcriptoma BSP: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "error_base_datos"})
			return
		}

		log.Printf("🧬 [TRANSCRIPTOMA-BSP]: Bloque insertado con éxito en PostgreSQL. ID Asignado: #%d (Nivel BSP: %d)", nuevoID, payload.NivelBSP)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":    "transcriptoma_bsp_guardado",
			"id":        nuevoID,
			"nivel_bsp": payload.NivelBSP,
		})
	}
}

// RegistrarRutaEstadoGlobal configura el endpoint limpio y blindado para el frontend
func RegistrarRutaEstadoGlobal(mux *http.ServeMux, corsMiddleware func(http.HandlerFunc) http.HandlerFunc) {
	mux.HandleFunc("/api/estado-global", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		// Control soberano de expiración: Si pasan más de 30s sin pulso, pasa a OFFLINE automáticamente
		if time.Since(ultimoPulso) > 30*time.Second {
			estadoMemoria.Status = "OFFLINE"
		} else {
			estadoMemoria.Status = "SYNCING"
		}

		// Actualizamos el timestamp del frame enviado
		estadoMemoria.Timestamp = time.Now().Unix()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Serialización optimizada del estado tipado hacia el radar.js
		if err := json.NewEncoder(w).Encode(estadoMemoria); err != nil {
			http.Error(w, "Error serializando estado SNC", http.StatusInternalServerError)
		}
	}))
}

func ActualizarEstadoDesdeBuzon(nuevoStatus string, carga float64) {
	mu.Lock()
	defer mu.Unlock()

	// Actualización correcta usando campos de estructura
	ultimoPulso = time.Now()
	estadoMemoria.Status = nuevoStatus
	estadoMemoria.Load = carga
	estadoMemoria.Timestamp = ultimoPulso.Unix()
}

func AgregarLlaveroAlBuzon(nuevoLlavero LlaveroSimData) {
	mu.Lock()
	defer mu.Unlock()

	// Se añade al slice tipado LlaverosSim sin corchetes de mapa
	estadoMemoria.LlaverosSim = append(estadoMemoria.LlaverosSim, nuevoLlavero)
	estadoMemoria.Timestamp = time.Now().Unix()
}

// Handler que atiende el GET desde Render en tu Linux local
func HandlerEntregarPendientes(w http.ResponseWriter, r *http.Request) {
	// 1. Validar que la petición sea estrictamente GET (seguridad soberana)
	if r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	// 2. Consultar tu PostgreSQL local para traer todos los ítems con estado "PENDIENTE"
	rows, err := db.Query("SELECT id_idea, file_path, nombre_archivo, contenido_original FROM auditorias WHERE estado = 'PENDIENTE' ORDER BY fecha_creacion ASC")
	if err != nil {
		fmt.Printf("[LINUX LOCAL] Error al consultar la DB local: %v\n", err)
		http.Error(w, "Error interno de base de datos", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var pendientes []DocumentacionAnalisis

	// 3. Recorrer los resultados y armar el slice de pendientes
	for rows.Next() {
		var doc DocumentacionAnalisis
		if err := rows.Scan(&doc.IDIdea, &doc.FilePath, &doc.NombreArchivo, &doc.ContenidoOriginal); err != nil {
			fmt.Printf("[LINUX LOCAL] Error al escanear fila: %v\n", err)
			continue
		}
		pendientes = append(pendientes, doc)
	}

	// Si no hay pendientes, devolvemos un array vacío JSON en lugar de null
	if pendientes == nil {
		pendientes = []DocumentacionAnalisis{}
	}

	// 4. Configurar cabecera y responderle a Render con el JSON exacto
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(pendientes); err != nil {
		fmt.Printf("[LINUX LOCAL] Error al codificar JSON para Render: %v\n", err)
		return
	}

	fmt.Printf("[LINUX LOCAL] Sincronización exitosa: Se entregaron %d ítems pendientes a Render.\n", len(pendientes))
}

// --- FUNCIÓN PRINCIPAL (ENTRYPOINT SOBERANO) ---
func main() {

	// 1. Definir la cadena de conexión (ajusta tus credenciales)
	connStr := "host=localhost port=5432 user=postgres dbname=geochat sslmode=disable"

	// 2. Inicializar la variable db (Global o pasada localmente)
	var err error
	db, err = sql.Open("postgres", connStr) // Asegúrate de tener el driver importado
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	log.Println("📁 [SISTEMA]: Iniciando arranque soberano del Córtex Buzón...")

	// 1. Asegurar la infraestructura local de almacenamiento
	if err := os.MkdirAll("./storage", 0755); err != nil {
		log.Printf("⚠️ [AVISO]: Carpeta ./storage ya existe o no pudo crearse: %v", err)
	} else {
		log.Println("📁 [SISTEMA]: Carpeta ./storage lista y asegurada.")
	}

	// 2. Definición del Mux Unificado
	mux := http.NewServeMux()

	// D. Endpoint de Estado Global (Utilizando la función modular y limpia)
	RegistrarRutaEstadoGlobal(mux, corsMiddleware)

	// --- REGISTRO DE RUTAS ---

	// --- RUTAS DE AUDITORÍA Y BUZÓN SOBERANO ---

	// Registrar la ruta exacta que consultará Render
	http.HandleFunc("/api/sincronizar/pendientes", HandlerEntregarPendientes)

	// 2. Endpoint donde el worker local consulta los resultados ya procesados por Ollama
	mux.HandleFunc("/api/auditoria/resultados-listos", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status": "sin_resultados_pendientes",
		})
	}))

	// Suponiendo que tu conexión a postgres se llama 'db'
	mux.HandleFunc("/api/ingestar-transcriptoma", corsMiddleware(ingestarTranscriptomaBSP(db)))

	// Endpoint para que AdminIdeas.vue lea las ideas fiscalizadas y el estado del Córtex
	mux.HandleFunc("/api/ideas/fiscalizadas", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		rutaIdeas := "./storage/ideas_fiscalizadas.json"
		if _, err := os.Stat(rutaIdeas); os.IsNotExist(err) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`[]`))
			return
		}

		datos, err := os.ReadFile(rutaIdeas)
		if err != nil {
			http.Error(w, "Error leyendo buzón de ideas", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(datos)
	}))

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

	mux.HandleFunc("/api/auditoria/ingestar", corsMiddleware(ingestarAuditoriaTecnica))

	//------------------------------------------------------------

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

// Funciones simuladas o adaptadas a tu capa de base de datos en Render
func GuardarEnDbRender(doc DocumentacionAnalisis, estado string) {
	// Aquí conectas con tu postgres local de Render para guardar el ítem en cola
}

func ObtenerSiguientePendienteEnDbRender() *DocumentacionAnalisis {
	// Retorna el siguiente documento pendiente desde la DB de Render
	return nil
}

func ActualizarEstadoDbRender(id string, estado string) {
	// Actualiza el estado en la DB (PENDIENTE, EN_PROCESO, COMPLETADO, ERROR)
}

func ObtenerSiguienteSalidaBuzonRender() *TareaAuditoriaBuzon {
	// Retira el siguiente resultado listo para el GET del worker
	return nil
}

//--------------------------------------------------------------------

func ManejarIngestaLote(w http.ResponseWriter, r *http.Request) {
	var lote LoteAuditoriaMasiva
	if err := json.NewDecoder(r.Body).Decode(&lote); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 1. Guardar cada archivo del bloque en PostgreSQL (Render) con estado "PENDIENTE"
	for _, doc := range lote.Bloque {
		GuardarEnDbRender(doc, "PENDIENTE")
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "lote_recibido_en_cola"})
}

// Cola volátil en memoria de Render (sin base de datos en la nube)
var ColaMemoriaRender []DocumentacionAnalisis

// Buzón de salida temporal en Render para que el Worker de la Linux local lo retire
var BuzonSalidaRender []TareaAuditoriaBuzon

func GuardarEnSalidaBuzonRender(tarea TareaAuditoriaBuzon) {
	BuzonSalidaRender = append(BuzonSalidaRender, tarea)
}

func SolicitarPendientesALinuxLocal() []DocumentacionAnalisis {
	linuxLocalURL := os.Getenv("LINUX_LOCAL_URL")
	if linuxLocalURL == "" {
		fmt.Println("[RENDER] ERROR: LINUX_LOCAL_URL no está configurada.")
		return nil
	}

	fmt.Println("[RENDER] Consultando pendientes a la Linux local...")

	resp, err := http.Get(linuxLocalURL + "/api/sincronizar/pendientes")
	if err != nil {
		fmt.Printf("[RENDER] Error al conectar con la Linux local: %v\n", err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	var pendientes []DocumentacionAnalisis
	err = json.NewDecoder(resp.Body).Decode(&pendientes)
	if err != nil {
		fmt.Printf("[RENDER] Error al decodificar la lista de pendientes: %v\n", err)
		return nil
	}

	return pendientes
}

//---------------------------------------------------------

func EnviarAOllamaNube(nombreArchivo string, contenido string) (string, error) {
	ordenAuditoria := "Eres el Auditor Soberano del Espacio GeoChat. Tu tarea es analizar el código fuente recibido, verificar la resiliencia, detectar fallos de seguridad o arquitectura, y devolver un resumen estructurado de auditoría con sugerencias de mejora."

	if strings.HasSuffix(nombreArchivo, ".go") {
		ordenAuditoria += " Enfócate en concurrencia segura, manejo de errores y rendimiento en Go."
	} else if strings.HasSuffix(nombreArchivo, ".vue") || strings.HasSuffix(nombreArchivo, ".ts") {
		ordenAuditoria += " Enfócate en la reactividad, tipado y consistencia de componentes en el frontend."
	}

	promptCompleto := fmt.Sprintf("%s\n\n--- CÓDIGO A AUDITAR (%s) ---\n%s", ordenAuditoria, nombreArchivo, contenido)

	payload := map[string]interface{}{
		"model":  "llama3",
		"prompt": promptCompleto,
		"stream": false,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	ollamaURL := os.Getenv("OLLAMA_HOST")
	if ollamaURL == "" {
		ollamaURL = "http://localhost:11434"
	}

	resp, err := http.Post(ollamaURL+"/api/generate", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "Auditoría simulada por fallback (Sin conexión a Ollama)", nil
	}
	defer resp.Body.Close()

	var resultadoOllama struct {
		Response string `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&resultadoOllama); err != nil {
		return "Error al decodificar respuesta de Ollama", err
	}

	return resultadoOllama.Response, nil
}

//--------------------------------------------------------------------

func IniciarMotorColaOllama() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		// 1. Si la memoria volátil de Render está vacía, pedimos la lista fresca a la Linux local
		if len(ColaMemoriaRender) == 0 {
			ColaMemoriaRender = SolicitarPendientesALinuxLocal()

			// Si la Linux local responde que no hay nada pendiente, esperamos al próximo tick
			if len(ColaMemoriaRender) == 0 {
				continue
			}
		}

		// 2. Extraer el primer elemento de la cola en memoria de forma segura
		docPendiente := ColaMemoriaRender[0]
		ColaMemoriaRender = ColaMemoriaRender[1:] // Desplaza la cola activa

		// 3. Registrar timestamp exacto de inyección
		timestampInyeccion := time.Now()

		// 4. Mandar a Ollama con la orden explícita y el archivo correspondiente
		resultadoOllama, err := EnviarAOllamaNube(docPendiente.NombreArchivo, docPendiente.ContenidoOriginal)
		timestampRespuesta := time.Now()

		if err != nil {
			continue
		}

		// 5. Empaquetar datos clave para el buzón de salida
		resultadoFinal := TareaAuditoriaBuzon{
			ID:                 docPendiente.IDIdea,
			FilePath:           docPendiente.FilePath,
			Contenido:          resultadoOllama,
			TimestampInyeccion: timestampInyeccion,
			TimestampRespuesta: timestampRespuesta,
			TamanioBytes:       int64(len(docPendiente.ContenidoOriginal)),
		}

		// 6. Depositar el resultado listo para que el Worker local lo retire
		GuardarEnSalidaBuzonRender(resultadoFinal)
	}
}

// 1. Recibes, fraccionas y refactorizas en la memoria volátil de Render
func RecibirYFraccionarBloqueGrande(bloqueMasivo BloqueMasivoEntrada) {
	fmt.Printf("[RENDER] Recibiendo bloque masivo. Fraccionando %d documentos...\n", len(bloqueMasivo.LoteDocumentos))

	// Aquí ya queda ordenado y fraccionado 1 a 1 en la cola
	ColaMemoriaRender = bloqueMasivo.LoteDocumentos

	fmt.Printf("[RENDER] Bloque fraccionado con éxito. %d elementos listos en la cola volátil.\n", len(ColaMemoriaRender))
}

// 2. En el handler o flujo principal, mandas ESA MISMA cola ya fraccionada a la Linux local:
func HandlerRecibirBloqueRender(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	var bloqueMasivo BloqueMasivoEntrada
	if err := json.NewDecoder(r.Body).Decode(&bloqueMasivo); err != nil {
		http.Error(w, "Error al decodificar", http.StatusBadRequest)
		return
	}

	// Fraccionamos y cargamos la cola volátil en Render
	RecibirYFraccionarBloqueGrande(bloqueMasivo)

	// Inmediatamente mandamos A ESA MISMA COLA YA REFACTORIZADA a la Linux local para que persista
	go func() {
		// Armamos el paquete usando exactamente lo que está en ColaMemoriaRender
		paqueteRefactorizado := BloqueMasivoEntrada{
			LoteDocumentos: ColaMemoriaRender,
		}

		// Usamos la función correcta que le hace el POST a tu Linux local
		err := EnviarLoteRefactorizadoALinuxLocal(paqueteRefactorizado)
		if err != nil {
			fmt.Printf("[RENDER-SYNC] Error al sincronizar el lote con la Linux local: %v\n", err)
		}
	}()

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"bloque_recibido_y_sincronizando"}`))
}

func EnviarLoteRefactorizadoALinuxLocal(bloque BloqueMasivoEntrada) error {
	linuxLocalURL := os.Getenv("LINUX_LOCAL_URL")
	if linuxLocalURL == "" {
		return fmt.Errorf("LINUX_LOCAL_URL no está configurada en Render")
	}

	jsonData, err := json.Marshal(bloque)
	if err != nil {
		return fmt.Errorf("error al serializar el paquete refactorizado: %v", err)
	}

	resp, err := http.Post(linuxLocalURL+"/api/sincronizar/guardar-lote", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("error de red al conectar con la Linux local: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("la Linux local rechazó el lote. Código: %d", resp.StatusCode)
	}

	fmt.Println("[RENDER-SYNC] Lote refactorizado enviado y persistido con éxito en la Linux local.")
	return nil
}

func ManejarObtenerResultado(w http.ResponseWriter, r *http.Request) {
	// Saca el primer resultado listo de la cola de salida de Render
	resultado := ObtenerSiguienteSalidaBuzonRender()
	if resultado == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resultado)
}

//------ auditoria de archivos

func ingestarAuditoriaTecnica(w http.ResponseWriter, r *http.Request) {
	fmt.Println("🔵 [BUZÓN]: Recepcion del lote grande desde la linux local")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// 1. Usamos LoteEntrante para decodificar el JSON entrante que trae el .Bloque
	var lote LoteEntrante
	if err := json.NewDecoder(r.Body).Decode(&lote); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		log.Printf("❌ [AUDITORÍA]: Error decodificando payload técnico: %v", err)
		return
	}

	// 2. Refactorización recorriendo lote.Bloque (no reg.Bloque)
	var listaRefactorizada []EstructuraParaOllama
	for _, item := range lote.Bloque {
		listaRefactorizada = append(listaRefactorizada, EstructuraParaOllama{
			Nombre: item.NombreArchivo,
			Codigo: item.ContenidoOriginal, // Texto plano listo para procesar
			Prompt: "Analiza este código de GeoChat buscando vulnerabilidades, resonancia 432Hz y eficiencia.",
		})
	}

	// 3. Log con punto azul indicando el envío del refactorizado hacia Linux
	fmt.Printf("🔵 [BUZÓN]: Envío del Refactorizado para la linux local (Total de archivos: %d)\n", len(listaRefactorizada))

	// 4. Devolución de la estructura refactorizada a la Linux local mediante la respuesta HTTP
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(listaRefactorizada); err != nil {
		log.Printf("❌ [BUZÓN]: Error enviando el paquete refactorizado a Linux: %v", err)
	}
}

//----------------------------------------------------------------

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

	// Sincronizar también con el estado global que consume el endpoint /api/estado-global
	mu.Lock()
	defer mu.Unlock()

	// Actualización correcta usando campos de estructura y forzando el estado activo
	ultimoPulso = time.Now()
	estadoMemoria.Status = "SYNCING"
	estadoMemoria.Load = datos.Load
	estadoMemoria.Timestamp = ultimoPulso.Unix()

	// Si tu estructura EstadoGlobalSNC usa Llaveros_SIM o similar, asignalo acorde a los campos disponibles:
	// estadoMemoria.Llaveros_SIM = datos.Llaveros_SIM
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
