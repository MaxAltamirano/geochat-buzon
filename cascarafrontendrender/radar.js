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
function dibujar() {
    if (!ctx) return;
    
    actividad_usuario *= 0.98;
    mutacion_entropia = 1.0 + (actividad_usuario * 0.2);

    const centerX = canvas.width / 2;
    const centerY = canvas.height / 2;
    const radioBase = 200 * mutacion_entropia;

    ctx.clearRect(0, 0, canvas.width, canvas.height);
    
    // Dibujar anillos del Radar
    ctx.strokeStyle = '#00ff41';
    ctx.lineWidth = 1;
    for (let i = 1; i <= 3; i++) {
        ctx.beginPath();
        ctx.arc(centerX, centerY, (radioBase / 3) * i, 0, Math.PI * 2);
        ctx.stroke();
    }

    // Brazo de rotación
    const tiempo = Date.now() / 1000;
    const angulo = tiempo * mutacion_entropia;
    ctx.beginPath();
    ctx.moveTo(centerX, centerY);
    ctx.lineTo(centerX + Math.cos(angulo) * radioBase, centerY + Math.sin(angulo) * radioBase);
    ctx.stroke();

    // Dibujar Satélites/Vuelos detectados
    ctx.fillStyle = '#d4af37';
    satelitesGlobal.forEach((s, idx) => {
        // Mapeo de azimut a coordenadas circulares
        const rad = (parseFloat(s.azimuth || 0) * Math.PI) / 180;
        const x = centerX + Math.cos(rad) * (radioBase * 0.8);
        const y = centerY + Math.sin(rad) * (radioBase * 0.8);
        
        ctx.beginPath();
        ctx.arc(x, y, 4, 0, Math.PI * 2);
        ctx.fill();
    });

    requestAnimationFrame(dibujar);
}

function iniciarMotorRadar() {
    console.log("🔱 [SNC]: Motor unificado activo. Sintonizando 432Hz...");
    conectarSNC();
    dibujar();
}

window.iniciarMotorRadar = iniciarMotorRadar;