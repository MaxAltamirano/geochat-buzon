<template>
  <Transition name="fade-warp">
    <div v-if="isVisible" class="tesla-stage" @click.self="closePortal">
      <div class="resonance-grid"></div>

      <div class="tesla-container">
        <div class="tesla-header">
          <div class="status-blink" :class="{ 'speaking': isSpeaking }"></div>
          <span class="gold-text">TRANSYSTEM: IA CEO (TESLA/KIMI)</span>
          <button class="close-btn" @click="closePortal">X</button>
        </div>

        <div class="tesla-body">
          <div class="avatar-section">
            <div class="avatar-glow" :class="{ 'speaking': isSpeaking }">
              <img src="/tesla_kimi.png" alt="Tesla AI" class="tesla-head" :class="{ 'pulse-active': isSpeaking }">
            </div>
            <div v-if="isListening" class="listening-indicator">ESCANEANDO LATTICE...</div>
            <div class="audio-visualizer">
              <div v-for="n in 12" :key="n" class="bar" :class="{ 'active': isSpeaking }"></div>
            </div>
          </div>

          <div class="info-section">
            <h2 class="matrix-title">{{ activeModule.titulo }}</h2>
            <p class="tesla-speech">{{ activeModule.detalle }}</p>
            
            <div class="tech-specs" v-if="activeModule.specs">
              <div v-for="(val, key) in activeModule.specs" :key="key" class="spec-item">
                <span class="spec-label">{{ key }}:</span>
                <span class="spec-value">{{ val }}</span>
              </div>
            </div>
            
            <button class="mic-btn" @click="activarMicrofono" :class="{ 'listening': isListening }">
              {{ isListening ? 'ESCUCHANDO...' : 'HABLAR CON KIMI' }}
            </button>
          </div>
        </div>

        <div class="tesla-footer">
          <span class="paxg-indicator">RESERVA PAXG: 1.4320</span>
          <span class="sovereign-tag">15% DEL PUEBLO VALIDATED</span>
        </div>
      </div>
    </div>
  </Transition>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue';

const props = defineProps<{
  isVisible: boolean;
  activeModule: { titulo: string; detalle: string; specs?: Record<string, string>; };
}>();

const emit = defineEmits(['close']);
const isSpeaking = ref(false);
const isListening = ref(false);

// --- AUDIO & VOZ ---
const audioHendrix = new Audio('/hendrix_432.mp3');
audioHendrix.loop = true;
audioHendrix.volume = 0.3;

const synth = window.speechSynthesis;
const Recognition = (window as any).SpeechRecognition || (window as any).webkitSpeechRecognition;
const recognition = Recognition ? new Recognition() : null;

const hablar = (texto: string) => {
  const utterance = new SpeechSynthesisUtterance(texto);
  utterance.lang = 'es-AR';
  utterance.onstart = () => isSpeaking.value = true;
  utterance.onend = () => isSpeaking.value = false;
  synth.speak(utterance);
};

const activarMicrofono = () => {
  if (!recognition) return alert("Navegador no compatible.");
  isListening.value = true;
  recognition.start();
  recognition.onresult = (event: any) => {
    isListening.value = false;
    const transcript = event.results[0][0].transcript;
    console.log("Comando:", transcript);
    hablar("Entendido, procesando " + transcript); // Respuesta simulada
  };
};

const closePortal = () => {
  audioHendrix.pause();
  emit('close');
};

onMounted(() => {
  if (props.isVisible) {
    audioHendrix.play().catch(() => {});
    hablar("Sistema en línea. ¿Qué necesita el nodo?");
  }
});
</script>

<style scoped>
.tesla-stage {
  position: fixed; top: 0; left: 0; width: 100vw; height: 100vh;
  background: rgba(0, 5, 10, 0.98); z-index: 9999;
  display: flex; justify-content: center; align-items: center;
  backdrop-filter: blur(10px);
}

.avatar-glow.speaking { box-shadow: 0 0 30px #00ff41; border-radius: 50%; }
.tesla-head.pulse-active { animation: pulse-talk 0.5s infinite; }

@keyframes pulse-talk { 0%, 100% { transform: scale(1); } 50% { transform: scale(1.05); } }

.mic-btn { 
  margin-top: 20px; padding: 10px 20px; background: transparent; 
  border: 1px solid #00ff41; color: #00ff41; cursor: pointer; 
}
.mic-btn.listening { background: #00ff41; color: black; }

.listening-indicator { color: #00ff41; font-family: monospace; text-align: center; margin-top: 10px; }

/* Mantener el resto de tu CSS original debajo... */
</style>