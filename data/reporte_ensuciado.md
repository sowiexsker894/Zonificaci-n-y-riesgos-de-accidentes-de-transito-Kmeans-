# Reporte de ensuciado intencional

- Filas de entrada: 1,201,860
- Filas de salida: 1,213,878
- Semilla: 7

| Tipo de error | Filas afectadas | % | Detalle |
|---|---:|---:|---|
| Nulos (celdas vacías) | 36,055 | 3.00% | latitud, longitud, hora, departamento, clase, fallecidos |
| Coordenadas inválidas | 12,018 | 1.00% | 0,0 / fuera de Perú / invertidas / texto |
| Hora mal formateada | 12,018 | 1.00% | 25:70, 4.40, SIN DATO, 4:5, 99:99 |
| Fecha mal formateada | 6,009 | 0.50% | dd/mm/aaaa, aaaa/mm/dd, 00/00/0000 |
| Texto inconsistente | 24,037 | 2.00% | minúsculas / espacios extra en departamento, provincia, distrito, clase, clima |
| Fallecidos imposibles | 6,009 | 0.50% | -1, -3, 150, 999 |
| Filas duplicadas | 12,018 | 1.00% | copia exacta de filas existentes |
| **Total** | **108,164** | **9.00%** | |
