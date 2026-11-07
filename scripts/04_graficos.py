"""
Paso 6 - Gráficos para el informe (PNG a 200 dpi, listos para Word).

Genera en docs/graficos/:
  1. mapa_zonas.png        Siniestros reales + 15 zonas K-Means (tamaño = fallecidos)
  2. curva_codo.png        SSE vs k, con el codo elegido por Kneedle
  3. curvas_speedup.png    Speedup del cargador (Parte A) y de K-Means (Parte B),
                           Windows vs WSL, frente al ideal lineal

Uso:
    python scripts/04_graficos.py
"""
import json
import math
from pathlib import Path

import matplotlib

matplotlib.use("Agg")
import matplotlib.pyplot as plt
import pandas as pd

RAIZ = Path(__file__).resolve().parent.parent
DATA = RAIZ / "data"
SALIDA = RAIZ / "docs" / "graficos"

# Paleta validada (CVD y contraste) sobre superficie clara
SUPERFICIE = "#ffffff"
TEXTO = "#0b0b0b"
TEXTO_2 = "#52514e"
MUTED = "#898781"
GRID = "#e1e0d9"
AZUL = "#2a78d6"      # serie 1: Windows / zonas
NARANJA = "#eb6834"   # serie 2: WSL
PUNTOS = "#b9b8b2"    # siniestros individuales (contexto, recesivo)

plt.rcParams.update({
    "font.family": ["Segoe UI", "DejaVu Sans"],
    "font.size": 10,
    "axes.edgecolor": GRID,
    "axes.labelcolor": TEXTO_2,
    "axes.titlecolor": TEXTO,
    "axes.titlesize": 12,
    "axes.titleweight": "bold",
    "axes.titlelocation": "left",
    "xtick.color": MUTED,
    "ytick.color": MUTED,
    "xtick.labelcolor": TEXTO_2,
    "ytick.labelcolor": TEXTO_2,
    "axes.grid": True,
    "grid.color": GRID,
    "grid.linewidth": 0.6,
    "figure.facecolor": SUPERFICIE,
    "axes.facecolor": SUPERFICIE,
    "savefig.dpi": 200,
    "savefig.bbox": "tight",
})


def limpiar_ejes(ax):
    for s in ("top", "right"):
        ax.spines[s].set_visible(False)
    ax.tick_params(length=0)


def titulo(fig, t, sub, y=1.0):
    fig.suptitle(t, x=0.01, y=y, ha="left", va="bottom", fontsize=12.5, fontweight="bold", color=TEXTO)
    fig.text(0.01, y - 0.004, sub, ha="left", va="top", color=TEXTO_2, fontsize=9)


def nota(fig, texto):
    fig.text(0.01, -0.02, texto, color=MUTED, fontsize=8, ha="left", va="top")


