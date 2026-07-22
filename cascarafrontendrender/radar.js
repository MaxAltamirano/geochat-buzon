/**
 * DNA_ID: RADAR_JS_SNC_FUSION_FINAL | ORGAN: VISION-SNC | RESONANCE: 432Hz
 * Arquitectura unificada: Telemetría de Red + Mutación Biométrica de Arquitecto.
 */


// Variable de estado global para persistencia entre frames
let satelitesGlobal = [];

// --- 🎨 DEFINICIÓN DE COLORES SOBERANOS ---
const PALETA = {
    AEREO: '#00ff41',    // Verde Neón (Aviones/Satélites)
    LLAVERO: '#d4af37',  // Oro (Llaveros)
    MOVIL: '#00ccff'     // Cian (Celulares)
};

const canvas = document.getElementById('radarCanvas');
const ctx = canvas.getContext ? canvas.getContext('2d') : null;

// --- 🧬 VARIABLES DE ESTADO ---
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

/**
 * 📡 MOTOR DE RENDERIZADO [SNC] - FUSIÓN FINAL
 */

// 1. Eliminamos el fetch interno y dejamos que monitorPulse inyecte los datos
window.updateRadarData = (data) => {
    // Recibimos los datos desde el monitor externo (index.html)
    satelitesGlobal = data.Satelites || data.satelites || [];
    
    // Actualizamos visor lateral
    actualizarVisorLateral(satelitesGlobal);
};

// 2. Función dibujar: Pura cinemática y renderizado (sin Fecth)
async function dibujar() {
    if (!ctx || canvas.width === 0) {
        requestAnimationFrame(dibujar);
        return;
    }

    // [MANTÉN AQUÍ TODO TU CÓDIGO DE DIBUJO DE ANILLOS, BRAZO Y NODOS]
    // ...
    
    requestAnimationFrame(dibujar);
}

// 3. Inicio del motor simplificado
window.iniciarMotorRadar = () => {
    if (motorCorriendo) return;
    motorCorriendo = true;
    console.log("🚀 [SNC]: Motor de radar activado (Modo Pasivo).");
    dibujar();
};


const renderVisorLateral = (items) => {
    const visor = document.getElementById('visor-telemetria');
    if (!visor) return;

    visor.innerHTML = items.length > 0 ?
        items.map(s => {
            let color = PALETA.AEREO;
            if (s.name.includes("LLAVERO")) color = PALETA.LLAVERO;
            if (s.name.includes("MOVIL")) color = PALETA.MOVIL;

            return `
            <div class="log-entry" style="border-bottom: 1px solid #111; margin-bottom: 8px; padding: 5px; text-align: left; border-left: 3px solid ${color};">
                <span style="color: ${color}; font-weight: bold;">> ${s.name || 'OBJETO'}</span><br>
                <small style="color: #888;">AZ: ${parseFloat(s.azimuth || 0).toFixed(0)}° | ALT: ${parseFloat(s.altitud || 0).toFixed(0)}km</small>
            </div>`;
        }).join('') :
        `<div class="log-entry">[ ESCANEANDO LATTICE... ]</div>`;
};

// --- 🎨 MOTOR DE RENDERIZADO ---

function iniciarMotorRadar() {
    console.log("🔱 [SNC]: Motor unificado activo. Sintonizando 432Hz...");
    conectarSNC();
    dibujar();
}


/**
 * 📡 MOTOR DE RENDERIZADO [SNC] - VERSIÓN FINAL
 * Responsabilidad: Visualización fluida de la Lattice GeoChat.
 * Optimización: Bucle de renderizado independiente de la red.
 */
