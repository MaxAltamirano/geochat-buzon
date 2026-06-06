#!/data/data/com.termux/files/usr/bin/bash
echo "🧬 Sincronizando ADN GeoChat..."
termux-setup-storage
# Descarga el binario que generó tu PC (Espejo ARM64)
curl -L -o ~/geochat-movil https://geochat-buzon.onrender.com/descargar/geochat-movil
chmod +x ~/geochat-movil
export GEOCHAT_RESONANCE=432
~/geochat-movil
