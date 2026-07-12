/**
 * DNA_ID: RADAR_JS_SNC_FUSION_FINAL | ORGAN: VISION-SNC | RESONANCE: 432Hz
 * Arquitectura unificada: Telemetría de Red + Mutación Biométrica de Arquitecto.
 */

const canvas = document.getElementById('radarCanvas');
const ctx = canvas.getContext('2d');

// --- 🧬 VARIABLES DE ESTADO Y MUTACIÓN ---
let satelitesGlobal = [];
let mutacion_entropia = 1.0;
let actividad_usuario = 0;

// --- 🖱️ TRANSDUCTOR BIOLÓGICO (Eventos del Arquitecto) ---
window.addEventListener('mousemove', (e) => {
    actividad_usuario = Math.min(actividad_usuario + 0.1, 2.0);
    mutacion_entropia = 1.0 + (e.clientX / window.innerWidth) * 0.5;
});

window.addEventListener('keydown', () => {
    actividad_usuario = 2.5;
    mutacion_entropia = 1.8;
});


/**
 * 📡 CONEXIÓN SINTERGIAL (Polling al Cortex vía Buzón)
 * DNA_ID: RADAR_JS_SNC_FUSION_FINAL
 * Protocolo de verificación mediante Buzón (Render)
 */
async function conectarSNC() {
    try {
        // Consultamos al Buzón central en Render para saber si el Córtex está vivo
        const res = await fetch("https://geochat-buzon.onrender.com/api/estado-global", {
            cache: "no-store",
            headers: { 'Accept': 'application/json' }
        });

        if (!res.ok) throw new Error(`HTTP Error: ${res.status}`);

        const data = await res.json();
        const modoDisplay = document.querySelector('#radar-container h1');

        // 🧬 LÓGICA DE FUSIÓN SOBERANA
        if (data && data.status === "ONLINE") {
            // Actualización de estado visual positivo
            if (modoDisplay) {
                modoDisplay.innerText = "🔱 SNC: ONLINE-SINTÉRGICO";
                modoDisplay.style.color = "#d4af37";
                modoDisplay.style.textShadow = "0 0 15px #d4af37";
            }
            
            // Inyección de datos al motor de renderizado
            if (data.frecuencia || data.satelites) {
                window.updateRadarData(data);
            }
            
            console.log(`✅ [SISTEMA]: Iron Grid en sintonía. Estado: ${data.mode || 'ACTIVO'}`);
        } else {
            // Estado donde el Buzón responde pero el Córtex está en modo inactivo
            console.warn("⚠️ [SISTEMA]: Buzón operativo, pero el Córtex local reporta inactividad.");
            if (modoDisplay) modoDisplay.innerText = "🔱 SNC: STANDBY / SILENCIO";
        }

    } catch (err) {
        // Fallo de conexión o tiempo de espera
        console.warn("📡 [SNC]: Pulso perdido. Intentando reconexión en el siguiente ciclo...");
        const modoDisplay = document.querySelector('#radar-container h1');
        if (modoDisplay) {
            modoDisplay.innerText = "🔱 SNC: OFFLINE / BUSCANDO...";
            modoDisplay.style.color = "#666";
            modoDisplay.style.textShadow = "none";
        }
    } finally {
        // Ciclo de vida: 5 segundos de intervalo para mantener la latencia baja sin saturar la red
        setTimeout(conectarSNC, 5000); 
    }
}

// --- 📥 PROCESAMIENTO DE DATOS ---
window.updateRadarData = (data) => {
    satelitesGlobal = data.satelites || [];
    const ids = {
        satCount: document.getElementById('sat-count'),
        freqVal: document.getElementById('freq-val'),
        paxgVal: document.getElementById('paxg-val')
    };

    if (ids.satCount) ids.satCount.innerText = satelitesGlobal.length.toString();
    if (ids.freqVal) ids.freqVal.innerText = (data.frecuencia || 432.00).toFixed(2);
    if (ids.paxgVal) ids.paxgVal.innerText = (data.paxg || 15.15).toFixed(2);

    renderVisorLateral(satelitesGlobal);
};

const renderVisorLateral = (satelites) => {
    const visor = document.getElementById('visor-telemetria');
    if (!visor) return;
    visor.innerHTML = satelites.length > 0 ?
        satelites.map(s => `<div class="log-entry">🛰️ ${s.name} | AZ:${Number(s.azimuth).toFixed(0)}°</div>`).join('') :
        `<div class="log-entry">[ ESCANEANDO LATTICE... ]</div>`;
};

// --- 🎨 MOTOR DE RENDERIZADO (Ciclo de Vida) ---
function iniciarMotorRadar() {
    console.log("🔱 [SNC]: Motor unificado activo. Sintonizando 432Hz...");
    conectarSNC();
    dibujar();
}

function dibujar() {
    monitorSNC();
    actividad_usuario *= 0.98;
    mutacion_entropia = 1.0 + (actividad_usuario * 0.2);

    if (!canvas) return;
    const centerX = canvas.width / 2;
    const centerY = canvas.height / 2;
    const radioBase = 200 * mutacion_entropia;

    ctx.clearRect(0, 0, canvas.width, canvas.height);
    ctx.strokeStyle = '#00ff41';
    ctx.lineWidth = 1;

    for (let i = 1; i <= 3; i++) {
        ctx.beginPath();
        ctx.arc(centerX, centerY, (radioBase / 3) * i, 0, Math.PI * 2);
        ctx.stroke();
    }

    const tiempo = Date.now() / 1000;
    const angulo = tiempo * mutacion_entropia;
    ctx.beginPath();
    ctx.moveTo(centerX, centerY);
    ctx.lineTo(centerX + Math.cos(angulo) * radioBase, centerY + Math.sin(angulo) * radioBase);
    ctx.stroke();

    ctx.fillStyle = '#d4af37';
    satelitesGlobal.forEach((s, idx) => {
        const x = centerX + Math.cos(tiempo + idx) * (radioBase * 0.8);
        const y = centerY + Math.sin(tiempo + idx) * (radioBase * 0.8);
        ctx.beginPath();
        ctx.arc(x, y, 3, 0, Math.PI * 2);
        ctx.fill();
    });

    const mindStatus = document.getElementById('mind-status');
    if (mindStatus) {
        mindStatus.innerText = actividad_usuario > 0.5 ? "MUTANDO SNC..." : "SINCRO_OK";
    }

    requestAnimationFrame(dibujar);
}

// --- 👁️ OJO DEL SNC: MONITORIZACIÓN DE SEGURIDAD ---
function monitorSNC() {
    const esBarridoExterno = (Math.random() < 0.001);
    if (esBarridoExterno) {
        console.warn("🔱 [SNC]: Observación detectada. Activando protocolo filantrópico.");
        activarModoTesla();
    }
}

function activarModoTesla() {
    const eventoTesla = new CustomEvent('abrir-tesla', {
        detail: {
            titulo: "PROTOCOLO FILANTRÓPICO",
            detalle: "Se ha detectado una observación externa. El sistema se proyecta en modo transparente y soberano.",
            specs: { "MODO": "TESLA-KIMI", "ESTADO": "PROTECCIÓN" }
        }
    });
    window.dispatchEvent(eventoTesla);
}

window.iniciarMotorRadar = iniciarMotorRadar;