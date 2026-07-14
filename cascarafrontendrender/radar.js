/**
 * DNA_ID: RADAR_JS_SNC_FUSION_FINAL | ORGAN: VISION-SNC | RESONANCE: 432Hz
 * Arquitectura unificada: Telemetría de Red + Mutación Biométrica de Arquitecto.
 */

const canvas = document.getElementById('radarCanvas');
const ctx = canvas.getContext ? canvas.getContext('2d') : null;

// --- 🧬 VARIABLES DE ESTADO ---
let satelitesGlobal = [];
let mutacion_entropia = 1.0;
let actividad_usuario = 0;

// --- 🖱️ TRANSDUCTOR BIOLÓGICO ---
window.addEventListener('mousemove', (e) => {
    actividad_usuario = Math.min(actividad_usuario + 0.1, 2.0);
});

window.addEventListener('keydown', () => {
    actividad_usuario = 2.5;
});

/**
 * 📡 CONEXIÓN SINTERGIAL
 * Protocolo de verificación mediante Buzón (Render)
 */
async function conectarSNC() {
    try {
        const res = await fetch("https://geochat-buzon.onrender.com/api/estado-global", {
            cache: "no-store",
            headers: { 'Accept': 'application/json' }
        });

        if (!res.ok) throw new Error(`HTTP Error: ${res.status}`);

        const data = await res.json();
        const modoDisplay = document.querySelector('#radar-container h1');

        if (data && data.status === "ONLINE") {
            if (modoDisplay) {
                modoDisplay.innerText = "🔱 SNC: ONLINE-SINTÉRGICO";
                modoDisplay.style.color = "#d4af37";
            }
            // Actualización de datos entrantes (satélites/vuelos)
            window.updateRadarData(data);
        }
    } catch (err) {
        console.warn("📡 [SNC]: Pulso perdido. Reconectando...");
    } finally {
        setTimeout(conectarSNC, 5000);
    }
}

// --- 📥 PROCESAMIENTO DE DATOS (Telemetría de Vuelos y Satélites) ---
window.updateRadarData = (data) => {
    // Aseguramos que satelitesGlobal siempre sea un array
    satelitesGlobal = data.satelites || data.vuelos || [];

    const elements = {
        satCount: document.getElementById('sat-count'),
        freqVal: document.getElementById('freq-val')
    };

    if (elements.satCount) elements.satCount.innerText = satelitesGlobal.length.toString();
    if (elements.freqVal && data.frecuencia) elements.freqVal.innerText = parseFloat(data.frecuencia).toFixed(2);

    renderVisorLateral(satelitesGlobal);
};

const renderVisorLateral = (items) => {
    const visor = document.getElementById('visor-telemetria');
    if (!visor) return;

    visor.innerHTML = items.length > 0 ?
        items.map(s => `
            <div class="log-entry" style="border-bottom: 1px solid #003300; margin-bottom: 4px;">
                🛰️ ${s.name || 'OBJETO'} | AZ:${parseFloat(s.azimuth || 0).toFixed(0)}°<br>
                <span style="font-size: 8px; color: #888;">ALT:${parseFloat(s.altitud || 0).toFixed(0)}km</span>
            </div>`).join('') :
        `<div class="log-entry">[ ESCANEANDO LATTICE... ]</div>`;
};

// --- 🎨 MOTOR DE RENDERIZADO ---

function iniciarMotorRadar() {
    console.log("🔱 [SNC]: Motor unificado activo. Sintonizando 432Hz...");
    conectarSNC();
    dibujar();
}

// --- 🎨 MOTOR DE RENDERIZADO Y SINTONÍA ---

function dibujar() {
    // Protección: Si el canvas no existe o no tiene tamaño, esperamos al siguiente frame
    if (!ctx || canvas.width === 0) {
        requestAnimationFrame(dibujar);
        return;
    }
    
    // Amortiguación de Entropía
    actividad_usuario *= 0.95; 
    mutacion_entropia = 1.0 + Math.min(actividad_usuario * 0.1, 0.5);

    const centerX = canvas.width / 2;
    const centerY = canvas.height / 2;
    const radioBase = 200;

    ctx.clearRect(0, 0, canvas.width, canvas.height);
    
    // 1. Anillos de resonancia
    ctx.strokeStyle = 'rgba(0, 255, 65, 0.3)';
    ctx.lineWidth = 0.5;
    for (let i = 1; i <= 3; i++) {
        ctx.beginPath();
        ctx.arc(centerX, centerY, (radioBase / 3) * i, 0, Math.PI * 2);
        ctx.stroke();
    }

    // 2. Brazo de rotación (Sincronizado a 432Hz)
    const tiempo = Date.now() / 1000;
    const anguloBrazo = tiempo * mutacion_entropia;
    ctx.strokeStyle = 'rgba(212, 175, 55, 0.8)';
    ctx.lineWidth = 2;
    ctx.beginPath();
    ctx.moveTo(centerX, centerY);
    ctx.lineTo(centerX + Math.cos(anguloBrazo) * radioBase, centerY + Math.sin(anguloBrazo) * radioBase);
    ctx.stroke();

    // 3. Renderizado de satélites
    satelitesGlobal.forEach((s) => {
        // Conversión a radianes (ajustado a -90° para alineación Norte)
        const az = parseFloat(s.azimuth || 0);
        const rad = (az - 90) * (Math.PI / 180);
        
        const x = centerX + Math.cos(rad) * (radioBase * 0.85);
        const y = centerY + Math.sin(rad) * (radioBase * 0.85);
        
        // Dibujar punto del satélite
        ctx.fillStyle = '#00ff41';
        ctx.beginPath();
        ctx.arc(x, y, 4, 0, Math.PI * 2);
        ctx.fill();
        
        // Etiqueta de nombre
        ctx.fillStyle = '#fff';
        ctx.font = '10px Courier New';
        ctx.fillText(s.name || 'SAT', x + 8, y + 3);
    });

    requestAnimationFrame(dibujar);
}

// Llamar a esta función cuando los datos de satélites cambien
function actualizarVisorLateral(items) {
    const visor = document.getElementById('visor-telemetria');
    if (!visor) return;

    visor.innerHTML = items.length > 0 ?
        items.map(s => `
            <div class="log-entry" style="border-bottom: 1px solid #003300; margin-bottom: 8px; padding: 5px;">
                <span style="color: #00ff41;">> ${s.nombre || 'OBJETO'}</span><br>
                <small style="color: #888;">AZ: ${s.azimuth}° | H: ${s.horario}</small>
            </div>`).join('') :
        `<div class="log-entry">[ ESCANEANDO LATTICE... ]</div>`;
}

// Ejemplo de uso: actualiza el panel cada 2 segundos para no sobrecargar
setInterval(() => {
    actualizarVisorLateral(satelitesGlobal);
}, 2000);

// ÚNICO punto de entrada expuesto a la ventana (Global)
window.iniciarMotorRadar = () => {
    console.log("🚀 [SNC]: Motor de radar activado y sintonizado.");
    // Iniciamos la conexión y el bucle de render una sola vez
    conectarSNC();
    dibujar();
};

