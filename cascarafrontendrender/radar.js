/**
 * DNA_ID: RADAR_JS_SNC_LAVEROS_SIM_PRO | ORGAN: VISION-SNC | RESONANCE: 432Hz
 * Arquitectura unificada: Telemetría de Llaveros SIM Soberanos, Firmas ECDSA y Visor Sintérgico.
 */

// --- 🧬 VARIABLES DE ESTADO GLOBAL ---
let llaverosGlobal = [];
let motorCorriendo = false;
let mutacion_entropia = 1.0;
let actividad_usuario = 0;
let estadoUltimo = "";

// --- 🎨 PALETA SOBERANA DE LLAVEROS ---
const PALETA = {
    LLAVERO_ACTIVO: '#d4af37',  // Oro (Llaveros SIM validados por firma criptográfica)
    SIM_MOVIL: '#00ccff',      // Cian (Terminales de nodo en movimiento)
    GRID_BASE: '#00ff41'       // Verde Neón (Infraestructura de red general)
};

const canvas = document.getElementById('radarCanvas');
const ctx = canvas.getContext ? canvas.getContext('2d') : null;

// --- 🖱️ TRANSDUCTOR BIOLÓGICO Y ENTROPÍA ---
window.addEventListener('mousemove', () => {
    actividad_usuario = Math.min(actividad_usuario + 0.1, 2.0);
});

window.addEventListener('keydown', () => {
    actividad_usuario = 2.5;
});

/**
 * 📡 CONEXIÓN SINTERGIAL CON EL BÚZON / ENDPOINT GLOBAL
 */
/**
 * 📡 CONEXIÓN SINTERGIAL CON EL BÚZON / ENDPOINT GLOBAL
 */
async function conectarSNC() {
    try {
        const res = await fetch("https://geochat-buzon.onrender.com/api/estado-global", {
            cache: "no-store",
            headers: { 'Accept': 'application/json' }
        });

        if (!res.ok) throw new Error(`HTTP Error: ${res.status}`);

        const data = await res.json();
        const modoDisplay = document.querySelector('#radar-container h1') || document.querySelector('h1');

        // Procesamos siempre que el estado sea activo (SYNCING, ONLINE o vacío por defecto)
        if (data && data.status !== "OFFLINE") {
            const estadoActual = data.status || "SYNCING";
            if (modoDisplay) {
                modoDisplay.innerText = `🔱 SNC: NODO ${data.nodo || 'Avellaneda'} (${estadoActual})`;
                modoDisplay.style.color = "#d4af37";
            }
            // Forzamos la actualización inmediata del array de llaveros sin importar variaciones de nombres
            window.updateRadarData(data);
        } else {
            if (modoDisplay) {
                modoDisplay.innerText = "⚠️ SNC: MODO OFFLINE (LATIDO PERDIDO)";
                modoDisplay.style.color = "#ff4444";
            }
        }
    } catch (err) {
        console.warn("📡 [SNC]: Pulso perdido con el buzón. Reintentando...");
    } finally {
        setTimeout(conectarSNC, 5000);
    }
}

/**
 * 📡 ACTUALIZADOR DE TELEMETRÍA DE LLAVEROS SIM
 */
/**
 * 📡 ACTUALIZADOR DE TELEMETRÍA DE LLAVEROS SIM
 */
window.updateRadarData = (data) => {
    if (!data) return;

    // Sincronización exacta con la estructura emitida por Go (`Llaveros_SIM`)
    llaverosGlobal = data.Llaveros_SIM || data.llaveros || data.Satelites || [];

    // Actualización del visor lateral de telemetría en el DOM
    actualizarVisorLateral(llaverosGlobal);
};

