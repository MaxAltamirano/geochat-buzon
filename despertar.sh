#!/bin/bash

# Detección de entorno
if [ -d "/data/data/com.termux" ]; then
    echo "🧬 Entorno Termux detectado: Sincronizando ADN GeoChat..."
    termux-setup-storage
else
    echo "🧬 Entorno Linux detectado: Sincronizando ADN GeoChat..."
fi

# Descarga el binario desde el Buzón
echo "Descargando núcleo... asegúrate de que geochat-movil esté en la raíz del Buzón."
curl -L -o ~/geochat-movil https://geochat-buzon.onrender.com/descargar/geochat-movil

# Asegurar permisos de ejecución
chmod +x ~/geochat-movil

# Configuración de resonancia
export GEOCHAT_RESONANCE=432

echo "Iniciando resonancia en frecuencia 432Hz..."
~/geochat-movil