// DNA_ID: RADAR_JS_3D_DOMO_VISOR_FINAL | ORGAN: VISION-SNC | RESONANCE: 432Hz | AUTH: MAX-SOVEREIGN

const canvas = document.getElementById('radarCanvas');
const ctx = canvas.getContext('2d');

let satelitesGlobal = [];
let ultimoMensajeVoz = "";
const INCLINACION = 0.6; 
const OFFSET_TEXTO = 12; 

// --- 🧬 CONEXIÓN SINTERGIAL (POLLING DINÁMICO) ---
async function conectarSNC() {
    try {
        // Consultamos al endpoint que definimos en main.go
        const res = await fetch("/api/radar-pulse", {
            cache: "no-store",
            headers: { 'Accept': 'application/json' }
        });
        
        if (res.ok) {
            const data = await res.json();
            window.updateRadarData(data);
        }
    } catch (err) {
        console.warn("📡 [SNC]: Pulso perdido, reconectando...");
    } finally {
        // Mantiene el pulso constante cada 3 segundos
        setTimeout(conectarSNC, 3000);
    }
}

// --- 📥 ASIGNACIÓN SOBERANA Y ACTUALIZACIÓN DE INTERFAZ ---
window.updateRadarData = (data) => {
    satelitesGlobal = data.satelites || [];
    
    const elementos = {
        satCount: document.getElementById('sat-count'),
        iaMsg: document.getElementById('ia-text'),
        freqVal: document.getElementById('freq-val'),
        paxgVal: document.getElementById('paxg-val')
    };

    if (elementos.satCount) elementos.satCount.innerText = satelitesGlobal.length.toString();
    if (elementos.iaMsg) elementos.iaMsg.innerText = data.mensaje_ia || "Lattice Estable.";
    if (elementos.freqVal) elementos.freqVal.innerText = (data.frecuencia || 432.169).toFixed(3);
    if (elementos.paxgVal) elementos.paxgVal.innerText = (data.paxg || 15.15).toFixed(3);

    renderVisorLateral(satelitesGlobal);

    if (data.mensaje_ia && window.speechSynthesis) ejecutarVozSoberana(data.mensaje_ia);
};

// --- 📊 VISOR LATERAL Y RECALIBRACIÓN (Sin cambios, manteniendo tu lógica) ---
const renderVisorLateral = (satelites) => {
    const visor = document.getElementById('visor-telemetria');
    if (!visor) return;
    if (!satelites || satelites.length === 0) {
        visor.innerHTML = `<div class="log-entry">[ ESCANEANDO LATTICE... ]</div>`;
        return;
    }
    const enVuelo = satelites.filter(s => s.elevation > 0 || s.is_real);
    const enTierra = satelites.filter(s => (s.elevation <= 0) && !s.is_real);
    let htmlContent = "";
    if (enVuelo.length > 0) {
        htmlContent += `<h4 class="visor-header">🛰️ EN VUELO (${enVuelo.length})</h4>`;
        enVuelo.forEach(s => htmlContent += crearTarjeta(s, 'vuelo'));
    }
    if (enTierra.length > 0) {
        htmlContent += `<h4 class="visor-header tierra">📡 ESTACIONARIOS (${enTierra.length})</h4>`;
        enTierra.forEach(s => htmlContent += crearTarjeta(s, 'tierra'));
    }
    visor.innerHTML = htmlContent;
    visor.scrollTop = 0; 
};

const crearTarjeta = (sat, estado) => {
    const esMensajeMesh = sat.payload && sat.payload.length > 0;
    const colorPrimario = esMensajeMesh ? '#00ffff' : (sat.is_real ? '#00ff41' : '#d4af37');
    const nombreLimpio = sat.name.split('(')[0].trim();
    return `
        <div class="sat-card" style="border-left: 2px solid ${colorPrimario}; margin-bottom: 4px; padding: 5px; background: rgba(0,20,0,0.3);">
            <div style="display: flex; justify-content: space-between; font-size: 10px;">
                <span style="color: #fff;">[${nombreLimpio}]</span>
                <span style="color: ${colorPrimario}; font-size: 8px;">${esMensajeMesh ? 'MESH' : 'SDR'}</span>
            </div>
            <div style="color: rgba(0,255,65,0.6); font-size: 9px; font-family: monospace;">
                AZ:${Number(sat.azimuth||0).toFixed(1)}° EL:${Number(sat.elevation||0).toFixed(1)}°
            </div>
        </div>
    `;
};

function ajustarResolucion() {
    if (!canvas) return;
    const dpr = window.devicePixelRatio || 1;
    const rect = canvas.getBoundingClientRect();
    canvas.width = rect.width * dpr;
    canvas.height = rect.height * dpr;
    ctx.scale(dpr, dpr);
}

// ... [Mantén tu función draw() y ejecutarVozSoberana() originales abajo] ...

window.addEventListener('DOMContentLoaded', () => {
    ajustarResolucion();
    conectarSNC(); // Inicia el pulso
    draw();        // Inicia el renderizado
});