function actualizarVisorLateral(items) {
    const visor = document.getElementById('visor-telemetria');
    if (!visor) return;

    const nuevoHTML = items.length > 0 ?
        items.map(s => {
            let color = PALETA.LLAVERO_ACTIVO;
            if (s.name && s.name.includes("MOVIL")) color = PALETA.SIM_MOVIL;

            // Extraemos una porción limpia de la firma hexadecimal si existe
            const firmaLimpia = s.firma ? s.firma.replace('... (ECDSA Hex)', '') : '';
            const firmaCorta = firmaLimpia ? firmaLimpia.substring(0, 12) + "..." : "PENDIENTE";

            return `
            <div class="log-entry" style="border-bottom: 1px solid #332700; margin-bottom: 8px; padding: 6px; text-align: left; border-left: 3px solid ${color}; background: rgba(0,0,0,0.4);">
                <span style="color: ${color}; font-weight: bold; font-family: 'Courier New', monospace;">> ${s.name || 'LLAVERO_SIM'}</span><br>
                <small style="color: #aaa;">AZ: ${parseFloat(s.azimuth || 0).toFixed(0)}° | RSSI: ${s.rssi || '-65'}dBm</small><br>
                <small style="color: #d4af37; font-family: monospace;">SIG: ${firmaCorta}</small>
            </div>`;
        }).join('') :
        `<div class="log-entry" style="color: #888; font-family: 'Courier New', monospace;">[ ESCANEANDO LATTICE DE LLAVEROS SIM... ]</div>`;

    if (nuevoHTML !== estadoUltimo) {
        visor.innerHTML = nuevoHTML;
        estadoUltimo = nuevoHTML;
    }
}

/**
 * 🎨 MOTOR DE RENDERIZADO DEL RADAR [SNC] (Frecuencia 432Hz)
 */
async function dibujar() {
    if (!ctx || !canvas || canvas.width === 0) {
        requestAnimationFrame(dibujar);
        return;
    }

    actividad_usuario *= 0.95;
    const entropiaActual = 1.0 + Math.min(actividad_usuario * 0.1, 0.5);

    const centerX = canvas.width / 2;
    const centerY = canvas.height / 2;
    const radioBase = Math.min(centerX, centerY) * 0.85;

    ctx.clearRect(0, 0, canvas.width, canvas.height);

    // 1. Anillos de resonancia soberana
    ctx.strokeStyle = 'rgba(212, 175, 55, 0.2)';
    ctx.lineWidth = 0.5;
    for (let i = 1; i <= 3; i++) {
        ctx.beginPath();
        ctx.arc(centerX, centerY, (radioBase / 3) * i, 0, Math.PI * 2);
        ctx.stroke();
    }

    // 2. Brazo de escaneo dinámico
    const tiempo = Date.now() / 1000;
    const anguloBrazo = tiempo * entropiaActual;
    ctx.strokeStyle = 'rgba(212, 175, 55, 0.8)';
    ctx.lineWidth = 2;
    ctx.beginPath();
    ctx.moveTo(centerX, centerY);
    ctx.lineTo(centerX + Math.cos(anguloBrazo) * radioBase, centerY + Math.sin(anguloBrazo) * radioBase);
    ctx.stroke();

    // 3. Renderizado de los Llaveros SIM sobre la retícula del radar
    llaverosGlobal.forEach((s) => {
        let color = PALETA.LLAVERO_ACTIVO;
        if (s.name && s.name.includes("MOVIL")) color = PALETA.SIM_MOVIL;

        const az = parseFloat(s.azimuth || 0);
        const rad = (az - 90) * (Math.PI / 180);

        const x = centerX + Math.cos(rad) * (radioBase * 0.80);
        const y = centerY + Math.sin(rad) * (radioBase * 0.80);

        // Nodo físico en el radar
        ctx.fillStyle = color;
        ctx.beginPath();
        ctx.arc(x, y, 5, 0, Math.PI * 2);
        ctx.fill();

        // Etiqueta del Llavero SIM
        ctx.fillStyle = color;
        ctx.font = '10px Courier New';
        ctx.textAlign = 'left';
        ctx.textBaseline = 'middle';
        ctx.fillText(`${s.name || 'SIM'}`, x + 8, y - 4);
        
        // Mini extracto de firma criptográfica bajo el nodo
        if (s.firma) {
            const firmaLimpia = s.firma.replace('... (ECDSA Hex)', '');
            ctx.fillStyle = '#888';
            ctx.font = '8px Courier New';
            ctx.fillText(`[${firmaLimpia.substring(0, 6)}...]`, x + 8, y + 6);
        }
    });

    requestAnimationFrame(dibujar);
}

// --- 🚀 PUNTO DE ENTRADA ÚNICO ---
window.iniciarMotorRadar = () => {
    if (motorCorriendo) return;
    motorCorriendo = true;
    console.log("🚀 [SNC]: Motor de radar de Llaveros SIM activado. Sintonizando 432Hz...");
    conectarSNC();
    dibujar();
};