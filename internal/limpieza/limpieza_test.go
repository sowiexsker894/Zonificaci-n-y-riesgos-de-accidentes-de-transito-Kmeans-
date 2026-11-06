package limpieza

import (
	"strings"
	"testing"
)

const encabezado = "codigo,fecha,hora,clase,fallecidos,departamento,provincia,distrito,latitud,longitud,clima"

func fila(s string) []string { return strings.Split(s, ",") }

func TestLimpiar(t *testing.T) {
	c, err := NuevasColumnas(fila(encabezado))
	if err != nil {
		t.Fatal(err)
	}
	casos := []struct {
		nombre, entrada, motivo, salida string
	}{
		{"valida", "A1,2022-03-23,22:06,CHOQUE,1,LIMA,LIMA,ATE,-12.05,-77.04,DESPEJADO",
			"", "A1,2022-03-23,22:06,CHOQUE,1,LIMA,LIMA,ATE,-12.050000,-77.040000,DESPEJADO"},
		{"texto sucio", "A1,2022-03-23,22:06,  choque   ,1,lima,LIMA,ATE,-12.05,-77.04,DESPEJADO",
			"", "A1,2022-03-23,22:06,CHOQUE,1,LIMA,LIMA,ATE,-12.050000,-77.040000,DESPEJADO"},
		{"coord invertidas", "A1,2022-03-23,22:06,CHOQUE,1,LIMA,LIMA,ATE,-77.04,-12.05,DESPEJADO",
			"", "A1,2022-03-23,22:06,CHOQUE,1,LIMA,LIMA,ATE,-12.050000,-77.040000,DESPEJADO"},
		{"hora 4.40", "A1,2022-03-23,4.40,CHOQUE,1,LIMA,LIMA,ATE,-12.05,-77.04,X",
			"", "A1,2022-03-23,04:40,CHOQUE,1,LIMA,LIMA,ATE,-12.050000,-77.040000,X"},
		{"fecha dd/mm", "A1,23/03/2022,22:06,CHOQUE,1,LIMA,LIMA,ATE,-12.05,-77.04,X",
			"", "A1,2022-03-23,22:06,CHOQUE,1,LIMA,LIMA,ATE,-12.050000,-77.040000,X"},
		{"imputaciones", "A1,2022-03-23,22:06,,,,LIMA,ATE,-12.05,-77.04,X",
			"", "A1,2022-03-23,22:06,NO ESPECIFICADO,1,DESCONOCIDO,LIMA,ATE,-12.050000,-77.040000,X"},
		{"coord texto", "A1,2022-03-23,22:06,CHOQUE,1,LIMA,LIMA,ATE,-12.05,NO REGISTRA,X", DescCoordTexto, ""},
		{"coord vacia", "A1,2022-03-23,22:06,CHOQUE,1,LIMA,LIMA,ATE,,-77.04,X", DescCoordTexto, ""},
		{"coord cero", "A1,2022-03-23,22:06,CHOQUE,1,LIMA,LIMA,ATE,0,0,X", DescCoordCero, ""},
		{"fuera de peru", "A1,2022-03-23,22:06,CHOQUE,1,LIMA,LIMA,ATE,45.3,-77.04,X", DescCoordFuera, ""},
		{"hora 25:70", "A1,2022-03-23,25:70,CHOQUE,1,LIMA,LIMA,ATE,-12.05,-77.04,X", DescHora, ""},
		{"hora sin dato", "A1,2022-03-23,SIN DATO,CHOQUE,1,LIMA,LIMA,ATE,-12.05,-77.04,X", DescHora, ""},
		{"fecha 00", "A1,00/00/0000,22:06,CHOQUE,1,LIMA,LIMA,ATE,-12.05,-77.04,X", DescFecha, ""},
		{"fallecidos -1", "A1,2022-03-23,22:06,CHOQUE,-1,LIMA,LIMA,ATE,-12.05,-77.04,X", DescFallecidos, ""},
		{"fallecidos 999", "A1,2022-03-23,22:06,CHOQUE,999,LIMA,LIMA,ATE,-12.05,-77.04,X", DescFallecidos, ""},
		{"columnas", "A1,2022-03-23", DescColumnas, ""},
	}
	for _, tc := range casos {
		t.Run(tc.nombre, func(t *testing.T) {
			f := fila(tc.entrada)
			got := Limpiar(f, c, NuevasStats())
			if got != tc.motivo {
				t.Fatalf("motivo = %q, esperado %q", got, tc.motivo)
			}
			if tc.motivo == "" && strings.Join(f, ",") != tc.salida {
				t.Fatalf("salida =\n%s\nesperado\n%s", strings.Join(f, ","), tc.salida)
			}
		})
	}
}

func TestNormalizarHora(t *testing.T) {
	for in, want := range map[string]string{"4:5": "04:05", "04:40": "04:40", "23.59": "23:59"} {
		if got, ok := NormalizarHora(in); !ok || got != want {
			t.Errorf("NormalizarHora(%q) = %q,%v", in, got, ok)
		}
	}
	for _, in := range []string{"99:99", "24:00", "", "abc", ":30"} {
		if _, ok := NormalizarHora(in); ok {
			t.Errorf("NormalizarHora(%q) debería fallar", in)
		}
	}
}
