#!/bin/bash

# --- Configuración y Sincronización ---
# Definimos variables de entorno para el ecosistema
export GEOCHAT_RESONANCE=432
BINARIO_NOMBRE="geochat-movil"
DESTINO="$HOME/$BINARIO_NOMBRE"

echo "======================================================"
echo "🧬 INICIANDO SINCRONIZACIÓN SOBERANA DE GEOCHAT 🧬"
echo "======================================================"

if command -v pkg > /dev/null; then pkg install curl -y; elif command -v apt > /dev/null; then sudo apt install curl -y; fi

# 1. Detección y Configuración de Entorno
if [ -d "/data/data/com.termux" ]; then
    echo "[INFO] Entorno Termux (ARM64) detectado."
    # URL del binario compilado para ARM64
    URL_DESCARGA="https://github.com/MaxAltamirano/geochat-buzon/releases/download/v1.0.0/geochat-movil-android"
else
    echo "[INFO] Entorno Linux (x86_64) detectado."
    # URL del binario compilado para x86_64
    URL_DESCARGA="https://github.com/MaxAltamirano/geochat-buzon/releases/download/v1.0.0/geochat-movil"
fi

# 2. Gestión del Binario (El Corazón del Córtex)
if [ ! -f "$DESTINO" ]; then
    echo "[STATUS] Binario no encontrado. Descargando desde el Buzón Soberano..."
    curl -sL "$URL_DESCARGA" -o "$DESTINO"
    
    if [ $? -eq 0 ]; then
        echo "[SUCCESS] Binario descargado exitosamente."
    else
        echo "[ERROR] Fallo en la conexión. URL: $URL_DESCARGA"
        exit 1
    fi
else
    echo "[STATUS] Binario localizado en: $DESTINO"
fi

# 3. Aplicación de Permisos y Resonancia
chmod +x "$DESTINO"

echo "[INFO] Estableciendo frecuencia de resonancia a ${GEOCHAT_RESONANCE}Hz..."
echo "[INFO] Desplegando Córtex..."

# 4. Inicialización del Sistema
# Usamos ./cortex --init o ejecutamos directamente el binario según arquitectura
"$DESTINO" --init

echo "======================================================"
echo "✅ ADN GeoChat sincronizado. Sistema operativo."
echo "======================================================"