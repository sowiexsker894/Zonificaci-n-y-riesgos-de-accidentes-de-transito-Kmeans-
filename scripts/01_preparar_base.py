"""
Paso 1 - Preparar la data base.

Lee el Excel original de la ONSV (siniestros fatales 2021-2025), renombra las
columnas a snake_case (más fácil de leer desde Go), normaliza textos, valida
coordenadas dentro del territorio peruano y exporta un CSV limpio.

Uso:
    python scripts/01_preparar_base.py
Salida:
    data/siniestros_base.csv
"""
from pathlib import Path

import pandas as pd

RAIZ = Path(__file__).resolve().parent.parent
EXCEL = RAIZ / "BBDD ONSV - SINIESTROS FATALES 2021-2025 (preliminar).xlsx"
SALIDA = RAIZ / "data" / "siniestros_base.csv"

# Límites aproximados del Perú (bounding box)
LAT_MIN, LAT_MAX = -18.40, -0.03
LON_MIN, LON_MAX = -81.40, -68.65

COLUMNAS = {
    "CÓDIGO SINIESTRO": "codigo",
    "FECHA SINIESTRO": "fecha",
    "HORA SINIESTRO": "hora",
    "CLASE SINIESTRO": "clase",
    "CANTIDAD DE FALLECIDOS": "fallecidos",
    "CANTIDAD DE LESIONADOS": "lesionados",
    "CANTIDAD DE VEHICULOS DAÑADOS": "vehiculos_danados",
    "DEPARTAMENTO": "departamento",
    "PROVINCIA": "provincia",
    "DISTRITO": "distrito",
    "ZONA": "zona",
    "TIPO DE VÍA": "tipo_via",
    "RED VIAL": "red_vial",
    "COD CARRETERA": "cod_carretera",
    "COORDENADAS LATITUD": "latitud",
    "COORDENADAS  LONGITUD": "longitud",
    "CONDICIÓN CLIMÁTICA": "clima",
    "ZONIFICACIÓN": "zonificacion",
    "CARACTERÍSTICAS DE VÍA": "caracteristica_via",
    "PERFIL LONGITUDINAL VÍA": "perfil_via",
    "SUPERFICIE DE CALZADA": "superficie",
    "¿EXISTE SEÑAL VERTICAL?": "senal_vertical",
    "CLASIFICACIÓN DE LA SEÑAL VERTICAL Nº 1": "senal_vertical_1",
    "CLASIFICACIÓN DE LA SEÑAL VERTICAL Nº 2": "senal_vertical_2",
    "¿EXISTE SEÑAL HORIZONTAL?": "senal_horizontal",
    "CAUSA FACTOR PRINCIPAL": "causa_principal",
    "CAUSA ESPECÍFICA": "causa_especifica",
}


def main() -> None:
    df = pd.read_excel(EXCEL, header=4)  # encabezados en la fila 5
    n_original = len(df)
    df = df.rename(columns=COLUMNAS)[list(COLUMNAS.values())]

    # Textos: quitar espacios sobrantes y pasar a mayúsculas
    texto = df.select_dtypes(include="object").columns.drop(["fecha", "hora"])
    for c in texto:
        df[c] = df[c].astype("string").str.strip().str.upper()

    # Fecha y hora -> formatos únicos
    df["fecha"] = pd.to_datetime(df["fecha"].astype(str), dayfirst=True, errors="coerce", format="mixed")
    df["hora"] = pd.to_datetime(df["hora"].astype(str), format="%H:%M", errors="coerce").dt.strftime("%H:%M")

    # Coordenadas numéricas y dentro del Perú
    df["latitud"] = pd.to_numeric(df["latitud"], errors="coerce")
    df["longitud"] = pd.to_numeric(df["longitud"], errors="coerce")
    dentro = df["latitud"].between(LAT_MIN, LAT_MAX) & df["longitud"].between(LON_MIN, LON_MAX)

    # Numéricos: vacío en vehículos dañados = 0
    for c in ["fallecidos", "lesionados", "vehiculos_danados"]:
        df[c] = pd.to_numeric(df[c], errors="coerce").fillna(0).astype(int)

    validas = (dentro & df["fecha"].notna() & df["hora"].notna()).fillna(False).astype(bool)
    descartadas = df[~validas]
    df = df[validas].copy()
    df["fecha"] = df["fecha"].dt.strftime("%Y-%m-%d")

    SALIDA.parent.mkdir(exist_ok=True)
    df.to_csv(SALIDA, index=False, encoding="utf-8")

    print(f"Filas originales : {n_original:,}")
    print(f"Filas descartadas: {len(descartadas):,} (coordenadas/fecha/hora inválidas)")
    print(f"Filas base       : {len(df):,}")
    print(f"Guardado en      : {SALIDA}")


if __name__ == "__main__":
    main()
