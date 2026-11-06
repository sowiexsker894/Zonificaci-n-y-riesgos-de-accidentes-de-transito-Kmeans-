"""
Paso 3 - Ensuciar la data a propósito (dirty data injection / data corruption).

Introduce errores controlados y realistas sobre el dataset aumentado para que
el cargador concurrente en Go (Parte A) demuestre la etapa de limpieza.
Todo lo inyectado queda registrado en un reporte para usarlo como evidencia.

Errores inyectados (porcentajes sobre el total de filas):
  1. Nulos en columnas clave ............... 3.0 %
  2. Coordenadas inválidas ................. 1.0 %  (0,0 / fuera de Perú / lat-lon invertidas / texto)
  3. Hora mal formateada ................... 1.0 %  ("25:70", "4.40", "SIN DATO", "4:5")
  4. Fecha mal formateada .................. 0.5 %  (dd/mm/aaaa, aaaa/mm/dd, "00/00/0000")
  5. Texto inconsistente ................... 2.0 %  (minúsculas, espacios extra)
  6. Valores numéricos imposibles .......... 0.5 %  (fallecidos negativos o > 100)
  7. Filas duplicadas ...................... 1.0 %

Uso:
    python scripts/03_ensuciar.py [--semilla 7]
Salida:
    data/siniestros_sucio.csv
    data/reporte_ensuciado.md
"""
import argparse
from pathlib import Path

import numpy as np
import pandas as pd

RAIZ = Path(__file__).resolve().parent.parent
ENTRADA = RAIZ / "data" / "siniestros_aug.csv"
SALIDA = RAIZ / "data" / "siniestros_sucio.csv"
REPORTE = RAIZ / "data" / "reporte_ensuciado.md"


def elegir(rng, n, pct):
    return rng.choice(n, size=int(n * pct), replace=False)


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("--semilla", type=int, default=7)
    args = ap.parse_args()
    rng = np.random.default_rng(args.semilla)

    df = pd.read_csv(ENTRADA, dtype=str, keep_default_na=False)
    n = len(df)
    log = []

    # 1. Nulos
    cols_nulos = ["latitud", "longitud", "hora", "departamento", "clase", "fallecidos"]
    idx = elegir(rng, n, 0.03)
    cols = rng.choice(cols_nulos, size=len(idx))
    for c in cols_nulos:
        df.loc[idx[cols == c], c] = ""
    log.append(("Nulos (celdas vacías)", len(idx), ", ".join(cols_nulos)))

    # 2. Coordenadas inválidas
    idx = elegir(rng, n, 0.01)
    partes = np.array_split(idx, 4)
    df.loc[partes[0], ["latitud", "longitud"]] = "0"
    df.loc[partes[1], "latitud"] = (rng.uniform(10, 60, len(partes[1]))).round(6).astype(str)
    lat, lon = df.loc[partes[2], "latitud"].copy(), df.loc[partes[2], "longitud"].copy()
    df.loc[partes[2], "latitud"], df.loc[partes[2], "longitud"] = lon.values, lat.values
    df.loc[partes[3], "longitud"] = "NO REGISTRA"
    log.append(("Coordenadas inválidas", len(idx), "0,0 / fuera de Perú / invertidas / texto"))

    # 3. Hora mal formateada
    idx = elegir(rng, n, 0.01)
    df.loc[idx, "hora"] = rng.choice(["25:70", "4.40", "SIN DATO", "4:5", "99:99"], size=len(idx))
    log.append(("Hora mal formateada", len(idx), "25:70, 4.40, SIN DATO, 4:5, 99:99"))

    # 4. Fecha mal formateada
    idx = elegir(rng, n, 0.005)
    fechas = pd.to_datetime(df.loc[idx, "fecha"], errors="coerce")
    formato = rng.integers(0, 3, len(idx))
    nuevas = np.where(formato == 0, fechas.dt.strftime("%d/%m/%Y"),
             np.where(formato == 1, fechas.dt.strftime("%Y/%m/%d"), "00/00/0000"))
    df.loc[idx, "fecha"] = nuevas
    log.append(("Fecha mal formateada", len(idx), "dd/mm/aaaa, aaaa/mm/dd, 00/00/0000"))

    # 5. Texto inconsistente
    idx = elegir(rng, n, 0.02)
    cols_txt = ["departamento", "provincia", "distrito", "clase", "clima"]
    cols = rng.choice(cols_txt, size=len(idx))
    for c in cols_txt:
        sel = idx[cols == c]
        mitad = len(sel) // 2
        df.loc[sel[:mitad], c] = df.loc[sel[:mitad], c].str.lower()
        df.loc[sel[mitad:], c] = "  " + df.loc[sel[mitad:], c] + "   "
    log.append(("Texto inconsistente", len(idx), "minúsculas / espacios extra en " + ", ".join(cols_txt)))

    # 6. Numéricos imposibles
    idx = elegir(rng, n, 0.005)
    df.loc[idx, "fallecidos"] = rng.choice(["-1", "-3", "150", "999"], size=len(idx))
    log.append(("Fallecidos imposibles", len(idx), "-1, -3, 150, 999"))

    # 7. Duplicados
    idx = elegir(rng, n, 0.01)
    df = pd.concat([df, df.iloc[idx]], ignore_index=True)
    df = df.sample(frac=1, random_state=args.semilla).reset_index(drop=True)
    log.append(("Filas duplicadas", len(idx), "copia exacta de filas existentes"))

    df.to_csv(SALIDA, index=False, encoding="utf-8")

    total = sum(c for _, c, _ in log)
    with open(REPORTE, "w", encoding="utf-8") as f:
        f.write("# Reporte de ensuciado intencional\n\n")
        f.write(f"- Filas de entrada: {n:,}\n- Filas de salida: {len(df):,}\n- Semilla: {args.semilla}\n\n")
        f.write("| Tipo de error | Filas afectadas | % | Detalle |\n|---|---:|---:|---|\n")
        for nombre, cant, det in log:
            f.write(f"| {nombre} | {cant:,} | {cant / n:.2%} | {det} |\n")
        f.write(f"| **Total** | **{total:,}** | **{total / n:.2%}** | |\n")

    print(open(REPORTE, encoding="utf-8").read())
    print(f"Guardado en: {SALIDA}")


if __name__ == "__main__":
    main()
