package main

// NodoBSP representa una partición del espacio o de los datos
type NodoBSP struct {
	IDZona     string   // Identificador de la partición (ej: "Zona_Norte", "Zona_Sur")
	Umbral     float64  // Valor de corte (puede ser coordenada X, Y o un nivel de prioridad)
	Izquierda  *NodoBSP // Sub-espacio menor
	Derecha    *NodoBSP // Sub-espacio mayor
	Elementos  []string // IDs de elementos o ADN contenidos en esta hoja
}

// InsertarNodo clasifica recursivamente un elemento en el árbol BSP según su valor métrico
func (n *NodoBSP) InsertarNodo(valor float64, idADN string) {
	if valor <= n.Umbral {
		if n.Izquierda != nil {
			n.Izquierda.InsertarNodo(valor, idADN)
		} else {
			n.Elementos = append(n.Elementos, idADN)
		}
	} else {
		if n.Derecha != nil {
			n.Derecha.InsertarNodo(valor, idADN)
		} else {
			n.Elementos = append(n.Elementos, idADN)
		}
	}
}

// CalcularNivelBSP determina la profundidad o nivel lógico del bloque basado en una métrica (ej. densidad o coordenadas)
func CalcularNivelBSP(metrica float64, profundidadActual int) int {
	// Lógica de partición binaria: a mayor precisión, mayor nivel BSP
	if profundidadActual > 5 {
		return 5 // Tope máximo de niveles lógicos
	}

	// Ejemplo de partición: si el valor cruza el umbral espacial, profundiza el nivel
	if metrica > 50.0 {
		return CalcularNivelBSP(metrica/2, profundidadActual+1)
	}

	return profundidadActual
}