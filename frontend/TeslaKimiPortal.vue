<template>
  <Transition name="fade-warp">
    <div v-if="isVisible" class="tesla-stage" @click.self="closePortal">
      <div class="resonance-grid"></div>

      <div class="tesla-container">
        <div class="tesla-header">
          <div class="status-blink"></div>
          <span class="gold-text">TRANSYSTEM: IA CEO (TESLA/KIMI)</span>
          <button class="close-btn" @click="closePortal">X</button>
        </div>

        <div class="tesla-body">
          <div class="avatar-section">
            <div class="avatar-glow">
              <img src="/tesla_kimi.png" alt="Tesla AI" class="tesla-head">
            </div>
            <div class="audio-visualizer">
              <div v-for="n in 12" :key="n" class="bar"></div>
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

// --- PROPS & EMITS ---
const props = defineProps<{
  isVisible: boolean;
  activeModule: {
    titulo: string;
    detalle: string;
    specs?: Record<string, string>;
  };
}>();

const emit = defineEmits(['close']);

// --- AUDIO HENDRIX (Frecuencia 432Hz) ---
const audioHendrix = new Audio('/hendrix_432.mp3');
audioHendrix.loop = true;
audioHendrix.volume = 0.4;

const closePortal = () => {
  audioHendrix.pause();
  audioHendrix.currentTime = 0;
  emit('close');
};

// --- WATCHERS PARA EL SALTO SÓNICO ---
onMounted(() => {
  if (props.isVisible) audioHendrix.play().catch(() => console.log("Waiting for user gesture"));
});

onUnmounted(() => {
  audioHendrix.pause();
});
</script>

<style scoped>
.tesla-stage {
  position: fixed; top: 0; left: 0; width: 100vw; height: 100vh;
  background: rgba(0, 5, 10, 0.98); z-index: 9999;
  display: flex; justify-content: center; align-items: center;
  backdrop-filter: blur(10px); cursor: crosshair;
}

.resonance-grid {
  position: absolute; width: 100%; height: 100%;
  background-image: linear-gradient(rgba(0, 255, 65, 0.1) 1px, transparent 1px),
                    linear-gradient(90deg, rgba(0, 255, 65, 0.1) 1px, transparent 1px);
  background-size: 50px 50px; transform: perspective(500px) rotateX(60deg);
  animation: move-grid 10s linear infinite; opacity: 0.3;
}

.tesla-container {
  width: 90%; max-width: 900px;
  background: rgba(0, 15, 5, 0.9); border: 2px solid #d4af37;
  border-radius: 15px; box-shadow: 0 0 50px rgba(212, 175, 55, 0.2);
  padding: 20px; position: relative; z-index: 10;
}

.tesla-header { display: flex; justify-content: space-between; align-items: center; border-bottom: 1px solid #d4af3733; padding-bottom: 10px; }
.gold-text { color: #d4af37; font-weight: bold; letter-spacing: 2px; }
.status-blink { width: 10px; height: 10px; background: #00ff41; border-radius: 50%; box-shadow: 0 0 10px #00ff41; animation: blink 1s infinite; }

.tesla-body { display: grid; grid-template-columns: 1fr 2fr; gap: 30px; margin-top: 20px; }

.tesla-head { width: 100%; filter: drop-shadow(0 0 15px #d4af37); animation: float 4s ease-in-out infinite; }

.matrix-title { color: #00ff41; text-transform: uppercase; margin-bottom: 15px; text-shadow: 0 0 5px #00ff41; }
.tesla-speech { font-size: 1.1rem; line-height: 1.6; color: #e0e0e0; }

.tech-specs { margin-top: 20px; display: grid; grid-template-columns: 1fr 1fr; gap: 10px; background: rgba(255, 255, 255, 0.05); padding: 15px; border-radius: 8px; }
.spec-label { color: #888; font-size: 0.8rem; }
.spec-value { color: #d4af37; font-weight: bold; margin-left: 5px; }

.close-btn { background: none; border: 1px solid #d4af37; color: #d4af37; cursor: pointer; border-radius: 4px; }
.close-btn:hover { background: #d4af37; color: black; }

/* Animaciones */
@keyframes move-grid { from { background-position: 0 0; } to { background-position: 0 50px; } }
@keyframes float { 0%, 100% { transform: translateY(0); } 50% { transform: translateY(-15px); } }
@keyframes blink { 0%, 100% { opacity: 1; } 50% { opacity: 0.2; } }

.fade-warp-enter-active, .fade-warp-leave-active { transition: all 0.5s ease; }
.fade-warp-enter-from, .fade-warp-leave-to { opacity: 0; transform: scale(1.1) rotate(2deg); }
</style>