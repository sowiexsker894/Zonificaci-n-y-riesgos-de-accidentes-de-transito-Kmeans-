"""
Paso 2 - Data augmentation por inyección de ruido (jittering).

Cada siniestro real se replica N veces. En cada réplica se perturba:
  - latitud/longitud con ruido gaussiano (sigma ~ 100 m) -> el punto negro
    se mantiene en la misma zona, pero no es una copia exacta.
  - hora con ruido gaussiano (sigma = 15 min), acotada a 00:00-23:59.
  - fecha con un desplazamiento aleatorio de -7 a +7 días.
El resto de atributos (clase, causa, clima, etc.) se conserva.

Se agrega la columna `es_sintetico` (0 = real, 1 = generado) para que en el
informe se pueda distinguir la data real de la aumentada.

Uso:
    python scripts/02_augmentation.py [--objetivo 1200000] [--semilla 42]
Salida:
    data/siniestros_aug.csv
"""
import argparse
import math
from pathlib import Path

import numpy as np
import pandas as pd

RAIZ = Path(__file__).resolve().parent.parent
ENTRADA = RAIZ / "data" / "siniestros_base.csv"
SALIDA = RAIZ / "data" / "siniestros_aug.csv"

SIGMA_COORD = 0.0009  # grados ~ 100 m
SIGMA_MIN = 15        # minutos
MAX_DIAS = 7


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("--objetivo", type=int, default=1_200_000)
    ap.add_argument("--semilla", type=int, default=42)
    args = ap.parse_args()
    rng = np.random.default_rng(args.semilla)

    base = pd.read_csv(ENTRADA, dtype=str, keep_default_na=False)
    n = len(base)
    copias = math.ceil((args.objetivo - n) / n)  # réplicas sintéticas por fila
    print(f"Filas base: {n:,} | réplicas por fila: {copias} | objetivo: {args.objetivo:,}")

    idx = np.repeat(np.arange(n), copias)
    sint = base.iloc[idx].reset_index(drop=True)
    m = len(sint)
    replica = np.tile(np.arange(1, copias + 1), n)

    # Coordenadas
    lat = base["latitud"].astype(float).to_numpy()[idx] + rng.normal(0, SIGMA_COORD, m)
    lon = base["longitud"].astype(float).to_numpy()[idx] + rng.normal(0, SIGMA_COORD, m)
    sint["latitud"] = np.round(lat, 6).astype(str)
    sint["longitud"] = np.round(lon, 6).astype(str)

    # Hora
    hm = base["hora"].str.split(":", expand=True).astype(int)
    minutos = (hm[0] * 60 + hm[1]).to_numpy()[idx] + rng.normal(0, SIGMA_MIN, m)
    minutos = np.clip(np.round(minutos), 0, 1439).astype(int)
    sint["hora"] = [f"{h:02d}:{mm:02d}" for h, mm in zip(minutos // 60, minutos % 60)]

    # Fecha (acotada al rango original)
    fechas = pd.to_datetime(base["fecha"]).to_numpy()[idx]
    fechas = fechas + rng.integers(-MAX_DIAS, MAX_DIAS + 1, m).astype("timedelta64[D]")
    fmin, fmax = pd.to_datetime(base["fecha"]).min(), pd.to_datetime(base["fecha"]).max()
    sint["fecha"] = pd.Series(fechas).clip(fmin, fmax).dt.strftime("%Y-%m-%d").to_numpy()

    # Código único para cada réplica
    sint["codigo"] = sint["codigo"] + "-S" + pd.Series(replica).astype(str)
    sint["es_sintetico"] = "1"

    base = base.copy()
    base["es_sintetico"] = "0"
    aug = pd.concat([base, sint], ignore_index=True)
    aug = aug.sample(frac=1, random_state=args.semilla).reset_index(drop=True)  # mezclar

    aug.to_csv(SALIDA, index=False, encoding="utf-8")
    print(f"Filas reales     : {n:,}")
    print(f"Filas sintéticas : {m:,}")
    print(f"Total            : {len(aug):,}")
    print(f"Guardado en      : {SALIDA}")


if __name__ == "__main__":
    main()
