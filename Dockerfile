# --- ETAPA 1: Construcción (El Laboratorio de ADN) ---
FROM golang:1.23-alpine AS builder

# Instalamos herramientas esenciales para compilación de red
RUN apk add --no-cache git alpine-sdk

WORKDIR /app

# Sincronía de módulos para evitar latencia en build
COPY go.mod go.sum ./
RUN go mod download

# Inyectamos todo el sistema de archivos
COPY . .

# Compilación estática extrema: -s (quita tablas de símbolos) -w (quita info de debug)
# Esto hace que el binario sea una "piedra" sólida e indestructible.
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o geochat-nexo main.go

# --- ETAPA 2: Ejecución (El Organismo con IA y Sentidos) ---
FROM python:3.11-slim-bookworm 

# Seteamos variables de entorno para que Python no genere basura (.pyc)
ENV PYTHONDONTWRITEBYTECODE=1
ENV PYTHONUNBUFFERED=1

# Instalamos certificados, zona horaria y librerías para Ettus Research (UHD)
RUN apt-get update && apt-get install -y \
    ca-certificates \
    tzdata \
    libuhd-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /root/

# --- 🔱 TRASVASE DE ACTIVOS CRÍTICOS ---

# 1. El Cerebro (Binario de Go)
COPY --from=builder /app/geochat-nexo .

# 2. El Oráculo (Interfaz Principal)
COPY --from=builder /app/index.html .

# 3. Los Órganos Visuales (Frontend y Naves Offline)
# NOTA: Ajustado a 'cascarafrontendrender' según tu comando tree
COPY --from=builder /app/cascarafrontendrender ./cascarafrontendrender

# --- 🔱 INSTALACIÓN DEL CEREBELO (IA) ---
# Instalamos TensorFlow-CPU para mantener el consumo de RAM bajo los 4GB de Render
RUN pip install --no-cache-dir tensorflow-cpu

# --- 🔱 CONFIGURACIÓN DE IGNICIÓN ---

# Permisos para que el Nexo pueda operar
RUN chmod +x ./geochat-nexo

# Puerto estándar de Render
EXPOSE 8080

# El despertar del Nexo: Go toma el mando y orquesta a Python y el SDR
CMD ["./geochat-nexo"]