async function dibujar() {
    // 0. Protección de contexto y validación de Lattices
    if (!ctx || canvas.width === 0) {
        requestAnimationFrame(dibujar);
        return;
    }

    // 1. Cálculo de Entropía y Cinemática
    // La actividad_usuario es gestionada externamente por eventos (mouse/teclado)
    actividad_usuario *= 0.95;
    const mutacion_entropia = 1.0 + Math.min(actividad_usuario * 0.1, 0.5);

    const centerX = canvas.width / 2;
    const centerY = canvas.height / 2;
    const radioBase = 200;

    // 2. Limpieza del Frame (Sincronía 432Hz)
    ctx.clearRect(0, 0, canvas.width, canvas.height);

    // 3. Dibujo de Anillos de resonancia
    ctx.strokeStyle = 'rgba(0, 255, 65, 0.3)';
    ctx.lineWidth = 0.5;
    for (let i = 1; i <= 3; i++) {
        ctx.beginPath();
        ctx.arc(centerX, centerY, (radioBase / 3) * i, 0, Math.PI * 2);
        ctx.stroke();
    }

    // 4. Renderizado del Brazo de rotación
    const tiempo = Date.now() / 1000;
    const anguloBrazo = tiempo * mutacion_entropia;
    ctx.strokeStyle = 'rgba(212, 175, 55, 0.8)';
    ctx.lineWidth = 2;
    ctx.beginPath();
    ctx.moveTo(centerX, centerY);
    ctx.lineTo(centerX + Math.cos(anguloBrazo) * radioBase, centerY + Math.sin(anguloBrazo) * radioBase);
    ctx.stroke();

    // 5. Renderizado dinámico de la Lattice (Usando estado global cacheado por conectarSNC)
    satelitesGlobal.forEach((s) => {
        let color = PALETA.AEREO; 
        if (s.name.includes("LLAVERO")) color = PALETA.LLAVERO;
        if (s.name.includes("MOVIL")) color = PALETA.MOVIL;

        const az = parseFloat(s.azimuth || 0);
        const rad = (az - 90) * (Math.PI / 180);

        const x = centerX + Math.cos(rad) * (radioBase * 0.85);
        const y = centerY + Math.sin(rad) * (radioBase * 0.85);

        // Nodos
        ctx.fillStyle = color;
        ctx.beginPath();
        ctx.arc(x, y, 5, 0, Math.PI * 2);
        ctx.fill();

        // Etiquetas
        ctx.fillStyle = color;
        ctx.font = '10px Courier New';
        ctx.textAlign = 'left';
        ctx.textBaseline = 'middle';
        ctx.fillText(s.name || 'NODO', x + 8, y + 3);
    });

    // 6. Ciclo infinito de alta frecuencia
    requestAnimationFrame(dibujar);
}




// Llamar a esta función cuando los datos de satélites cambien
// --- 📋 ACTUALIZADOR DE TELEMETRÍA ---
// Variable global para persistencia de estado (debe ir fuera de la función)
let estadoUltimo = "";

function actualizarVisorLateral(items) {
    const visor = document.getElementById('visor-telemetria');
    if (!visor) return;

    // Generamos el nuevo HTML basado en los datos actuales
    const nuevoHTML = items.length > 0 ?
        items.map(s => `
            <div class="log-entry" style="border-bottom: 1px solid #003300; margin-bottom: 8px; padding: 5px; text-align: left;">
                <span style="color: #00ff41; font-weight: bold;">> ${s.name || 'OBJETO'}</span><br>
                <small style="color: #888;">AZ: ${parseFloat(s.azimuth || 0).toFixed(0)}° | H: ${s.horario || 'N/A'}</small>
            </div>`).join('') :
        `<div class="log-entry">[ ESCANEANDO LATTICE... ]</div>`;

    // Comparación de estado para evitar refresco innecesario del DOM
    if (nuevoHTML !== estadoUltimo) {
        visor.innerHTML = nuevoHTML;
        estadoUltimo = nuevoHTML;
    }
}

// Ejemplo de uso: actualiza el panel cada 2 segundos para no sobrecargar
setInterval(() => {
    actualizarVisorLateral(satelitesGlobal);
}, 2000);

// ÚNICO punto de entrada expuesto a la ventana (Global)
let motorCorriendo = false;

// 3. Inicio del motor simplificado
window.iniciarMotorRadar = () => {
    if (motorCorriendo) return;
    motorCorriendo = true;
    console.log("🚀 [SNC]: Motor de radar activado (Modo Pasivo).");
    dibujar();
};