# ---------------------------------------------------------------------------
def mapa_zonas():
    pts = pd.read_csv(DATA / "kmeans_puntos_reales.csv")
    zonas = pd.DataFrame(json.loads((DATA / "kmeans_clusters.json").read_text(encoding="utf-8")))

    fig, ax = plt.subplots(figsize=(8, 8))
    ax.scatter(pts.longitud, pts.latitud, s=2, c=PUNTOS, alpha=0.6, linewidths=0, zorder=1,
               label=f"Siniestro fatal real ({len(pts):,})")

    # burbuja: área proporcional a fallecidos reales
    escala = 1.1
    tam = zonas.fallecidos_reales * escala
    ax.scatter(zonas.lon, zonas.lat, s=tam, c=AZUL, alpha=0.78, edgecolors=SUPERFICIE,
               linewidths=2, zorder=3)
    for _, z in zonas.iterrows():
        if z.fallecidos_reales < 150:  # burbuja muy pequeña: número al lado
            ax.text(z.lon + 0.35, z.lat, str(z.ranking), ha="left", va="center", fontsize=8.5,
                    fontweight="bold", color=TEXTO, zorder=4)
        else:
            ax.text(z.lon, z.lat, str(z.ranking), ha="center", va="center", fontsize=8.5,
                    fontweight="bold", color="white", zorder=4)

    # etiquetas directas solo para las 5 zonas con más fallecidos
    desplaz = {1: (-2.3, -0.3), 2: (-2.4, 0.0), 3: (0.7, 1.6), 4: (-2.4, -0.6), 5: (-1.6, -1.0)}
    for _, z in zonas[zonas.ranking <= 5].iterrows():
        dx, dy = desplaz[z.ranking]
        nombre = z.provincia.title() if z.departamento != "LIMA" else "Lima Metropolitana"
        ax.annotate(
            f"{z.ranking}. {nombre}\n{z.pct_fallecidos_nacional:.1f}% de muertes · pico {z.franja_pico_3h[:5]}–{z.franja_pico_3h[6:]}",
            xy=(z.lon, z.lat), xytext=(z.lon + dx, z.lat + dy), fontsize=8.5, color=TEXTO,
            ha="right" if dx < 0 else "left", va="center",
            arrowprops=dict(arrowstyle="-", color=MUTED, lw=0.8, shrinkA=0, shrinkB=6), zorder=5)

    # leyenda de tamaño
    for v in (100, 500, 1000):
        ax.scatter([], [], s=v * escala, c=AZUL, alpha=0.78, edgecolors=SUPERFICIE, linewidths=2,
                   label=f"Zona con {v:,} fallecidos")
    leg = ax.legend(loc="lower left", frameon=False, labelspacing=2.0, borderpad=0.4, fontsize=8.5,
                    labelcolor=TEXTO_2, scatterpoints=1, handletextpad=1.6)
    leg.legend_handles[0]._sizes = [18]

    lat0 = pts.latitud.mean()
    ax.set_aspect(1 / math.cos(math.radians(lat0)))
    ax.set_xlim(-85.5, -67.5)
    ax.set_ylim(-19.5, -3.0)
    ax.set_axis_off()
    fig.subplots_adjust(left=0.01, right=0.99, top=0.9, bottom=0.03)
    titulo(fig, "Zonas de mayor siniestralidad fatal en el Perú",
           "K-Means paralelo, k = 15 · número = ranking por fallecidos · 2021–2025", y=0.955)
    nota(fig, "Fuente: ONSV-MTC, siniestros fatales 2021–2025 (preliminar). Solo registros reales (es_sintetico = 0).")
    fig.savefig(SALIDA / "mapa_zonas.png")
    plt.close(fig)


# ---------------------------------------------------------------------------
def curva_codo():
    c = pd.read_csv(DATA / "kmeans_codo.csv")
    c["sse_mil_millones"] = c.sse / 1e9
    k_elegido = json.loads((DATA / "metricas_kmeans.json").read_text(encoding="utf-8"))["codo"]["k_elegido"]

    fig, ax = plt.subplots(figsize=(7.5, 4.2))
    # recta del método Kneedle (entre extremos)
    ax.plot([c.k.iloc[0], c.k.iloc[-1]], [c.sse_mil_millones.iloc[0], c.sse_mil_millones.iloc[-1]],
            color=MUTED, lw=1, ls=(0, (4, 3)), zorder=1)
    ax.text(41, 13.2, "recta entre extremos (Kneedle):\nel codo es el punto más alejado",
            color=MUTED, fontsize=8, ha="left")
    ax.plot(c.k, c.sse_mil_millones, color=AZUL, lw=2, marker="o", markersize=6,
            markeredgecolor=SUPERFICIE, markeredgewidth=1.5, zorder=3)
    sel = c[c.k == k_elegido].iloc[0]
    ax.scatter([sel.k], [sel.sse_mil_millones], s=180, facecolors="none", edgecolors=TEXTO, lw=1.5, zorder=4)
    ax.annotate(f"k = {k_elegido} (codo)\nSSE = {sel.sse_mil_millones:.1f} mil millones de km²",
                xy=(sel.k, sel.sse_mil_millones), xytext=(sel.k + 5, sel.sse_mil_millones + 3.5),
                fontsize=9, color=TEXTO,
                arrowprops=dict(arrowstyle="-", color=MUTED, lw=0.8, shrinkB=8))
    ax.set_xticks(c.k)
    ax.set_ylim(0, None)
    ax.set_xlabel("Número de clusters (k)")
    ax.set_ylabel("SSE (miles de millones de km²)")
    limpiar_ejes(ax)
    titulo(fig, "Método del codo para elegir k",
           "Suma de distancias al cuadrado de cada siniestro a su centroide (SSE), 1,160,337 siniestros", y=0.99)
    fig.subplots_adjust(top=0.86)
    nota(fig, "Después de k = 15, cada cluster adicional reduce el error mucho menos.")
    fig.savefig(SALIDA / "curva_codo.png")
    plt.close(fig)


