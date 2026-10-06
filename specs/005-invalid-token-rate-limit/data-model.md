# Data Model: Documentar el 429 por tokens inexistentes (IPzilon 3.4.0)

La feature no añade ni modifica atributos, recursos, data sources ni estado de Terraform. Solo
modela una respuesta de la API y el evento de log que provoca.

## Respuesta: bloqueo por tokens inexistentes (IPzilon >= 3.4.0)

| Campo | Valor | Uso en el provider |
|---|---|---|
| Código HTTP | `429` | Entra en la rama de reintento existente (igual que el límite global). |
| `Retry-After` | segundos restantes de la ventana, `≤ 60` por defecto | Espera del reintento (sin cambios). |
| Cuerpo | `{"error": "Too many invalid API tokens"}` | `parseAPIError` lo extrae como mensaje; su prefijo identifica la causa. |

**Clasificación** (por respuesta reintentada):

| Código | Mensaje | Causa | Aviso |
|---|---|---|---|
| 429 | empieza por `Too many invalid API tokens` | bloqueo por dirección de cliente | sí (1 por petición) |
| 429 | cualquier otro (`Rate limit exceeded: …`, vacío…) | cupo del token / límite por ruta | no |
| 503 | cualquiera | servidor ocupado | no |

## Evento: aviso de bloqueo por dirección

Estado por petición (una llamada a `do`): `warned` (booleano, inicial `false`).

```text
recibe 429 "Too many invalid API tokens" ──► ¿se reintenta? ── no ──► error final (sin cambios)
                                                   │ sí
                                                   ▼
                                    warned == false ──► Warn + warned = true
                                                   │
                                                   ▼
                                     Debug de reintento (igual que hoy) ──► espera ──► reintento
```

Campos del aviso: ver [contracts/log-warning.md](./contracts/log-warning.md).
