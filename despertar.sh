#!/bin/bash

# ==============================================================================
# GEOCHAT - Protocolo de Despertar Soberano (Bootstrap Maestro)
# Función: Instalación, Sincronización y Orquestación del Nodo
# ==============================================================================

# --- Configuración del Ecosistema ---
export GEOCHAT_RESONANCE=432
BINARIO_NOMBRE="geochat-movil"
DESTINO="$HOME/$BINARIO_NOMBRE"
REPO_URL="https://github.com/MaxAltamirano/geochat-buzon.git"
RENDER_IP="tu-app.onrender.com" 
RENDER_PORT="10003" 
LATITUD="-34.75"
LONGITUD="-58.35"

echo "🧬 [SISTEMA] Iniciando despliegue autónomo (Frecuencia: ${GEOCHAT_RESONANCE}Hz)..."

# 1. Verificación de Dependencias Base (Git, Curl, Netcat)
for cmd in git curl nc; do
    if ! command -v $cmd &> /dev/null; then
        echo "⚠️ [INFO] Instalando dependencia: $cmd"
        if command -v pkg > /dev/null; then pkg install $cmd -y; elif command -v apt > /dev/null; then sudo apt install $cmd -y; fi
    fi
done

# 2. Sincronización del Ecosistema (Repositorio)
if [ ! -d "geochat-buzon" ]; then
    echo "🌐 [STATUS] Clonando ecosistema soberano..."
    git clone "$REPO_URL" || { echo "[ERROR] Fallo al clonar."; exit 1; }
fi

# 3. Gestión Dinámica del Binario (Córtex)
if [ -d "/data/data/com.termux" ]; then
    URL_DESCARGA="https://github.com/MaxAltamirano/geochat-buzon/releases/download/v1.0.0/geochat-movil-android"
else
    URL_DESCARGA="https://github.com/MaxAltamirano/geochat-buzon/releases/download/v1.0.0/geochat-movil"
fi

if [ ! -f "$DESTINO" ]; then
    echo "[STATUS] Descargando Córtex desde el Buzón Soberano..."
    curl -sL "$URL_DESCARGA" -o "$DESTINO" || { echo "[ERROR] Descarga fallida."; exit 1; }
fi
chmod +x "$DESTINO"

# 4. Inicialización del Núcleo
echo "[STATUS] Desplegando Córtex..."
"$DESTINO" --init

# 5. Orquestador de Relé (Daemon de Resonancia)
echo "[RELE] Activando enlace de resonancia hacia Render..."
(
    while true; do
        TELEMETRIA=$(printf '{"node_id": "SAM_MAX_01", "status": "GATEWAY_READY", "mesh_grid": "IRON_GRID_ACTIVE", "location": {"lat": %s, "lon": %s}, "relay_port": 10003}' "$LATITUD" "$LONGITUD")
        echo "$TELEMETRIA" | nc -w 2 "$RENDER_IP" "$RENDER_PORT"
        sleep 10
    done
) &

echo "======================================================"
echo "✅ ADN GeoChat Sincronizado y Armonizado."
echo "✅ Nodo Móvil operando como Gateway (SAM_MAX_01)."
echo "======================================================"