# ---------------------------------------------------------------------------
def curvas_speedup():
    def cargar(nombre):
        return pd.DataFrame(json.loads((DATA / nombre).read_text(encoding="utf-8"))["benchmark"])

    paneles = [
        ("A. Cargador concurrente (limpieza)", cargar("metricas_loader.json"), cargar("metricas_loader_wsl.json"), 8),
        ("B. K-Means paralelo (30 iteraciones)", cargar("metricas_kmeans.json"), cargar("metricas_kmeans_wsl.json"), 12),
    ]
    fig, axes = plt.subplots(1, 2, figsize=(10, 4.4), sharey=True)
    for ax, (titulo, win, wsl, optimo) in zip(axes, paneles):
        ws = win.workers
        ax.plot(ws, ws, color=MUTED, lw=1, ls=(0, (4, 3)), zorder=1)
        ax.text(6.0, 5.75, "ideal (lineal)", color=MUTED, fontsize=8, ha="left", va="center")
        ys = {"Windows": win.speedup.iloc[-1], "WSL Ubuntu": wsl.speedup.iloc[-1]}
        if abs(ys["Windows"] - ys["WSL Ubuntu"]) < 0.5:  # separar etiquetas que chocan
            alto, bajo = max(ys, key=ys.get), min(ys, key=ys.get)
            medio = (ys[alto] + ys[bajo]) / 2
            ys[alto], ys[bajo] = medio + 0.28, medio - 0.28
        for df, color, nombre in ((win, AZUL, "Windows"), (wsl, NARANJA, "WSL Ubuntu")):
            ax.plot(df.workers, df.speedup, color=color, lw=2, marker="o", markersize=6,
                    markeredgecolor=SUPERFICIE, markeredgewidth=1.5, label=nombre, zorder=3)
            ult = df.iloc[-1]
            ax.text(ult.workers + 0.4, ys[nombre], f"{nombre} {ult.speedup:.2f}x", color=TEXTO_2,
                    fontsize=8, va="center")
        # óptimo elegido
        mejor = max(win[win.workers == optimo].speedup.iloc[0], wsl[wsl.workers == optimo].speedup.iloc[0])
        ax.axvline(optimo, color=GRID, lw=6, zorder=0)
        ax.text(optimo, 0.25, f" elegido: {optimo} workers", color=TEXTO, fontsize=8.5, ha="center",
                va="bottom", bbox=dict(facecolor=SUPERFICIE, edgecolor="none", pad=1))
        ax.set_xticks(ws)
        ax.set_xlim(0.5, 16.5)
        ax.set_ylim(0, 6)
        ax.set_xlabel("Workers (goroutines)")
        ax.set_title(titulo, fontsize=11)
        limpiar_ejes(ax)
    axes[0].set_ylabel("Speedup (tiempo 1 worker / tiempo N)")
    axes[0].legend(loc="upper left", frameon=False, fontsize=8.5, labelcolor=TEXTO_2)
    fig.suptitle("Aceleración por número de goroutines", x=0.01, ha="left", fontsize=12.5,
                 fontweight="bold", color=TEXTO)
    fig.text(0.01, 0.905, "Intel i7-1265U (2 P-cores con HT + 8 E-cores = 12 hilos) · mediana de varias repeticiones",
             color=TEXTO_2, fontsize=9)
    fig.tight_layout(rect=(0, 0, 1, 0.9))
    nota(fig, "El cargador satura en 8 workers (GC, deduplicación y escritura compiten por CPU); "
              "K-Means es cálculo puro y escala hasta los 12 hilos.")
    fig.savefig(SALIDA / "curvas_speedup.png")
    plt.close(fig)


def main():
    SALIDA.mkdir(parents=True, exist_ok=True)
    mapa_zonas()
    curva_codo()
    curvas_speedup()
    for f in sorted(SALIDA.glob("*.png")):
        print(f"Guardado: {f}")


if __name__ == "__main__":
    main()
